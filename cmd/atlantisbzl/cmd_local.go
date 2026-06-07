package main

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/ohno-cloud/rules_terraform/pkg/bazel"
)

type LocalCmd struct {
	Label string `arg:"" required:""`

	Args []string `arg:""`

	// Environment variables passed through ConfigMap
	TerraformBackend string `short:"b" env:"TERRAFORM_BACKEND" required:""`
}

func (r *LocalCmd) Run(c Common) error {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(c.Timeout))
	defer cancel()

	bzl, bzlErr := bazel.NewBazel(c.BazelDir)
	if bzlErr != nil {
		return fmt.Errorf("Failed to setup Bazel: %s", bzlErr)
	}

	// Subpath and workspace are components of the bazel target label
	label := bazel.Label(r.Label)

	// Setup the backend environment
	baseURL, urlErr := url.Parse(r.TerraformBackend)
	if urlErr != nil {
		return fmt.Errorf("Failed to parse URL %s : %s", r.TerraformBackend, urlErr)
	}

	return c.tfRunner(ctx, label, bzl, c.BazelDir, baseURL, r.Args, "show", "plan")
}
