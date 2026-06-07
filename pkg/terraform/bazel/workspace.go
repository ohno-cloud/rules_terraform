package bazel

const (
	TerraformWorkspaceProviderStarlarkExpr = "json.encode(providers(target)[\"@@rules_terraform~//internal:modules.bzl%TerraformWorkspaceInfo\"])"

	TerraformRootModuleDepsStarlarkExpr = "json.encode(providers(target)[\"@@rules_terraform~//internal:modules.bzl%TerraformRootModuleInfo\"].deps)"
)

// Refects the @rules_terraform//internal:TerraformWorkflowProvider
type TerraformWorkspaceProvider struct {
	Tfvars    []string `json:"tfvars"`
	Workspace string   `json:"workspace"`
}

type TerraformRootModuleDep = struct {
	Label     string `json:"label"`
	Workspace string `json:"workspace"`
}
