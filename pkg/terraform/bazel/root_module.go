package bazel

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ohno-cloud/rules_terraform/pkg/terraform"
)

var (
	_ terraform.RootModule = &rootModuleExec{}
)

// rootModuleExec is an implemenator of terraform.RootModule for use
// in a Bazel context
type rootModuleExec struct {
	WorkingDir string
	Script     string
}

func (t *rootModuleExec) WritePlanToShowFile(ctx context.Context, env map[string]string, planFile, showFile string) error {
	workingDir, wdErr := filepath.EvalSymlinks(t.WorkingDir)
	if wdErr != nil {
		return wdErr
	}

	file, fileErr := os.OpenFile(showFile, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if fileErr != nil {
		return fileErr
	}
	defer file.Close()
	defer file.Sync()

	script, scriptErr := filepath.EvalSymlinks(t.Script)
	if scriptErr != nil {
		return scriptErr
	}

	cmd := exec.CommandContext(ctx, script, "show", "-json", planFile)
	cmd.Dir = workingDir

	for key, val := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, val))
	}

	cmd.Stdout = file
	cmd.Stderr = os.Stderr

	startErr := cmd.Start()
	if startErr != nil {
		return startErr
	}

	return cmd.Wait()
}

func (t *rootModuleExec) Init(ctx context.Context, env map[string]string) error {
	workingDir, wdErr := filepath.EvalSymlinks(t.WorkingDir)
	if wdErr != nil {
		return wdErr
	}

	script, scriptErr := filepath.EvalSymlinks(t.Script)
	if scriptErr != nil {
		return scriptErr
	}

	cmd := exec.CommandContext(ctx, script, "init", "-reconfigure", "-lock=false")
	cmd.Dir = workingDir

	for key, val := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, val))
	}

	var stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	startErr := cmd.Start()
	if startErr != nil {
		return startErr
	}

	waitErr := cmd.Wait()

	if waitErr != nil {
		fmt.Fprintln(os.Stderr, "An error occurred while running terraform init")
		fmt.Fprintln(os.Stderr, "Error:", waitErr)
		fmt.Fprintln(os.Stderr, "stdout")
		io.Copy(os.Stdout, &stdoutBuf)
		fmt.Fprintln(os.Stderr, "stderr")
		io.Copy(os.Stderr, &stderrBuf)
	}

	return waitErr
}

func (t *rootModuleExec) Run(ctx context.Context, env map[string]string, args ...string) error {
	workingDir, wdErr := filepath.EvalSymlinks(t.WorkingDir)
	if wdErr != nil {
		return wdErr
	}

	script, scriptErr := filepath.EvalSymlinks(t.Script)
	if scriptErr != nil {
		return scriptErr
	}

	cmd := exec.CommandContext(ctx, script, args...)
	cmd.Dir = workingDir

	for key, val := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, val))
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	startErr := cmd.Start()
	if startErr != nil {
		return startErr
	}

	return cmd.Wait()
}
