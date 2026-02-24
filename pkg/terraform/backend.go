package terraform

import (
	"context"
	"net/url"
	"strings"

	"github.com/ohno-cloud/rules_tf/pkg/vault"
)

const (
	EnvHttpAddress       = "TF_HTTP_ADDRESS"        // "${tf_dns}/state/${project}/${state}"
	EnvHttpLockAddress   = "TF_HTTP_LOCK_ADDRESS"   // "${tf_dns}/state/${project}/${state}"
	EnvHttpUnlockAddress = "TF_HTTP_UNLOCK_ADDRESS" // "${tf_dns}/state/${project}/${state}"
	EnvHttpUserName      = "TF_HTTP_USERNAME"       // 'jwt'
	EnvHttpPassword      = "TF_HTTP_PASSWORD"       // $(vault read -field token "identity/oidc/token/${project}")
)

type BackendConfig struct {
	Config HttpBackendConfig `json:"config"`
}

type HttpBackendConfig struct {
	Address string `json:"address"`
}

func LabelToStatePath(backend *url.URL, label, workspace string) string {
	stateFilter := strings.Replace(strings.TrimPrefix(label, "@@//"), ":", "~", -1)
	stateFilter = strings.Replace(stateFilter, "/", "^", -1)

	stateUrl := backend.JoinPath("state", workspace, stateFilter)

	return stateUrl.String()
}

func GetJwtBackend(ctx context.Context, backend *url.URL, label, workspace string, client vault.Client) (map[string]string, error) {
	env := make(map[string]string, 0)

	stateUrl := LabelToStatePath(backend, label, workspace)

	token, tokenErr := client.GetIdentityToken(ctx, workspace)
	if tokenErr != nil {
		return env, tokenErr
	}

	env[EnvHttpAddress] = stateUrl
	env[EnvHttpLockAddress] = stateUrl
	env[EnvHttpUnlockAddress] = stateUrl
	env[EnvHttpUserName] = "jwt"
	env[EnvHttpPassword] = token

	return env, nil
}
