# `atlantis-bzl`

A CLI tool that is used with the `multienv` step of an `atlantis` workflow and injects required credentials into
the environment to be used by terraform.

## Notes

- [Has the environment variables from as a custom `run` step](https://www.runatlantis.io/docs/custom-workflows.html#custom-run-command)
- > The result of the executed command must have a fixed format: `EnvVar1Name=value1,EnvVar2Name=value2,EnvVar3Name=value3`
- Source code: https://github.com/runatlantis/atlantis/blob/main/server/core/runtime/multienv_step_runner.go
