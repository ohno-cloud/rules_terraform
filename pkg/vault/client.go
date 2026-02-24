package vault

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/hashicorp/vault/api"
	k8sAuth "github.com/hashicorp/vault/api/auth/kubernetes"
)

var (
	_ Client = &vaultClient{}
)

type Client interface {
	GetIdentityToken(ctx context.Context, project string) (string, error)
	GetKvSecret(ctx context.Context, path string) (map[string]interface{}, error)

	// Pass in an io.WriteCloser
	ReadSnapshot(ctx context.Context, output io.Writer) error

	GetConfig() *api.Config
	GetToken() string
}

type vaultClient struct {
	vault *api.Client
}

func (v *vaultClient) GetConfig() *api.Config {
	return v.vault.CloneConfig()
}

func (v *vaultClient) GetToken() string {
	return v.vault.Token()
}

func (v *vaultClient) GetIdentityToken(ctx context.Context, workspace string) (string, error) {
	result, err := v.vault.Logical().ReadWithContext(ctx, strings.Join([]string{"identity/oidc/token", fmt.Sprintf("terraform-workspace-%s", workspace)}, "/"))
	if err != nil {
		return "", err
	}

	return result.Data["token"].(string), nil
}

func (v *vaultClient) GetKvSecret(ctx context.Context, path string) (map[string]interface{}, error) {
	result, err := v.vault.Logical().ReadWithContext(ctx, strings.Join([]string{"secret/data", path}, "/"))
	if err != nil {
		return nil, err
	}

	return result.Data["data"].(map[string]interface{}), nil
}

func (v *vaultClient) ReadSnapshot(ctx context.Context, output io.Writer) error {
	return v.vault.Sys().RaftSnapshot(output)
}

func GetDefaultClient(ctx context.Context) (Client, error) {
	config := api.DefaultConfig()

	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize Vault client: %w", err)
	}

	return &vaultClient{
		vault: client,
	}, nil
}

func GetTokenClient(ctx context.Context, token string) (Client, error) {
	config := api.DefaultConfig()

	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize Vault client: %w", err)
	}

	client.SetToken(token)

	return &vaultClient{
		vault: client,
	}, nil
}

func GetKubernetesClient(ctx context.Context, role string) (Client, error) {
	config := api.DefaultConfig()

	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize Vault client: %w", err)
	}

	k8sAuth, err := k8sAuth.NewKubernetesAuth(role)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize Kubernetes auth method: %w", err)
	}

	authInfo, err := client.Auth().Login(ctx, k8sAuth)
	if err != nil {
		return nil, fmt.Errorf("unable to log in with Kubernetes auth: %w", err)
	}
	if authInfo == nil {
		return nil, fmt.Errorf("no auth info was returned after login")
	}

	return &vaultClient{
		vault: client,
	}, nil
}

func GetJWTClient(ctx context.Context, role string, jwt string, mount string) (Client, error) {
	config := api.DefaultConfig()

	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize Vault client: %w", err)
	}

	k8sAuth, err := k8sAuth.NewKubernetesAuth(role,
		k8sAuth.WithServiceAccountToken(jwt),
		k8sAuth.WithMountPath(mount),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize Kubernetes auth method: %w", err)
	}

	authInfo, err := client.Auth().Login(ctx, k8sAuth)
	if err != nil {
		return nil, fmt.Errorf("unable to log in with Kubernetes auth: %w", err)
	}
	if authInfo == nil {
		return nil, fmt.Errorf("no auth info was returned after login")
	}

	return &vaultClient{
		vault: client,
	}, nil
}
