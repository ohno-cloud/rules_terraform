package atlantis

type RepoConfig struct {
	Version      int
	Projects     []Project `yaml:"projects"`
	AutoMerge    bool      `yaml:"automerge"`
	ParallelPlan bool      `yaml:"parallel_plan"`
}

type Project struct {
	Name                string `yaml:"name,omitempty"`
	Directory           string `yaml:"dir"`
	Workspace           string `yaml:"workspace"`
	ExecutionOrderGroup int    `yaml:"execution_order_group"`
	TerraformVersion    string `yaml:"terraform_version"`
	AutoPlan            AutoPlan
}

type AutoPlan struct {
	Enabled      bool
	WhenModified []string `yaml:"when_modified"`
}
