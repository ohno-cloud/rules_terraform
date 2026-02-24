package bazel

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/ohno-cloud/rules_terraform/pkg/bazel"
	"github.com/ohno-cloud/rules_terraform/pkg/terraform"
)

var (
	ErrQueryError = errors.New("Failed to xml query for providers")
)

func QueryTfProviders(ctx context.Context, bzl bazel.Queries, label bazel.Label) ([]terraform.ProviderName, error) {
	providers := make([]terraform.ProviderName, 0)

	results, queryErr := bzl.QueryXml(ctx, fmt.Sprintf("kind(terraform_provider, deps(kind(terraform_root_module,%s)))", label))
	if queryErr != nil {
		return nil, errors.Join(ErrQueryError, queryErr)
	}

	for _, rule := range results.Rule {
		for _, str := range rule.String {
			if str.Name == "source" {
				value := strings.TrimSpace(str.Value)
				if _, errProvider := terraform.ProviderFactory(value, nil); errProvider != nil {
					return nil, fmt.Errorf("unknown provider: '%s'", value)
				} else {
					providers = append(providers, terraform.ProviderName(value))
				}
			}
		}
	}

	return providers, nil
}

func QueryRootModule(ctx context.Context, bzl bazel.Queries, label bazel.Label) (terraform.RootModule, error) {
	scripts, queryErr := bzl.QueryFiles(ctx, fmt.Sprintf("kind(terraform_root_module, %s)", string(label)))
	if queryErr != nil {
		return nil, errors.Join(fmt.Errorf("Failed to query for terraform_root_module of %s", label), ErrQueryError, queryErr)
	}

	if len(scripts) == 0 {
		return nil, fmt.Errorf("%s is not a terraform_root_module", label)
	} else if len(scripts) > 1 {
		return nil, fmt.Errorf("expected only 1 terraform_root_module from %s, got %d", label, len(scripts))
	}
	script := strings.TrimSpace(scripts[0])

	baseDir := bzl.Workspace()
	rootModule := rootModuleExec{
		WorkingDir: path.Join(baseDir, script+".runfiles", "_main"),
		Script:     path.Join(baseDir, script),
	}

	return &rootModule, nil
}
