package terraform_test

import (
	"context"
	"errors"
	"net/url"
	"reflect"
	"testing"

	"github.com/ohno-cloud/rules_tf/pkg/terraform"
	"github.com/ohno-cloud/rules_tf/pkg/vault"
)

func TestGetBackend(t *testing.T) {
	type test struct {
		backend   *url.URL
		workspace string
		label     string
		want      map[string]string
		wantErr   error
	}

	defaultUrl, _ := url.Parse("https://test.local")

	tests := []test{
		{
			backend:   defaultUrl,
			workspace: "test",
			label:     "@@//simple/path:path",

			want: map[string]string{
				terraform.EnvHttpAddress:       defaultUrl.JoinPath("state/test/simple^path~path").String(),
				terraform.EnvHttpLockAddress:   defaultUrl.JoinPath("state/test/simple^path~path").String(),
				terraform.EnvHttpUnlockAddress: defaultUrl.JoinPath("state/test/simple^path~path").String(),
				terraform.EnvHttpUserName:      "jwt",
				terraform.EnvHttpPassword:      "identity-token",
			},
		},
		{
			backend:   defaultUrl,
			workspace: "test",
			label:     "@@//:single",

			want: map[string]string{
				terraform.EnvHttpAddress:       defaultUrl.JoinPath("state/test/~single").String(),
				terraform.EnvHttpLockAddress:   defaultUrl.JoinPath("state/test/~single").String(),
				terraform.EnvHttpUnlockAddress: defaultUrl.JoinPath("state/test/~single").String(),
				terraform.EnvHttpUserName:      "jwt",
				terraform.EnvHttpPassword:      "identity-token",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.label, func(tt *testing.T) {
			env, err := terraform.GetJwtBackend(context.TODO(),
				tc.backend,
				tc.label,
				tc.workspace,
				&vault.FakeClient{},
			)

			if err != nil {
				if tc.wantErr == nil {
					tt.Fatalf("got an unexpected error: %v", err)
				} else if !errors.Is(err, tc.wantErr) {
					tt.Fatalf("not the expected error. got=%v ; wanted=%v", err, tc.wantErr)
				}
			}

			if !reflect.DeepEqual(env, tc.want) {
				tt.Fatalf("not the expected output. got=%v ; wanted=%v", env, tc.want)
			}
		})
	}

}
