package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/alecthomas/kong"

	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"

	"github.com/ohno-cloud/rules_terraform/pkg/vault"
)

type CLI struct {
	// General configuration
	Timeout time.Duration `help:"Timeout for commands" default:"5m" env:"ATLATNIS_BAZEL_TIMEOUT"`
	// Bazel related configuration
	BazelDir string `help:"Path for the Bazel directory" env:"DIR,BUILD_WORKING_DIRECTORY,PWD"`
	// SPIFFE related configuration
	SpiffeAudience string `help:"The audience for the SPIFFE SVID to use" env:"SPIFFE_AUDIENCE"`

	// Vault related configuration
	VaultMount string `help:"The Vault mount for the authentication method" default:"kubernetes" env:"VAULT_AUTH_MOUNT"`
	VaultRole  string `env:"VAULT_K8S_ROLE,VAULT_ROLE" optional:""`

	// sub commands
	Repo  RepoCmd  `cmd:"" help:"Generate atlantis repository file"`
	Run   RunCmd   `cmd:"" help:"Run terraform commands in Atlantis"`
	Local LocalCmd `cmd:"" help:"Run terraform commands locally"`
}

type Common struct {
	BazelDir       string
	SpiffeAudience string
	VaultMount     string
	VaultRole      string
	Timeout        time.Duration
}

func main() {
	var cli CLI
	kCtx := kong.Parse(&cli)

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	slog.Debug("Got bazel dir", "bazelDir", cli.BazelDir)

	err := kCtx.Run(Common{
		BazelDir:       cli.BazelDir,
		SpiffeAudience: cli.SpiffeAudience,
		VaultRole:      cli.VaultRole,
		VaultMount:     cli.VaultMount,
		Timeout:        cli.Timeout,
	})

	kCtx.FatalIfErrorf(err)
}

func (c *Common) getVaultClient(ctx context.Context) (vault.Client, error) {
	if c.VaultRole != "" {
		slog.Debug("vault role detected", "vault-role", c.VaultRole)
		if c.SpiffeAudience != "" {
			slog.Debug("spiffe role selected", "spiffe-audience", c.SpiffeAudience, "vault-mount", c.VaultMount)
			svid, err := workloadapi.FetchJWTSVID(ctx, jwtsvid.Params{
				Audience: c.SpiffeAudience,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to get JWT SVID: %w", err)
			}

			slog.Info("received spiffe id", "spiffe", svid.ID.String())
			return vault.GetJWTClient(ctx, c.VaultRole, svid.Marshal(), c.VaultMount)
		} else {
			slog.Info("using kubernetes-based client", "vault-role", c.VaultRole)
			return vault.GetKubernetesClient(ctx, c.VaultRole)
		}
	} else {
		slog.Debug("no explicit vault config detected, using default client")
		return vault.GetDefaultClient(ctx)
	}
}
