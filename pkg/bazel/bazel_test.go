package bazel_test

import (
	"reflect"
	"testing"

	"github.com/ohno-cloud/rules_tf/pkg/bazel"
)

func TestLabel_ToPath(t *testing.T) {
	type test struct {
		input string
		want  string
	}

	tests := []test{
		{input: "//a/b/c", want: "/test/a/b/c"},
		{input: "//a/b/c:target", want: "/test/a/b/c"},
		{input: "//a", want: "/test/a"},
		{input: "//:target", want: "/test"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(tt *testing.T) {
			input := bazel.Label(tc.input)
			got := input.ToWorkspacePath("/test")
			if !reflect.DeepEqual(tc.want, got) {
				tt.Fatalf("expected: %v, got: %v", tc.want, got)
			}
		})
	}
}

func TestLabel_TargetName(t *testing.T) {
	type test struct {
		input string
		want  string
	}

	tests := []test{
		{input: "//a/b/c", want: "c"},
		{input: "//a/b/c:target", want: "target"},
		{input: "//a", want: "a"},
		{input: "//:target", want: "target"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(tt *testing.T) {
			input := bazel.Label(tc.input)
			got := input.TargetName()
			if !reflect.DeepEqual(tc.want, got) {
				tt.Fatalf("expected: %v, got: %v", tc.want, got)
			}
		})
	}
}
