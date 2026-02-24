package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/ohno-cloud/rules_tf/pkg/atlantis"
	"github.com/ohno-cloud/rules_tf/pkg/bazel"
	tfbzl "github.com/ohno-cloud/rules_tf/pkg/terraform/bazel"

	yaml "gopkg.in/yaml.v3"
)

// Resplaced using x_def
var TerraformVersion = "1.0.0"

type RepoCmd struct{}

func (r *RepoCmd) Run(c Common) error {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(c.Timeout))
	defer cancel()

	bzl, bzlErr := bazel.NewBazel(c.BazelDir)
	if bzlErr != nil {
		return fmt.Errorf("Failed to setup Bazel: %s", bzlErr)
	}

	atlantisPath := path.Join(c.BazelDir, "atlantis.yaml")

	results, queryErr := bzl.QueryMaxRank(ctx, "kind(terraform_root_module,//terraform/...)")
	if queryErr != nil {
		return fmt.Errorf("Failed to max-rank query: %s", queryErr)
	}

	var repo atlantis.RepoConfig
	repo.Version = 3
	repo.ParallelPlan = true

	for order, bucket := range results.Ranking {

		for _, result := range bucket {
			var project atlantis.Project

			var provider tfbzl.TerraformWorkspaceProvider

			if err := bzl.QueryStarlarkJson(
				ctx,
				&provider,
				"--starlark:expr",
				tfbzl.TerraformWorkspaceProviderStarlarkExpr,
				string(result),
			); err != nil {
				return fmt.Errorf("failed to query %s: %w", result, err)
			}

			project.Directory = bazel.LabelToPath(result)
			project.Workspace = provider.Workspace
			project.ExecutionOrderGroup = order
			project.TerraformVersion = TerraformVersion

			locs, locErr := bzl.QueryLocation(ctx, fmt.Sprintf("deps(%s)", string(result)))
			if locErr != nil {
				return fmt.Errorf("Failed to query source files of %s: %s", result, locErr)
			}

			project.AutoPlan.Enabled = true
			project.AutoPlan.WhenModified = make([]string, 0, len(locs.Locations))

			sourceDir := c.BazelDir
			if !path.IsAbs(sourceDir) {
				if cwd, err := os.Getwd(); err == nil {
					sourceDir = path.Join(cwd, sourceDir)
				}
			}

			for _, loc := range locs.Locations {
				if !strings.HasPrefix(loc.Path, sourceDir) {
					continue
				}

				if relpath, relErr := filepath.Rel(path.Join(sourceDir, project.Directory), loc.Path); relErr == nil {
					if strings.HasPrefix(loc.Path, c.BazelDir) {
						project.AutoPlan.WhenModified = append(project.AutoPlan.WhenModified, relpath)
					}
				} else {
					fmt.Printf("skipping %s as it had the error: %s\n", loc.Path, relErr)
				}
			}

			repo.Projects = append(repo.Projects, project)
		}
	}

	file, openErr := os.OpenFile(atlantisPath, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if openErr != nil {
		return fmt.Errorf("Failed to open file %s: %s", atlantisPath, openErr)
	}
	defer file.Close()

	encoder := yaml.NewEncoder(io.MultiWriter(file, os.Stdout))
	defer encoder.Close()

	if yamlErr := encoder.Encode(repo); yamlErr != nil {
		return fmt.Errorf("Failed to marshal yaml: %s", yamlErr)
	}

	return nil
}
