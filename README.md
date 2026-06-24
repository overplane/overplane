# Overplane
| Component | Build | Toolchain |
| --------- | ----- | --------- |
[![ci](https://github.com/overplane/overplane/actions/workflows/ci.yml/badge.svg)](https://github.com/overplane/overplane/actions/workflows/ci.yml) | `ubuntu-latest` | Go 1.26.4 |
[![release](https://github.com/overplane/overplane/actions/workflows/release.yml/badge.svg)](https://github.com/overplane/overplane/actions/workflows/release.yml) | `ubuntu-latest` | Go 1.26.4 |
[![npm-publish](https://github.com/overplane/overplane/actions/workflows/npm-publish.yml/badge.svg)](https://github.com/overplane/overplane/actions/workflows/npm-publish.yml) | `ubuntu-latest` | Go 1.26.4 |

## [**Documentation**](https://www.overplane.dev/docs)

## Commands

| Command | Description |
| ------- | ----------- |
| `overplane init` | Initialize an Overplane project: create and validate `overplane.yaml`, ensure it is not gitignored, and verify the working directory is writable. Idempotent and non-destructive. |
| `overplane setup` | Validate `overplane.yaml`, run project-aware system checks, and build (or reuse) the agent container image. |
| `overplane shell` | Open an ephemeral interactive shell inside the agent image (alias for `agent shell`). |
| `overplane version` | Print build version information. |
| `overplane check` | Run local system checks (container engines, API keys, git). Passes with at least one working engine; API keys are optional. |
| `overplane config validate` | Validate repository configuration files, a project `overplane.yaml`, or a `recipes.yaml` registry. |
| `overplane agent setup` | Build (or reuse) the project's agent container image from the `agent` section of `overplane.yaml`. |
| `overplane agent shell` | Open an ephemeral interactive shell inside the agent image; no host mounts, discarded on exit. |
| `overplane agent list-images` | List this project's agent container images. |
| `overplane theme preview` | Preview the resolved terminal theme. |
| `overplane demo` | Run a sample interactive TUI. |
