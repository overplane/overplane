# Overplane Go Core Agent Guide

This file is canonical for work under `go-core/`.

- Modules: stdlib-first; new dependencies need justification. One package has one concern. `internal/platform/*` is app-agnostic, `internal/cli` is wiring, and domain logic belongs in its own packages. No package cycles.
- Subcommands: git-style via the registry; no Cobra/Viper/Urfave. New commands register in `commandRegistry`, implement a command interface (`ConfigOptionalCommand` for commands that run with or without a project `overplane.yaml`), provide `color.HelpSpec`, and map failures to the exit-code table. Commands with config-required semantics (today: the `agent` group) call the shared `requireProject` helper in `internal/cli`; a third consumer should trigger promoting it to a dispatcher-level `ConfigRequiredCommand` interface.
- Containers: engine access goes through `internal/container` (docker/podman CLI clients behind a `Client` interface; nerdctl/k3s are stubs); image planning (fragments, Dockerfile, build hash) is `internal/recipes`' job, driven by the embedded `static/files/recipes/recipes.yaml` registry (schema-validated, drift-tested against `config/data/recipes.yaml`). No real daemon in the default test tier — use the fake runner or PATH shims; real-engine tests live behind the opt-in e2e tag.
- Project config: `overplane.yaml` is owned by `internal/project` (discovery, defaults, strict three-pass validation) against the embedded `static/files/schema/overplane.schema.json`, which must stay byte-identical to `config/schema/overplane.schema.json` (drift-tested).
- Errors: use `errors.Is`/`As`; export sentinels where callers branch; user-facing remediation carries `Hint() string`; thread `context.Context` through blocking operations.
- Logging: use `internal/platform/log`; never stdlib `log`; diagnostics go to stderr, command output to stdout. Structured keys: `step`, `action`, `path`, `err`, `hint`, `source`.
- Serialization: all emitted JSON goes through `serde/canonjson`; all emitted YAML goes through `serde/yamlcanon`.
- Hashing: all content hashes use `internal/platform/hashutil`: SHA-256, first 12 hex chars; hash canonical encodings for structured data.
- Time: all timestamps use `internal/platform/timeutil`: UTC RFC3339; no raw `time.Now()` outside that package; tests use `Clock`.
- Dependencies: add or upgrade with `go get <module>@latest && go mod tidy`.
- Color/tables/TUI: use `internal/platform/color` and `internal/platform/tui`; no ad-hoc ANSI, go-pretty, Bubble Tea, or Huh in command code.
- CLI root help: use `internal/platform/clihelp` for banner/header/usage/commands/global flags. The splash banner belongs below the build-date line and above Usage. Keep command and flag columns padded by the shared renderer; do not hand-roll root help spacing.
- Tests: table-driven by default, hermetic env via `t.Setenv`, real files in `t.TempDir`, listeners on `127.0.0.1:0`, no fixed ports, no real `$HOME`/XDG/ambient `PATH`.
- Quality: do not loosen gates, lower coverage, or skip linters. Fix root causes. Run `make ci` and `make ci-rehearse` before merge. On `main`, use the `/mainmend` skill (`.cursor/skills/mainmend/`) to push and keep GitHub Actions green.
- Commits: user-facing impact in the title; body bullets emphasize why/impact.
- Embedded assets: edit `static/` or `config/data/theme.yaml`, then run `go generate ./...` and commit regenerated `assets_gen.go`.
