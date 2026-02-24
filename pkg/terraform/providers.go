package terraform

import (
	"context"
	"fmt"
	"maps"

	"github.com/ohno-cloud/rules_tf/pkg/vault"
)

type ProviderName = string

type Provider interface {
	GetEnviron(ctx context.Context) (map[string]string, error)
}

func ProviderFactory(name ProviderName, client vault.Client) (Provider, error) {
	switch string(name) {
	case "registry.terraform.io/okta/okta":
		return &OktaProvider{client}, nil

	case "registry.terraform.io/hashicorp/random":
		return &NoOpProvider{}, nil

	case "registry.terraform.io/hashicorp/vault":
		return &VaultProvider{client}, nil

	default:
		return nil, fmt.Errorf("Provider %s is not supported", name)
	}
}

func ProvidersEnviron(ctx context.Context, providers []ProviderName, client vault.Client) (map[string]string, error) {

	env := make(map[string]string, 0)

	for _, providerName := range providers {
		prov, err := ProviderFactory(providerName, client)
		if err != nil {
			return nil, err
		}

		provEnv, provErr := prov.GetEnviron(ctx)
		if provErr != nil {
			return nil, provErr
		}

		maps.Copy(env, provEnv)
	}

	return env, nil
}

type NoOpProvider struct{}

func (v *NoOpProvider) GetEnviron(ctx context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}

type VaultProvider struct {
	client vault.Client
}

func (v *VaultProvider) GetEnviron(ctx context.Context) (map[string]string, error) {
	cfg := v.client.GetConfig()

	return map[string]string{
		"VAULT_ADDR":  cfg.Address,
		"VAULT_TOKEN": v.client.GetToken(),
	}, nil
}

type OktaProvider struct {
	client vault.Client
}

func (o *OktaProvider) GetEnviron(ctx context.Context) (map[string]string, error) {
	result, err := o.client.GetKvSecret(ctx, "terraform/terraform/okta")
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"OKTA_API_TOKEN": result["token"].(string),
	}, nil
}
