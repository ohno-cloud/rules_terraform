package bazel

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/ohno-cloud/rules_tf/pkg/bazel"
)

var (
	_ bazel.Queries = &fakeBzl{}
)

type fakeBzl struct {
	workspace  string
	queryFiles []string
}

func (f *fakeBzl) QueryMaxRank(ctx context.Context, args ...string) (bazel.MaxRankResult, error) {
	return bazel.MaxRankResult{}, nil
}
func (f *fakeBzl) QueryLocation(ctx context.Context, args ...string) (bazel.LocationResult, error) {
	return bazel.LocationResult{}, nil
}
func (f *fakeBzl) QueryXml(ctx context.Context, args ...string) (bazel.XmlResult, error) {
	return bazel.XmlResult{}, nil
}

func (f *fakeBzl) QueryStarlarkJson(ctx context.Context, target any, args ...string) error {
	return nil
}

func (f *fakeBzl) QueryFiles(ctx context.Context, args ...string) ([]string, error) {
	if len(f.queryFiles) == 0 {
		return []string{}, fmt.Errorf("oh no")
	}

	return f.queryFiles, nil
}
func (f *fakeBzl) Build(ctx context.Context, label bazel.Label) error {
	return nil
}
func (f *fakeBzl) Run(ctx context.Context, label bazel.Label, env map[string]string, args ...string) error {
	return nil
}
func (f *fakeBzl) Workspace() string {
	return f.workspace
}

func TestQueryRootModule(t *testing.T) {
	type test struct {
		label          string
		bzlQueryResult []string

		want    rootModuleExec
		wantErr error
	}

	tests := []test{
		{
			label: "//terraform/vault",
			bzlQueryResult: []string{
				"bazel-out/fastbuild/bin/terraform/vault/vault_run_wrapper",
			},
			want: rootModuleExec{
				WorkingDir: "/workspace/bazel-out/fastbuild/bin/terraform/vault/vault_run_wrapper.runfiles/_main",
				Script:     "/workspace/bazel-out/fastbuild/bin/terraform/vault/vault_run_wrapper",
			},
		},

		{
			label:          "//terraform/vault",
			bzlQueryResult: []string{},
			wantErr:        ErrQueryError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.label, func(tt *testing.T) {
			got, err := QueryRootModule(context.TODO(), &fakeBzl{
				workspace:  "/workspace",
				queryFiles: tc.bzlQueryResult,
			}, bazel.Label(tc.label))

			if err != nil {
				if tc.wantErr == nil {
					tt.Fatalf("got an unexpected error: %v", err)
				} else if !errors.Is(err, tc.wantErr) {
					tt.Fatalf("not the expected error. got=%v ; wanted=%v", err, tc.wantErr)
				}
			} else if !reflect.DeepEqual(&tc.want, got) {
				tt.Fatalf("unexpected result, wanted=%v ; got=%v", tc.want, got)
			}
		})
	}

}
