# Contributing

Use `make ci` as the source of truth. The CLI is stdlib-first, git-style, and keeps reusable platform behavior under `internal/platform`.

- Add dependencies with `go get <module>@latest && go mod tidy`.
- Add commands through `internal/cli` registry and command interfaces.
- Use `internal/platform/log`, `serde/canonjson`, `serde/yamlcanon`, `hashutil`, `timeutil`, `color`, and `tui` instead of ad-hoc equivalents.
- Keep tests hermetic: temp dirs, explicit env, loopback `:0` listeners, no external network.
- Update embedded assets with `go generate ./...` after editing `static/` or `config/data/theme.yaml`.
