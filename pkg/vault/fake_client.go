package vault

import (
	"context"
	"github.com/hashicorp/vault/api"
	"io"
)

var (
	_ Client = &FakeClient{}
)

type FakeClient struct{}

func (f *FakeClient) GetIdentityToken(ctx context.Context, project string) (string, error) {
	return "identity-token", nil
}
func (f *FakeClient) GetKvSecret(ctx context.Context, path string) (map[string]interface{}, error) {
	return nil, nil
}
func (f *FakeClient) ReadSnapshot(ctx context.Context, output io.Writer) error {
	return nil
}
func (f *FakeClient) GetConfig() *api.Config {
	return nil
}
func (f *FakeClient) GetToken() string {
	return "token"
}
