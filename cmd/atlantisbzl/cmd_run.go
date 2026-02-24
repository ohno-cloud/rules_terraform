package main

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/ohno-cloud/rules_terraform/pkg/bazel"
	"github.com/ohno-cloud/rules_terraform/pkg/terraform"
	bzlTf "github.com/ohno-cloud/rules_terraform/pkg/terraform/bazel"
)

type RunCmd struct {
	// Atlantis provided variables
	// See https://www.runatlantis.io/docs/custom-workflows#custom-run-command for all
	// variables.
	SubPath     string `short:"t" env:"REPO_REL_DIR" required:""`
	Workspace   string `short:"w" env:"WORKSPACE" required:""`
	PlanFile    string `env:"PLANFILE" required:""`
	ShowFile    string `env:"SHOWFILE" required:""`
	ProjectName string `env:"PROJECT_NAME" required:""`

	Args []string `arg:""`

	// Environment variables passed through ConfigMap
	TerraformBackend string `env:"TERRAFORM_BACKEND" required:""`
}

func (r *RunCmd) Run(c Common) error {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(c.Timeout))
	defer cancel()

	basePath := strings.TrimSuffix(c.BazelDir, r.SubPath)
	bzl, bzlErr := bazel.NewBazel(basePath)
	if bzlErr != nil {
		return fmt.Errorf("Failed to setup Bazel: %s", bzlErr)
	}

	// Subpath and workspace are components of the bazel target label
	label := bazel.Label(fmt.Sprintf("//%s:%s", r.SubPath, r.Workspace))

	// Setup the backend environment
	baseURL, urlErr := url.Parse(r.TerraformBackend)
	if urlErr != nil {
		return fmt.Errorf("Failed to parse URL %s : %s", r.TerraformBackend, urlErr)
	}

	return c.tfRunner(ctx, label, bzl, basePath, baseURL, r.Args, r.ShowFile, r.PlanFile)
}

func (c Common) tfRunner(ctx context.Context, target bazel.Label, bzl bazel.Queries, basePath string, backend *url.URL, args []string, showFile string, planFile string) error {

	terraformProviders, queryErr := bzlTf.QueryTfProviders(ctx, bzl, target)
	if queryErr != nil {
		return fmt.Errorf("Failed to xml query for providers: %w", queryErr)
	}

	if buildErr := bzl.Build(ctx, target); buildErr != nil {
		return fmt.Errorf("Failed to build %s : %w", target, buildErr)
	}

	rootModule, rootModuleErr := bzlTf.QueryRootModule(ctx, bzl, target)
	if rootModuleErr != nil {
		return fmt.Errorf("Failed to query root module for %s : %w", target, rootModuleErr)
	}

	// Clean up any existing terraform directory
	dotTfPath := path.Join(target.ToWorkspacePath(basePath), ".terraform")
	if removeErr := os.RemoveAll(dotTfPath); removeErr != nil {
		return fmt.Errorf("Failed to remove .terraform path from %s: %s ", dotTfPath, removeErr)
	}

	runEnv := map[string]string{
		"TF_IN_AUTOMATION":          "true",
		"BUILD_WORKSPACE_DIRECTORY": basePath,
	}

	// Parse the Root Module deps to be imported by exposing them to terraform
	// as a environment variable tfvar.
	backendDeps := make([]bzlTf.TerraformRootModuleDep, 0, 0)
	if err := bzl.QueryStarlarkJson(
		ctx,
		&backendDeps,
		"--starlark:expr",
		bzlTf.TerraformRootModuleDepsStarlarkExpr,
		string(target),
	); err != nil {
		return fmt.Errorf("Failed to query for backend deps for %s : %w", target, err)
	}

	backendMap := map[string]terraform.BackendConfig{}
	for _, dep := range backendDeps {
		backendMap[dep.Label] = terraform.BackendConfig{
			Config: terraform.HttpBackendConfig{
				Address: terraform.LabelToStatePath(backend, dep.Label, dep.Workspace),
			},
		}
	}
	if len(backendMap) > 0 {
		jsonBytes, err := json.Marshal(backendMap)
		if err != nil {
			return fmt.Errorf("Failed to marshal backend config: %w", err)
		}

		runEnv["TF_VAR_root_module_deps"] = string(jsonBytes)
	}

	// Setup the backend environment
	client, clientErr := c.getVaultClient(ctx)
	if clientErr != nil {
		return fmt.Errorf("Failed to get vault client: %s", clientErr)
	}

	backendEnv, backendErr := terraform.GetJwtBackend(ctx, backend, string(target), target.TargetName(), client)
	if backendErr != nil {
		return fmt.Errorf("Failed to get terraform backend values: %s", backendErr)
	}
	maps.Copy(runEnv, backendEnv)

	// Initalize the terraform directory
	fmt.Println("Running terraform init")
	if initErr := rootModule.Init(ctx, runEnv); initErr != nil {
		return fmt.Errorf("Failed to run init command: %s", initErr)
	}
	fmt.Println("Completed terraform init")

	provEnv, queryErr := terraform.ProvidersEnviron(ctx, terraformProviders, client)
	if queryErr != nil {
		return fmt.Errorf("Failed to get provider values: %s", queryErr)
	}
	maps.Copy(runEnv, provEnv)

	// Run the terraform command and pass in the remaining arguments
	if runErr := rootModule.Run(ctx, runEnv, atlantisRunArgs(args, planFile)...); runErr != nil {
		return fmt.Errorf("Failed to run terraform %s values: %s", args, runErr)
	}

	// run terraform show -json on the resulting plan
	if args[0] == "plan" {
		writeShowErr := rootModule.WritePlanToShowFile(ctx, runEnv, planFile, showFile)
		return writeShowErr
	}

	return nil
}

// atlantisRunArgs returns a set of arguments for the Terraform CLI
// that will ensure it runs in an Atlantis environment
func atlantisRunArgs(args []string, planFile string) []string {
	switch args[0] {
	case "init":
		return append(args, "-input=false")

	case "plan":
		return append(args, "-input=false", "-refresh", "-out", planFile)

	case "apply":
		return append(args, planFile)

	default:
		return args
	}
}
