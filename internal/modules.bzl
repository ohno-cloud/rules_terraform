load(
    ":download.bzl",
    "TerraformBinaryInfo",
    "TerraformProviderInfo",
)
load(
    ":backend.bzl",
    "TerraformBackendInfo",
)

TerraformModuleInfo = provider(
    "Provider for the terraform_module rule",
    fields={
        "source_files": "depset of source Terraform files",
        "modules": "depset of modules",
        "providers": "depset of providers",
    })

def _terraform_module_impl(ctx):
    source_files = []

    # Symlink all non-generated files so they are stored alongside any generated
    # files. Terraform runs for a given directory and the files need to be in
    # their correct positions, so we can't just reference the different input
    # files if they are in different directories in the bazel sandbox.
    for src in ctx.files.srcs:
        if src.is_source:
            src_symlink = ctx.actions.declare_file(src.basename)
            ctx.actions.symlink(output = src_symlink, target_file = src)
            source_files.append(src_symlink)
        else:
            source_files.append(src)

    # Generate required_providers block based on any provider inputs to this
    # rule.
    if ctx.attr.generate_required_providers and ctx.attr.providers:
        required_providers = {}
        for provider in ctx.attr.providers:
            provider = provider[TerraformProviderInfo]
            required_providers[provider.provider_name] = {
                "source": provider.source,
                "version": "= " + provider.version,
            }

        required_providers_struct = struct(
            terraform = struct (
                required_providers = required_providers
            ),
        )

        required_providers_file = ctx.actions.declare_file("__bazel_required_providers.tf.json")
        ctx.actions.write(
            required_providers_file,
            json.encode_indent(required_providers_struct, indent = '  '),
            is_executable = False,
        )
        source_files.append(required_providers_file)

    return [
        DefaultInfo(files = depset(source_files)),
        TerraformModuleInfo(
            source_files = depset(
                source_files,
                transitive = [dep[TerraformModuleInfo].source_files for dep in ctx.attr.deps]
            ),
            modules = depset(
                ctx.attr.deps,
                transitive = [dep[TerraformModuleInfo].modules for dep in ctx.attr.deps]
            ),
            providers = depset(
                ctx.attr.providers,
                transitive = [dep[TerraformModuleInfo].providers for dep in ctx.attr.deps]
            ),
        )
    ]

terraform_module = rule(
    implementation = _terraform_module_impl,
    doc = """Collects files and dependencies for a Terraform module.

This rules does nothing by itself really, but its output of this rule is used
in other rules like terraform_root_module or for tests.
    """,
    attrs = {
        "srcs": attr.label_list(
            mandatory = True,
            allow_files = True,
        ),
        "providers": attr.label_list(
            providers = [TerraformProviderInfo],
        ),
        "deps": attr.label_list(
            providers = [TerraformModuleInfo],
        ),
        "generate_required_providers": attr.bool(
            default = True,
            doc = "Generate a required_providers block with provider versions",
        ),
    },
)

TerraformRootModuleInfo = provider(
    "Provider for the terraform_root_module rule",
    fields={
        "terraform_wrapper": "Terraform wrapper script to run terraform in this rule's output directory",
        "runfiles": "depset of collected files needed to run",
        "deps": "list of labels of other backends to import",
    }
)

TerraformWorkspaceInfo = provider(
    "Meta provider for the terraform_root_module rule to expose workspace and tfvars of the label",
    fields={
        "workspace": "name of the Terraform workspace",
        "tfvars": "depset of tfvar files",
    }
)


def _terraform_root_module_impl(ctx):
    terraform_info = ctx.attr.terraform[TerraformBinaryInfo]
    terraform_binary = terraform_info.binary
    terraform_version = terraform_info.version

    module = ctx.attr.module[TerraformModuleInfo]

    runfiles = [terraform_binary] + module.source_files.to_list()

    modules_list = module.modules.to_list()
    providers_list = [p[TerraformProviderInfo] for p in module.providers.to_list()]
    provider_files = [p.provider for p in providers_list]

    # Create a plugin cache dir.
    plugin_cache_dir = "plugin_cache"
    plugin_mirror_dir = "plugin_mirror"
    for provider in providers_list:
        output = ctx.actions.declare_file("{}/{}/{}".format(
            plugin_cache_dir,
            provider.platform,
            provider.provider.basename,
        ))
        ctx.actions.symlink(
            output = output,
            target_file = provider.provider,
        )
        runfiles.append(output)

        # Special filesystem mirror format
        if terraform_version >= "0.13.2":
            output = ctx.actions.declare_file("{}/{}/{}/{}/{}".format(
                plugin_mirror_dir,
                provider.source,
                provider.version,
                provider.platform,
                provider.provider.basename,
            ))
            ctx.actions.symlink(
                output = output,
                target_file = provider.provider,
            )
            runfiles.append(output)

    # Symlink the tfvar files into terraform "root"
    for tfvar_file in ctx.files.tfvars:
        output = ctx.actions.declare_file(tfvar_file.basename)
        ctx.actions.symlink(
            output = output,
            target_file = tfvar_file,
        )
        runfiles.append(output)


    # Create terraformrc
    terraformrc = ctx.actions.declare_file(ctx.label.name + "_terraformrc.tfrc")
    runfiles.append(terraformrc)
    terraformrc_content = """
plugin_cache_dir = "{}"
# Disable upgrade & security checks to HashiCorp servers
disable_checkpoint = true
    """.format(plugin_cache_dir)

    # Use and explicit filesystem_mirror block if terraform_version >= 0.13.2
    if terraform_version >= "0.13.2":
        terraformrc_content += """
provider_installation {{
  filesystem_mirror {{
    path    = "{plugin_mirror_dir}"
    include = ["*/*/*"]
  }}
}}
        """.format(plugin_mirror_dir = plugin_mirror_dir)

    ctx.actions.write(
        output = terraformrc,
        content = terraformrc_content,
        is_executable = False,
    )

    # Create a wrapper script that runs terraform in a bazel run directory with
    # all of the necessary files symlinked.
    wrapper = ctx.actions.declare_file(ctx.label.name + "_run_wrapper")
    runfiles.append(wrapper)
    ctx.actions.write(
        output = wrapper,
        is_executable = True,
        content = """#!/usr/bin/env bash
set -eu

terraform="$(pwd)/{terraform}"

cd "{package}"

# If TF_DATA_DIR is unset, set it to a special directory under the workspace
# root. This env var _is_ set in e.g. tests so they can do terraform init
# without affecting users' .terraform files.
#
# We can't store .terraform as a bazel file because there is lots of mutable
# state in there, and we can't mutate it if it is written from a bazel rule. For
# example, the S3 backend requires initialization with valid AWS credentials,
# which we can't provide during a bazel build.
#
# TODO: Try to more intelligently cache parts of .terraform, like the providers/
# directory. We should ideally make installing those as fast as possible.
#
export TF_DATA_DIR="${{TF_DATA_DIR:-$BUILD_WORKSPACE_DIRECTORY/{package}/.terraform}}"

export TF_CLI_CONFIG_FILE="{terraformrc}"

exec "$terraform" $@""".format(
            package = ctx.label.package,
            terraform = terraform_binary.short_path,
            terraformrc = terraformrc.basename,
        ),
    )

    # Call the wrapper script from the root module and just run validate
    validation_output = ctx.actions.declare_file(ctx.label.name + ".validation")
    ctx.actions.run_shell(
        mnemonic = "TerraformValidate",
        inputs = runfiles,
        outputs = [validation_output],
        # Allow user shell environment variables due to Terraform Providers
        # will require some variables as a part of validation.
        use_default_shell_env = True,
        command = """
export TF_DATA_DIR=".terraform";
export TF_SKIP_PROVIDER_VERIFY="1";
export TF_CLI_CONFIG_FILE="{terraformrc}";
"{terraform}" -chdir="{package}" init -input=false -backend=false &> {output} &&
"{terraform}" -chdir="{package}" validate &> {output} || ( cat {output} && false )""".format(
            package = validation_output.dirname,
            terraform = terraform_binary.path,
            output = validation_output.path,
            terraformrc = terraformrc.path,
        ),
    )

    return [
        DefaultInfo(
            runfiles = ctx.runfiles(files = runfiles),
            executable = wrapper,
        ),
        TerraformRootModuleInfo(
            terraform_wrapper = wrapper,
            runfiles = ctx.runfiles(files = runfiles),
            deps = [ dep[TerraformBackendInfo] for dep in ctx.attr.deps]
        ),
        TerraformWorkspaceInfo(
            workspace = ctx.attr.workspace,
            tfvars = [ f.path for f in ctx.files.tfvars ],
        ),
        TerraformBackendInfo(
            label = str(ctx.label),
            workspace = ctx.attr.workspace,
        ),
        OutputGroupInfo(_validation = depset([validation_output])),
    ]

terraform_root_module = rule(
    implementation = _terraform_root_module_impl,
    doc = """Provides runnable Terraform wrapper script and providers for a root module.

This rule builds an executable wrapper script that runs Terraform for the root module
with all of the necessary bits in place from the dependent module.
    """,
    attrs = {
        "module": attr.label(
            mandatory = True,
            providers = [TerraformModuleInfo],
        ),
        "workspace": attr.string(
            doc = "Terraform workspace for the Root Module",
            default = "default",
            mandatory = True,
        ),
        "tfvars": attr.label_list(
            allow_empty = True,
            allow_files = [".tfvars", ".tfvars.json", ".json"],
            doc = "Terraform variables for the Root Module",
            mandatory = True,
        ),
        "deps": attr.label_list(
            allow_empty = True,
            providers = [TerraformBackendInfo],
            doc = "Terraform variables for the Root Module",
        ),
        # TODO: replace with toolchain
        "terraform": attr.label(
            allow_single_file = True,
            executable = True,
            cfg = "host",
            providers = [TerraformBinaryInfo],
        ),
    },
    executable = True,
)
