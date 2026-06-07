package terraform

import (
	"context"
)

type RootModule interface {
	Init(ctx context.Context, env map[string]string) error
	Run(ctx context.Context, env map[string]string, args ...string) error
	WritePlanToShowFile(ctx context.Context, env map[string]string, planFile, showFile string) error
}
