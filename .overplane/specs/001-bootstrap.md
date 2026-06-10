---
# Advisory frontmatter (aperture spec-metadata schema, extended).
description: Bootstrap a production-quality Go CLI scaffold with git-style subcommands,
  themed 256-color terminal output, structured logging, JSON-schema-validated YAML
  config, embedded assets, TUI infrastructure, OTEL metrics, env loading, system
  checks, and strict quality gates.
id: '#0001'
parent: []
status: active  # not deprecated
tags:
  - bootstrap
  - cli
  - go
title: Go CLI bootstrap
# --- advisory extensions (not part of the base schema) ---
author: Mayank Lahiri
created: '2026-06-09'
target_dir: go-core
spec_version: 1
---

# Spec #0001 — Go CLI bootstrap

This is a **self-contained build prompt** for an AI coding agent. Executing it
should produce a complete, compiling, tested, lint-clean Go CLI scaffold in the
target directory. The architecture is modeled on a proven production CLI
(git-style subcommands, stdlib-first, themed terminal output, strict gates).
Every requirement below is normative unless marked *advisory*.

---

## 0. Parameters (edit this block ONCE — nothing else)

This spec uses symbolic tokens of the form `{{NAME}}` throughout its body.
**The human never edits those tokens in place.** Instead, the human edits only
the YAML block below; the AI agent executing this spec MUST resolve every
`{{NAME}}` token in this document to the corresponding `params.name` value
before doing anything else. If a required value is still a `FILL_IN`, the
agent must stop and ask rather than guess.

```yaml
params:
  binary: overplane            # {{BINARY}} — CLI binary name, short, lowercase (e.g. opl)
  project: Overplane          # {{PROJECT}} — human-readable project name (e.g. Overplane)
  module: github.com/overplane/overplane          # {{MODULE}} — Go module path (e.g. github.com/overplane/overplane-master)
  description: Evolve verified software.       # {{DESCRIPTION}} — one-line CLI description for help text
  env_prefix: OVERPLANE        # {{ENV_PREFIX}} — env var prefix, uppercase, no trailing underscore (e.g. OPL)
  repo_root_rel: '..'        # {{REPO_ROOT_REL}} — path from the Go module dir to the repo root
  license: Apache-2.0        # {{LICENSE}} — SPDX license shown in banner/help
  api_keys:                  # {{API_KEYS}} — env var names of API keys probed by `check`
    - ANTHROPIC_API_KEY
    - OPENAI_API_KEY
    - GEMINI_API_KEY
  container_engines:         # {{CONTAINER_ENGINES}} — container engines probed by `check`
    - docker
    - podman
  otel_service: overplane      # {{OTEL_SERVICE}} — OTEL service.name resource attribute (usually same as binary)
  coverage_min: 80.0         # {{COVERAGE_MIN}} — coverage floor percent (total/package/file)
  go_version: '1.26'         # {{GO_VERSION}} — pinned Go minor version (latest stable)
  palette: default           # {{PALETTE}} — `default` for the autumn palette in §4.2, or a list of 16 xterm-256 indices
  target_dir: go-core        # {{TARGET_DIR}} — directory of the Go module within the repo
```

Where a token appears inside a path, identifier, or code snippet (e.g.
`cmd/{{BINARY}}/main.go`, `{{ENV_PREFIX}}_LOG_LEVEL`), the agent substitutes
the literal value (e.g. `cmd/opl/main.go`, `OPL_LOG_LEVEL`). Tokens are
always UPPERCASE; GitHub Actions expressions like `${{ github.ref }}` (§14.5)
are not spec tokens and must be left verbatim.

---

## 1. Goal and non-goals

**Goal.** Generate the directory containing this spec's `.overplane/` folder
(i.e. `{{TARGET_DIR}}`, per the frontmatter `target_dir`) as a single Go module that builds
one static CLI binary `{{BINARY}}`, with all the platform subsystems specified
below, ready for feature commands to be added on top.

**Non-goals.** No business logic. No network services. No cobra/viper/urfave.
No web UI. The deliverable is a *scaffold of production quality*, not a demo.

---

## 2. Hard constraints

1. **Go {{GO_VERSION}}** (latest stable). `go.mod` declares `go {{GO_VERSION}}.0`
   and an explicit `toolchain` line for the newest patch release. A CI gate
   script fails the build if the local toolchain minor version differs.
2. **Stdlib-first.** Allowed third-party dependencies, exhaustively:
   - `github.com/santhosh-tekuri/jsonschema` (JSON Schema draft 2020-12; latest major)
   - `gopkg.in/yaml.v3` (YAML with `KnownFields`)
   - `github.com/jedib0t/go-pretty` (tables only; latest major)
   - bubbletea, bubbles, huh, lipgloss (TUI only; use the latest published
     stable major — resolve the current canonical module paths, whether
     `charm.land/...` or `github.com/charmbracelet/...`, at generation time)
   - `golang.org/x/term` (TTY detection)
   - `go.opentelemetry.io/otel` + OTLP gRPC trace/metric exporters + sdk
   - `google.golang.org/grpc` + `go.opentelemetry.io/proto/otlp` (already
     transitive via the exporters; direct use permitted only in the
     `telemetrytest` in-process collector, §18)
   Anything else requires a written justification in the PR description.
   **Versions:** every dependency MUST be added at its latest stable released
   version at generation time via `go get <module>@latest` (then
   `go mod tidy`). Never copy version numbers from this spec or from memory;
   never use pre-release/rc versions; never downgrade to match example code —
   adapt the code to the current API instead.
3. **Git-style subcommands.** Stdlib `flag` package; no command framework.
4. **No stdlib `log`.** All logging through the internal `log` package (§5),
   enforced by a `depguard` lint rule.
5. **Single static binary.** `CGO_ENABLED=0`; assets embedded (§7); no runtime
   file dependencies (theme/config files on disk *override* embedded defaults
   but are never required).
6. **`context.Context` threaded** through every operation that can block.
   Errors handled with `errors.Is` / `errors.As`; sentinel errors exported
   where callers branch on them.

---

## 3. Directory layout

```
config/
├── data/
│   ├── global.yaml                   # project-global constants
│   ├── theme.yaml                    # visual + terminal theme
│   └── infra.yaml                    # infrastructure config
└── schema/
    ├── global.schema.json
    ├── theme.schema.json
    └── infra.schema.json
go-core/                              # Go module root ({{MODULE}})
├── cmd/{{BINARY}}/main.go            # thin entrypoint: flags, logging, telemetry, dispatch
├── pkg/
│   └── config/                       # shared config discovery, schema validation, typed loaders
├── internal/
│   ├── cli/                          # command registry, dispatch, per-command handlers
│   │   ├── cli.go                    # Dispatch, Runner, registry, root help, exit codes
│   │   ├── subcommands.go            # nested subcommand router + usage printers
│   │   ├── check.go                  # system checks command
│   │   ├── configcmd.go              # `config validate` command
│   │   ├── themecmd.go               # `theme preview` command
│   │   └── democmd.go                # example TUI command (`demo`)
│   ├── platform/
│   │   ├── color/                    # palette, ANSI, tables, themed help rendering
│   │   ├── log/                      # slog: pretty + JSON handlers
│   │   ├── tui/                      # shared interactive shortcuts (huh wrappers)
│   │   │   └── nav/                  # reusable list+detail bubbletea navigator
│   │   ├── telemetry/                # OTEL providers (noop unless endpoint set)
│   │   ├── env/                      # .env loader + normalization
│   │   ├── hashutil/                 # SHA-256 short-hash utilities (§6.5)
│   │   ├── timeutil/                 # datetime utilities, injectable clock (§6.6)
│   │   ├── version/                  # Version/Commit/Date vars set via -ldflags
│   │   ├── serde/
│   │   │   ├── jsonschema/           # schema compile + validate → []Problem
│   │   │   ├── yamlstrict/           # two-phase strict YAML decode helpers
│   │   │   ├── canonjson/            # stable JSON encoding, sorted keys (§6.3)
│   │   │   └── yamlcanon/            # deterministic YAML emission (§6.4)
│   │   └── embed/assets/             # generated gzip asset map + virtual fs.FS
│   │       └── cmd/gen/main.go       # go:generate asset compiler
├── static/                           # embed source tree (input to the generator)
│   └── files/
│       └── misc/banner.ans           # ANSI splash banner (binary, pre-colored)
├── cli.sh                            # build-if-stale wrapper (§13)
├── Makefile                          # quality gates (§14)
├── .golangci.yml                     # strict lint config (§14)
├── scripts/
│   ├── checkgoversion                # Go toolchain gate
│   └── coveragegate                  # per-file/per-package/total coverage gate
├── AGENTS.md                         # agent instructions (§15)
├── CONTRIBUTING.md                   # human conventions (condensed from §15)
├── go.mod / go.sum
└── VERSION                           # semver, single line
```

**Note on config:** all project-wide static YAML lives under the repository-root
`config/data/`, and all JSON Schema lives under `config/schema/`. The Go module
does not keep a module-local `config/` directory; sitebuild, infrabuild, and the
CLI all reuse `go-core/internal/platform` primitives with per-build config wrappers.

---

## 4. Terminal color: `internal/platform/color`

### 4.1 Color model

- **xterm-256 indexed color only** (`\x1b[38;5;{idx}m` / `\x1b[48;5;{idx}m`).
  No 24-bit truecolor sequences anywhere in output.
- The app addresses colors through a **16-slot palette**: `type Palette [16]uint8`
  where each slot holds an xterm-256 index (0–255). All call sites use slot
  indices 0–15; slot lookup is masked (`i & 0x0F`) so any int is safe.

### 4.2 API (exact exports)

```go
type Palette [16]uint8

func Get() Palette                 // active palette, lazy-initialized
func Set(p Palette)                // replace (used by theme loader and tests)
func Enabled() bool                // color on/off for this process
func FG(slot int) string           // "\x1b[38;5;Nm" or "" when disabled
func BG(slot int) string
func Sprint(slot int, s string) string        // FG + s + reset, or s
func Fprint(w io.Writer, slot int, s string) (int, error)
func Table(out io.Writer) table.Writer        // go-pretty writer, themed box style
func RenderHelp(spec HelpSpec) string         // §9.3
func ResetForTest()                           // clears cached state
```

Default palette (used when `{{PALETTE}}` is `default` and no theme resolves) —
a warm "autumn" set; the agent may choose 16 harmonious xterm-256 indices in
the warm range (reds/oranges/golds/browns plus a dim gray and a near-white)
and document each slot's role:

| Slot | Role |
|---|---|
| 0 | titles, provenance (file/symbol) |
| 1 | step identifiers |
| 2 | warnings, examples |
| 3 | success |
| 4 | info level, flag names |
| 5 | accents |
| 6 | debug level |
| 7 | dim/secondary text |
| 8 | placeholders |
| 9–15 | rotating slots for hashed key coloring (§5.2) |

### 4.3 Theme: `config/data/theme.yaml`

Canonical theme file at the **repo root** `config/data/theme.yaml`, schema-validated
against `config/schema/theme.schema.json` through the platform config loader (strict;
`additionalProperties: false`).

Schema (YAML form shown; encode as JSON Schema draft 2020-12):

```yaml
terminal:
  name: string            # required, 1-64 chars
  palette: [int x 16]     # required, exactly 16 ints, each 0-255
  log:                    # optional per-concern slot overrides (ints 0-15)
    error: int
    warn: int
    info: int
    debug: int
```

**Resolution order** (first hit wins), executed once at process start:

1. `{{ENV_PREFIX}}_THEME` env var → explicit path to a full theme YAML (error
   if invalid).
2. Walk upward from CWD looking for `config/data/theme.yaml` (repo-root discovery,
   same upward-walk used for app config discovery, §8.2).
3. Built-in terminal palette fallback if no repository config can be discovered.

A discovered theme that fails schema validation falls back to the built-in
terminal palette. An explicit `{{ENV_PREFIX}}_THEME` error fails resolution.
After resolution, `color.Set` is called with the palette and the log package
picks up level-slot overrides.

### 4.4 Enable/disable

Resolved once, cached, overridable via `ResetForTest`:

1. `NO_COLOR` set (any value) → disabled.
2. `CLICOLOR_FORCE` set and ≠ `"0"` → enabled.
3. Otherwise enabled iff stdout **or** stderr is a TTY (`golang.org/x/term`).

When disabled, every API returns plain text — zero escape bytes in output.

### 4.5 Tables

`color.Table(out)` returns a `go-pretty` `table.Writer` preconfigured with a
named custom style: Unicode box-drawing borders (`┌─┐│└┘├┤┬┴┼`), header
separator row, no zebra striping. **All tabular output in the app goes through
this helper** — never ad-hoc go-pretty setup in command code. Cell coloring is
the caller's job via `color.Sprint`.

---

## 5. Structured logging: `internal/platform/log`

Built on `log/slog` with two custom handlers. The stdlib `log` package is
banned repo-wide via depguard except inside this package.

### 5.1 API

```go
const (
    FormatPretty = "pretty"
    FormatJSON   = "json"
)

func Configure(format, level string, w io.Writer, verbose bool) (*slog.Logger, error)
    // validates inputs, builds handler, sets slog.SetDefault, returns logger
func New(format, level string, w io.Writer, verbose bool) (*slog.Logger, error)
func Default() *slog.Logger                     // lazy pretty/info fallback
func WithContext(ctx context.Context, l *slog.Logger) context.Context
func FromContext(ctx context.Context) *slog.Logger
func StripANSI(s string) string                 // test helper
```

### 5.2 Pretty handler

Line format (single line + optional hint line):

```
2026-06-09T21:14:03Z INFO  |message padded to 45 cols              | key=value key2=value2
                                                                     ↳ hint text (if `hint` attr present)
```

- **Timestamp**: RFC3339 UTC, rendered dim.
- **Level**: uppercase, colored via theme slots — error→slot 0 (or theme
  `log.error`), warn→2, info→4, debug→6.
- **Message**: padded to 45 chars inside `|…|` for info/warn/error; debug lines
  use plain unpadded `key=value` form with no color.
- **Attributes**: `key=value`; the key's color slot is chosen by FNV-1a hash of
  the key name mod 16, so a given key is always the same color. Special-cased:
  - `file`, `symbol` → slot 0 (provenance)
  - `step` → slot 1
  - `err` / `error` → fixed bright red (xterm 196), not palette
  - `result` / `action` → fixed yellow (xterm 226)
  - `hint` → not inline; rendered on a second line, dim, prefixed `↳`
- **Verbose mode** (`-v`): derive `file` (basename:line) and `symbol` (short
  func name) from the record PC via `runtime.CallersFrames` and prepend them in
  fixed order: `file`, `symbol`, `step`, then everything else.
- **Non-verbose**: strip `file`, `symbol`, `source`, `pkg` attrs entirely.

### 5.3 JSON handler

Hand-rolled ordered JSON object per line: `{"time":…,"level":…,"message":…,`
then attrs in the same normalized order as pretty. No ANSI ever. Values
marshaled with `encoding/json`; `error` values via `.Error()`.

### 5.4 Structured metadata conventions (document in AGENTS.md)

| Key | Use |
|---|---|
| `step` | machine-readable phase id, kebab-case (`check-daemon`, `load-config`) |
| `action` | outcome verb (`created`, `verified`, `skipped`) |
| `path` | filesystem path involved |
| `err` | error value (always `err`, never `error`) |
| `hint` | one-line user remediation; pretty handler renders it on line 2 |
| `source` | explicit provenance override `symbol@file:line` |

### 5.5 Global flags (parsed in `main`, before dispatch)

| Flag | Env default | Values |
|---|---|---|
| `--log-format` | `{{ENV_PREFIX}}_LOG_FORMAT` | `pretty` (default) \| `json` |
| `--log-level` | `{{ENV_PREFIX}}_LOG_LEVEL` | `debug\|info\|warn\|error` (default `info`) |
| `--log-file` | `{{ENV_PREFIX}}_LOG_FILE` | path; appends, creates 0644 |
| `-v` / `--verbose` | — | provenance attrs on |
| `--version` | — | print version line, exit 0 |

---

## 6. Serialization, hashing, and time utilities

Serde packages live under `internal/platform/serde/*`; shorthand `serde/x`
below means `internal/platform/serde/x`.

### 6.1 `serde/jsonschema`

Wrap `santhosh-tekuri/jsonschema` (latest major) with draft 2020-12:

```go
type Problem struct {
    Pointer string // JSON Pointer into the instance, e.g. "/theme/palette/3"
    Message string
    Value   any
}
func Validate(schemaName string, schemaDoc, instance any) ([]Problem, error)
```

- Schema **compile** failure → returned `error` (internal bug, exit code 5).
- Instance **validation** failure → `[]Problem`, nil error. Callers print each
  problem as `pointer: message` and exit 3.

### 6.2 Two-phase strict validation (the pattern, applied everywhere)

Every YAML file the app reads (app config, theme) is validated in two layers:

1. Decode YAML → `map[string]any` (normalize `map[any]any` keys to strings,
   recursively), validate against the embedded JSON Schema. **All object levels
   in every schema set `"additionalProperties": false`.**
2. Re-decode into the typed Go struct using `yaml.Decoder` with
   `KnownFields(true)` so unknown YAML keys are rejected even if a schema is
   loosened later.
3. Optional third pass: semantic validation in Go (cross-field rules) returning
   `[]Problem` with pointers.

`serde/yamlstrict` provides the shared helpers: `NormalizeYAML`,
`DecodeStrict(data []byte, out any) error`.

### 6.3 `serde/canonjson` — stable JSON encoding

**All JSON the app emits** (files, `--json` command output, hashed payloads)
goes through this package — never bare `json.Marshal` for output. Encoding is
byte-stable for equal values: object keys lexicographically sorted at every
nesting level.

```go
func Marshal(v any) ([]byte, error)
    // marshal v → round-trip through map[string]any →
    // recursive canonical writer: sorted keys, compact separators
func MarshalIndent(v any, prefix, indent string) ([]byte, error)
```

Property test required: for any value, `Marshal(Unmarshal(Marshal(v)))` is
byte-identical, and key insertion order never affects output.

### 6.4 `serde/yamlcanon` — deterministic YAML emission

**All YAML the app writes** goes through this package. Reflection-based
encoder producing block-style YAML, 2-space indent, trailing newline:

```go
func Marshal(v any) ([]byte, error)
    // maps: keys sorted lexicographically
    // structs: fields sorted by their effective yaml tag name
    // scalars: explicit tags (!!str/!!int/!!bool/!!float/!!null);
    //          floats via strconv.FormatFloat(f, 'f', -1, 64)
func MarshalWithBanner(banner string, v any) ([]byte, error)
    // prepends a deterministic comment banner
```

Unsupported kinds (chan, func, complex, non-string map keys) return errors.
Property test required: equal values → byte-identical output regardless of
map insertion order.

### 6.5 `internal/platform/hashutil` — SHA-256 short hashing

**All content hashing in the app uses SHA-256, presented as the first 12 hex
characters (6 bytes) of the digest** — the "short hash". No other hash
algorithms or digest lengths appear anywhere (FNV in §5.2 is color-slot
selection, not content hashing).

```go
const PrefixHexLen = 12

func SumBytes(data []byte) string   // first 12 hex chars of SHA-256(data)
func SumString(s string) string
func SumFile(path string) (string, error)   // streaming; no full-file read
func EmptySHA256() string           // digest of empty payload

type Hasher struct{ /* wraps hash.Hash */ }
func NewHasher() *Hasher
func (h *Hasher) Write(p []byte) (int, error)
func (h *Hasher) WriteString(s string)
func (h *Hasher) Sum() string       // 12-hex-char short digest
```

When hashing structured data, callers MUST hash the canonical encoding
(`canonjson.Marshal` / `yamlcanon.Marshal`) so hashes are stable across
map ordering and struct field order.

### 6.6 `internal/platform/timeutil` — datetime handling

All time handling in the app goes through this package; no scattered
`time.Now()` calls outside it.

```go
type Clock interface {
    Now() time.Time
}
func Real() Clock                       // wall clock
func Fixed(t time.Time) Clock           // for tests
var Default Clock = Real()              // injectable package default

func NowUTC() time.Time                 // Default.Now().UTC()
func Stamp(t time.Time) string          // RFC3339, UTC, second precision
func StampMillis(t time.Time) string    // RFC3339 with milliseconds, UTC
func Parse(s string) (time.Time, error) // accepts RFC3339 / RFC3339Nano, returns UTC
func HumanDuration(d time.Duration) string // "1h12m", "3.2s", "415ms" — for UI/log text
```

Rules (also restate in AGENTS.md):

- Every persisted or displayed timestamp is **UTC RFC3339** via `Stamp`/
  `StampMillis`; never local time, never custom layouts.
- Code under test uses the injectable `Clock` (package-level `Default` or an
  explicit field), never raw `time.Now()`.
- Durations shown to users go through `HumanDuration`.

---

## 7. Embedded assets: `internal/platform/embed/assets`

### 7.1 Generator

`//go:generate go run ./cmd/gen -root ../../../.. -out assets_gen.go`

The `-root` path points from `internal/platform/embed/assets/` to the Go module
root. Config YAML and schemas are runtime files under repo-root `config/` and
are not embedded in this asset map.

The generator walks, **from the configured roots**:

- `static/files/**`

For each file: gzip (best compression) → emit as a `[]byte` literal in
`assets_gen.go` into `var generatedAssets = map[string][]byte{...}`. Keys strip
the `static/` prefix. The generated file carries a `// Code generated … DO NOT
EDIT.` header and is committed.

A CI gate re-runs `go generate ./...` and fails on `git diff --exit-code` so
the committed assets can never drift from the source tree.

### 7.2 Runtime FS

```go
var FS fs.FS                       // virtual fs over generatedAssets, transparent gunzip on Open
func ReadFile(name string) ([]byte, error)
func Sub(prefix string) (fs.FS, error)
func Keys() []string               // sorted; for tests
```

Virtual directories synthesized by prefix scan. `Open` of a missing key returns
`fs.ErrNotExist` wrapped with the name.

---

## 8. Shared config: `go-core/pkg/config`

`go-core/pkg/config` is the single Go package for repo-wide static config.
It is reused by the CLI, sitebuild, and infrabuild.

### 8.1 Files

- `config/data/global.yaml` validated by `config/schema/global.schema.json`
- `config/data/theme.yaml` validated by `config/schema/theme.schema.json`
- `config/data/infra.yaml` validated by `config/schema/infra.schema.json`

### 8.2 Discovery and load

```go
func Resolve(rootOrStart string) (*Paths, error)
func LoadGlobal(path string) (*Global, error)
func LoadTheme(path string) (*Theme, error)
func LoadInfra(path string) (*Infra, error)
func ValidatePath(path string) error
```

Loaders read schema files from `config/schema` at the resolved repo root,
decode YAML with strict fields, and run semantic validation.

---

## 9. CLI dispatch: `internal/cli`

### 9.1 Core types

```go
type Runner struct {
    In  io.Reader
    Out io.Writer
    Err io.Writer
    Tel *telemetry.Providers
}

type ConfiglessCommand interface {
    Name() string
    Usage() string
    Run(ctx context.Context, args []string) error
}
type ConfigCommand interface {
    Name() string
    Usage() string
    Run(ctx context.Context, cfg *config.Config, args []string) error
}

func Dispatch(ctx context.Context, r *Runner, args []string) error
```

- `commandRegistry(r *Runner) map[string]any` returns a literal map of name →
  command. Two small adapter structs lift plain handler funcs into the two
  interfaces.
- Dispatch: empty args → root usage to stderr, exit 2. `help|-h|--help` → root
  usage to stdout, exit 0. Unknown command → error + usage, exit 2.
  `ConfigCommand`s get `Discover` + `Load` run for them first.
- Nested groups via a shared router:
  `runSubcommandGroup(args, errW, group string, missingIsError bool, usageFn func(), handlers map[string]subcommandHandler) error`
  with shared `isHelpToken` handling.

### 9.2 Exit codes

Sentinel error type carrying an exit code, extracted in `main` via `errors.As`:

| Code | Meaning |
|---|---|
| 0 | success |
| 1 | generic failure |
| 2 | usage / flag error |
| 3 | config or input validation failure |
| 4 | filesystem / IO failure |
| 5 | internal invariant (schema compile, embed miss) |
| 6 | environment precondition failed (daemon unavailable, missing key) |

Exported helpers: `ExitCode(err error) int`, plus per-code constructors.

### 9.3 Help

**Root help** (`{{BINARY}}`, `{{BINARY}} help`):

1. Title line, themed: `{{BINARY}} - {{DESCRIPTION}} v{version} ({GOOS}/{GOARCH})`
2. Dim meta line: build date + commit.
3. Splash banner (§10).
4. Sections: `Usage`, `Commands` (name colored, aligned, one-line description),
   `Global flags`.

**Per-command help** via `color.RenderHelp`:

```go
type HelpFlag struct{ Name, Placeholder, Description string }
type HelpSpec struct {
    Command, Usage, Description string
    Flags    []HelpFlag
    Examples []string
}
```

Rendered with themed sections: command title (slot 0), Usage (dim), Flags
(flag name slot 4, placeholder slot 8, description plain), Examples (slot 2
header, dim commands). Every command and subcommand group must respond to
`help`, `-h`, `--help`.

### 9.4 Commands to implement

| Command | Kind | Behavior |
|---|---|---|
| `version` | configless | print `{{BINARY}} v{version} ({commit}, {date})` |
| `check` | configless | system checks, §12 |
| `config validate [path]` | configless | validate `config/data/{global,theme,infra}.yaml` or one explicit supported config path; exit 0/3 |
| `theme preview` | configless | render the resolved theme: 16 palette swatches (slot index, xterm index, colored block, role name), sample log lines at each level, a sample table, sample help block |
| `demo` | configless | example TUI (§11.4) proving the shared TUI stack end-to-end |

---

## 10. Splash banner

- `static/files/misc/banner.ans`: a **binary ANSI art file** containing raw
  pre-colored escape sequences (xterm-256 only). The agent must generate a
  tasteful placeholder banner programmatically (e.g. block-letter `{{PROJECT}}`
  with a palette gradient) and save it as the `.ans` file — the human will
  replace it with real art later. Width ≤ 80 columns.
- Additionally a small hardcoded ASCII block (project name + `{{LICENSE}}`)
  centered to 80 columns, printed above the `.ans` art.
- Shown **only** on root help (no-args invocation and `help|-h|--help`); never
  on normal command runs or `--version`.
- When color is disabled (§4.4), skip the `.ans` art entirely (it contains raw
  escapes) and print only the ASCII block.

---

## 11. TUI: `internal/platform/tui` (+ `tui/nav`)

### 11.1 Principle

Commands never construct bubbletea programs or huh forms directly. All
interactive flows go through this package so complex TUIs are written once and
reused. New interactive needs = new shared shortcut here, not inline code.

### 11.2 Shortcuts (`tui/shortcuts.go`)

```go
func ConfirmYesNo(ctx context.Context, title, desc string, defaultYes bool) (bool, error)
func PromptString(ctx context.Context, title, desc, placeholder string, validate func(string) error) (string, error)
func Select(ctx context.Context, title string, options []Option) (Option, error)
func MultiSelect(ctx context.Context, title string, options []Option, initial []string) ([]Option, error)
func ChoosePath(ctx context.Context, title, startDir string, onlyDirs bool) (string, error)
func OpenEditor(ctx context.Context, path string) error   // $VISUAL then $EDITOR
```

### 11.3 Cross-cutting requirements

- **Theme bridge**: a `Theme(dark bool) huh.Theme` (or equivalent lipgloss
  styles) mapping `color.Get()` palette slots → lipgloss colors, so the TUI
  matches terminal output.
- **Non-TTY**: every shortcut returns exported `ErrNotInteractive` wrapped with
  a `Hint() string`-bearing error type telling the user which flags to pass
  instead. Exported `ErrCancelled` for user abort (esc/ctrl-c); dispatcher maps
  it to exit 1 without a stack-trace-looking error.
- **Accessibility**: honor `ACCESSIBLE` env var → huh accessible mode.

### 11.4 `tui/nav` — reusable navigator

```go
type Item struct{ ID, Title, Description, Detail string }
func Run(items []Item, title string) (*Item, error)   // nil on quit
```

Split-pane bubbletea app: filterable list (left, bubbles/list) + detail
viewport (right), themed from the palette. Keys: ↑/↓ move, `/` filter, Enter
select, q/esc/ctrl-c quit. The `demo` command feeds it sample items and then
runs a `ConfirmYesNo` on the selection, demonstrating composition.

### 11.5 Testability

Shortcut entry points are injectable package vars (e.g. `var runForm = ...`)
so command tests can stub interactive flows without a PTY.

---

## 12. System checks: `check` command

`{{BINARY}} check` runs all checks, prints a themed table (name, status,
detail), and exits 0 if all pass, 6 if any fail. `--json` emits a JSON array
instead (via `serde/canonjson`, §6.3). Failures include a `hint` in logs and a
hint column in the table.

**Checks (presence/format only — no network calls):**

1. **Container engines** — for each of `{{CONTAINER_ENGINES}}` (or
   `checks.container_engines` from config): locate binary on `PATH`, then run
   `<engine> version` with a 3-second timeout (`exec.CommandContext`). Statuses:
   `ok` / `not installed` / `daemon unavailable`. Implement behind a small
   interface (`type Engine interface { Name() string; Available(ctx) error }`)
   with an injectable exec runner for tests.
2. **API keys** — for each env var in `{{API_KEYS}}` (or `checks.api_keys`):
   - present and non-empty after `strings.TrimSpace`
   - no interior whitespace or shell-quote characters
   - *(advisory)* known-prefix format checks where applicable
     (`sk-ant-` for `ANTHROPIC_API_KEY`, `sk-` for `OPENAI_API_KEY`); a wrong
     prefix is a **warning**, not a failure.
   Never print key material — only the var name and a masked length
   (`set, 51 chars`).
3. **Git** — `git` on `PATH` and CWD inside a work tree (warning if not).
4. **Theme** — resolved theme source (env/file/embedded) and schema validity.

---

## 13. `cli.sh`

Bash wrapper at the module root, committed executable (`#!/usr/bin/env bash`,
`set -euo pipefail`):

- Binary location: `dist/{{BINARY}}`.
- **Build-if-stale**: rebuild when the binary is missing, or any `*.go`,
  `go.mod`, `go.sum`, or `static/**` file is newer than the binary (use
  `find -newer`), or `{{BINARY}}_FORCE_BUILD=1`.
- Build command: `go build -trimpath -o dist/{{BINARY}} ./cmd/{{BINARY}}`,
  echoed to stderr when triggered.
- Then `exec dist/{{BINARY}} "$@"` — argv and exit code pass through untouched.
- Must work when invoked from any CWD (resolve script dir via
  `cd "$(dirname "${BASH_SOURCE[0]}")"` before building; run the binary with
  the **caller's** original CWD restored).

---

## 14. Quality gates

### 14.1 Makefile targets

| Target | Behavior |
|---|---|
| `make` / `make ci` | gate-go-version → gate-generate (assets drift check) → gate-fmt (gofmt + goimports diff) → gate-vet → gate-lint → gate-test (unit + cheap integration-realistic tests, `-race`) → gate-integration (`-tags=integration`: toolchain/binary-exec tests, §14.6) → gate-coverage → build |
| `make ci-rehearse` | re-run the full `ci` pipeline in a clean temp checkout with `CI=true`; local pass + CI fail is treated as a process bug |
| `make build` | native `dist/{{BINARY}}` |
| `make dev` | rebuild loop (`entr` if present, else sleep poll) + run |
| `make dist` | clean + ci + cross-compile linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64 + `SHA256SUMS` |
| `make clean` | remove `dist/`, coverage artifacts |

Release build flags: `CGO_ENABLED=0 -trimpath -ldflags "-s -w -buildid= -X {{MODULE}}/internal/platform/version.Version=$(VERSION) -X ...Commit=$(COMMIT) -X ...Date=$(DATE)" -buildvcs=false`, honoring `SOURCE_DATE_EPOCH` for reproducibility.

### 14.2 Lint: `.golangci.yml`

golangci-lint v2, enabled linters at minimum: `copyloopvar`, `depguard`,
`dupl`, `errorlint`, `exhaustive`, `funlen`, `gocritic`, `gocyclo` (min 10),
`gosec`, `lll` (120), `misspell`, `nakedret`, `nolintlint`, `prealloc`,
`revive` (error severity). Depguard rule: deny stdlib `log` everywhere except
`internal/platform/log`.

### 14.3 Coverage: `scripts/coveragegate`

Merge unit + integration cover profiles. Fail if **any** of:
- total coverage < `{{COVERAGE_MIN}}`
- any **package** < `{{COVERAGE_MIN}}`
- any **file** < `{{COVERAGE_MIN}}`

`COVERAGE_MIN ?= {{COVERAGE_MIN}}` in the Makefile; never lowered to make CI
pass.

### 14.4 Go version gate: `scripts/checkgoversion`

Compare `go env GOVERSION` minor against `go.mod`; fail with a clear message
on mismatch.

### 14.5 GitHub Actions CI

Generate a workflow at the **repository root**:
`{{REPO_ROOT_REL}}/.github/workflows/{{TARGET_DIR}}.yml`. It runs build +
tests on any change under the Go module directory.

Action versions below are the latest stable majors as of June 2026
(`actions/checkout` v6.0.3, `actions/setup-go` v6.4.0,
`golangci/golangci-lint-action` v9.2.1, `actions/upload-artifact` v7.0.1).
The agent MUST verify at generation time that these are still the latest
stable majors and bump if newer ones exist; reference actions by major tag.

```yaml
name: {{TARGET_DIR}}

on:
  push:
    branches: [main]
    paths:
      - '{{TARGET_DIR}}/**'
      - 'config/data/**'
      - 'config/schema/**'
      - '.github/workflows/{{TARGET_DIR}}.yml'
  pull_request:
    paths:
      - '{{TARGET_DIR}}/**'
      - 'config/data/**'
      - 'config/schema/**'
      - '.github/workflows/{{TARGET_DIR}}.yml'

concurrency:
  group: {{TARGET_DIR}}-${{ github.ref }}
  cancel-in-progress: true

permissions:
  contents: read

defaults:
  run:
    working-directory: {{TARGET_DIR}}

jobs:
  ci:
    runs-on: ubuntu-latest
    timeout-minutes: 20
    steps:
      - uses: actions/checkout@v6
      - uses: actions/setup-go@v6
        with:
          go-version-file: '{{TARGET_DIR}}/go.mod'
          cache-dependency-path: '{{TARGET_DIR}}/go.sum'
      - name: golangci-lint
        uses: golangci/golangci-lint-action@v9
        with:
          version: latest
          working-directory: {{TARGET_DIR}}
      - name: ci
        run: make ci
        env:
          CI: 'true'
      - name: coverage artifact
        if: always()
        uses: actions/upload-artifact@v7
        with:
          name: coverage
          path: {{TARGET_DIR}}/.coverage/
          if-no-files-found: ignore
```

Rules:

- **`make ci` is the single source of truth**; the workflow must not duplicate
  individual gates beyond the lint annotation step (which runs the same
  `.golangci.yml`). Any local-pass/CI-fail divergence is a process bug (§14.1
  `ci-rehearse` exists to catch it).
- The lint action installs the latest stable golangci-lint v2; `make ci` must
  tolerate a preinstalled binary (use it if compatible, else fetch its pinned
  version) so the two stay consistent.
- Coverage gate output is written under `.coverage/` so the artifact step can
  pick it up.

### 14.6 Testing strategy: integration-realism by default

**Design for testability first.** Every module is designed so its real
behavior can be exercised cheaply:

- I/O boundaries behind small interfaces or injectable package vars: exec
  runners (`check`), clocks (`timeutil.Clock`), TUI entry points (§11.5),
  editor/launcher hooks. Constructors take explicit `io.Reader`/`io.Writer`/
  paths/listeners — never reach for globals or `os.Stdout` directly.
- Prefer testing through the **real implementation** over mocks whenever the
  real thing is cheap: real files in `t.TempDir()`, real YAML/JSON fixtures on
  disk, real in-process servers. Mocks/fakes are reserved for boundaries that
  are expensive or nondeterministic (real container daemons, paid APIs, TTYs).

**Two tiers:**

| Tier | Gate | Contents |
|---|---|---|
| default (`gate-test`, `-race`) | every `make ci` run | unit tests **plus all cheap integration-realistic tests**: tempdir filesystems, in-process servers on OS-assigned ports, subprocess shims |
| `-tags=integration` (`gate-integration`) | every `make ci` run, separate profile | expensive tests only: anything invoking the Go toolchain (`go build`/`go run`), exec'ing the built CLI binary, shell-driven `cli.sh` tests |

Both profiles merge into the coverage gate (§14.3).

**Hermeticity norms (blanket, all tiers):**

- Network listeners always bind `127.0.0.1:0` and read back the assigned
  port; **never** fixed port numbers. **Zero external network access in any
  test tier.**
- No test touches the real `$HOME`, XDG dirs, or repo working tree: use
  `t.TempDir()` + `t.Setenv` (`HOME`, `XDG_CONFIG_HOME`, `PATH`, `NO_COLOR`)
  to build a hermetic world per test.
- Subprocesses get explicit env (`cmd.Env`), never inherited ambient state.

**Mandated integration-style tests** (minimum set; extend the same style to
every new module):

| Area | Tier | Test |
|---|---|---|
| `serde/canonjson`, `serde/yamlcanon` | default | write canonical output to real temp files, re-read, re-marshal → byte-identical; `hashutil.SumFile` of two semantically-equal-but-differently-ordered inputs → same short hash |
| `serde/jsonschema` + `yamlstrict` | default | validate real YAML fixture files (valid + invalid: unknown key, wrong type, bad enum) against the real embedded schemas; assert exact JSON Pointers in `Problem`s |
| `config` | default | full discovery + load against real temp project trees: nested CWD upward walk, valid config, schema violations with pointer assertions, `KnownFields` rejection, semantic-check failures |
| embed FS generator | integration | run the real `cmd/gen` against a temp fixture tree (files with known contents incl. binary data), emit `assets_gen.go` into a temp package, **compile it with `go build`** (compile-only, no exec), and assert the generator output round-trips byte-for-byte via a parallel decode; runtime FS unit tests run against the committed assets in the default tier |
| telemetry | default | start an **in-process OTLP/gRPC collector** (implementing the trace + metrics service protos) on `127.0.0.1:0`; `t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", addr)`; `telemetry.Init`; emit spans + counter/histogram values; assert the collector received them with expected names/attrs; also assert the disabled path (env unset → noop, instant shutdown, no connections) |
| `check` | default | fake `docker`/`podman` shim scripts written into a temp dir prepended to `PATH`: success, nonzero exit, and a sleep-beyond-timeout shim asserting the 3s `CommandContext` kill; API-key checks via `t.Setenv` |
| `env` | default | real `.env` files in temp dirs: parsing edge cases (quotes, escapes, `export`, comments), upward discovery stop at `.git`, and precedence (process env always wins) |
| theme resolution | default | real files exercising the §4.3 order: `{{ENV_PREFIX}}_THEME` path beats discovered `config/data/theme.yaml` beats embedded fallback; invalid theme file falls through with a logged warning |
| CLI end-to-end | integration | `TestMain` compiles the real binary **once** into a shared temp dir; tests exec it against temp project dirs asserting stdout/stderr content and exact exit codes (§9.2 table): help/banner, `version`, `check --json`, `config validate` happy + failure paths, `NO_COLOR` output purity |
| `cli.sh` | integration | shell-driven: first run builds, second run does not rebuild (assert binary mtime unchanged), touching a `.go` file triggers rebuild, args + exit codes pass through |

---

## 15. AGENTS.md (generate this file)

Generate an `AGENTS.md` at the module root containing, at minimum, these
norms (these also govern *this* bootstrap generation):

- **Modules**: stdlib-first; new deps need justification. One package = one
  concern; `internal/platform/*` is app-agnostic, `internal/cli` is wiring,
  domain logic lives in its own packages. No package cycles; `cli` may import
  anything, `platform/*` imports nothing app-specific.
- **Subcommands**: git-style via the registry; no cobra. New commands register
  in `commandRegistry`, implement one of the two command interfaces, provide a
  `HelpSpec`, and map failures to the exit-code table.
- **Errors**: `errors.Is`/`As`; exported sentinels for branchable conditions;
  user-facing errors carry `Hint() string` where remediation exists; thread
  `context.Context` through everything blocking.
- **Logging**: `internal/platform/log` only; structured keys per §5.4; never
  stdlib `log`, never bare `fmt.Println` for diagnostics (stdout is reserved
  for command output).
- **Serialization**: all emitted JSON via `serde/canonjson`, all emitted YAML
  via `serde/yamlcanon` (stable, sorted keys); bare `json.Marshal` /
  `yaml.Marshal` for output is forbidden.
- **Hashing**: all content hashing via `internal/platform/hashutil` — SHA-256,
  12-hex-char short hash; hash canonical encodings of structured data, never
  raw marshaled bytes.
- **Time**: all timestamps via `internal/platform/timeutil` — UTC RFC3339;
  no raw `time.Now()` outside the package; tests use the injectable `Clock`.
- **Dependencies**: stay on latest stable releases; upgrades via
  `go get <module>@latest && go mod tidy`, never hand-edited versions.
- **Color/tables/TUI**: only via `internal/platform/color` and
  `internal/platform/tui`; no ad-hoc ANSI, go-pretty, bubbletea, or huh usage
  in command code.
- **Tests**: table-driven; external test packages (`package foo_test`) by
  default; white-box tests in `*_internal_test.go`. **Design modules with
  testability at the forefront**: interfaces/injectable vars at every I/O
  boundary (exec, clock, TUI, listeners); constructors take explicit
  readers/writers/paths. Prefer exercising real behavior — real files in
  `t.TempDir()`, real fixture files, in-process servers on `127.0.0.1:0` —
  over mocks; mock only expensive/nondeterministic boundaries. Cheap
  integration-realistic tests run in the default gate; `-tags=integration` is
  reserved for toolchain-exec/binary-exec tests (§14.6). Never fixed ports,
  never external network, never the real `$HOME`/XDG/`PATH` — hermetic env
  via `t.Setenv`. Call `color.ResetForTest` when toggling color env vars.
  Every new module ships with integration-style tests in the §14.6 mold.
- **Quality**: never loosen or bypass gates (no disabling linters, no lowering
  `COVERAGE_MIN`, no skip flags). Fix root causes. Run `make ci` and
  `make ci-rehearse` before proposing a merge.
- **Commits**: user-facing impact in the title; bullet-list bodies emphasizing
  why/impact.
- **Embedded assets**: edit sources under `static/` (or `config/data/theme.yaml`),
  re-run `go generate ./...`, commit the regenerated `assets_gen.go`.

Also generate `CONTRIBUTING.md` (human-facing condensed version) and
`.cursor/rules/agents.mdc` + `CLAUDE.md`, each a one-liner pointing at
`AGENTS.md` as canonical.

---

## 16. `cmd/{{BINARY}}/main.go` contract

Thin `main`:

1. Parse global flags (§5.5) with stdlib `flag`, stopping at the first
   non-flag arg (subcommand).
2. Load `.env` then normalize env (§17).
3. `log.Configure(...)`; resolve theme (§4.3); `telemetry.Init(ctx)` with
   deferred shutdown.
4. Install signal handling: `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)`.
5. `cli.Dispatch(ctx, runner, rest)`; map returned error → exit code via
   `cli.ExitCode`; `os.Exit` from `main` only (a `run(args []string) int`
   inner function keeps `main` testable).

---

## 17. Env loading and normalization: `internal/platform/env`

- **`.env` autoload**: at startup, look for `.env` in CWD then walk upward to
  the repo root (stop at `.git`). Parse with a small **stdlib-only**
  implementation: `KEY=value` lines, `#` comments, blank lines, optional
  `export ` prefix, single/double quotes with standard escapes in
  double-quotes. Loaded values **never override** variables already present in
  the process environment.
- **Normalization** (`func Normalize()`): for every var matching
  `{{ENV_PREFIX}}_*`: trim whitespace from values; warn (structured log,
  debug level) on empty values; expose typed getters
  `String(name, def)`, `Bool(name, def)`, `Int(name, def)` that look up
  `{{ENV_PREFIX}}_<NAME>`.
- **Passthrough allowlist**: `env.passthrough` in config (§8.1) validated
  against POSIX name regex `^[A-Z_][A-Z0-9_]*$`; helper
  `Passthrough(names []string) map[string]string` copies present vars,
  trimming whitespace (this is the hook future subprocess/container features
  will use).
- Respected external standards: `NO_COLOR`, `CLICOLOR_FORCE`,
  `XDG_CONFIG_HOME`, `VISUAL`, `EDITOR`, `ACCESSIBLE`, `OTEL_*`.

---

## 18. Telemetry: `internal/platform/telemetry`

```go
type Providers struct {
    Tracer trace.Tracer
    Meter  metric.Meter
    // unexported: shutdown func, enabled bool
}
func Init(ctx context.Context) (*Providers, error)
func (p *Providers) StartSpan(ctx context.Context, name string) (context.Context, trace.Span)
func (p *Providers) Enabled() bool
func Global() *Providers
```

- **Enabled iff `OTEL_EXPORTER_OTLP_ENDPOINT` is non-empty**; otherwise noop
  tracer + meter providers, zero overhead, no warnings.
- OTLP/gRPC trace + metric exporters; resource attrs:
  `service.name={{OTEL_SERVICE}}`, `service.version` (from version pkg),
  `service.instance.id` (hostname). W3C tracecontext + baggage propagation.
- Metric naming convention: `{{BINARY}}.<area>.<thing>` — instrument at least:
  `{{BINARY}}.check.runs` (counter, attr `status`),
  `{{BINARY}}.check.duration` (histogram, seconds),
  `{{BINARY}}.cli.dispatches` (counter, attr `command`).
- Every command dispatch wrapped in a span `cli.<command>`.
- Provide a `telemetrytest` helper package with: (a) an in-memory collector
  for fast metric assertions, and (b) an **in-process OTLP/gRPC collector**
  implementing the trace + metrics service protos, listening on
  `127.0.0.1:0`, exposing received spans/metrics for assertions — used by the
  §14.6 telemetry integration test and reusable by future features.

---

## 19. Acceptance criteria

The agent's work is done when ALL of the following hold:

1. `make ci` passes from a clean checkout (fmt, vet, lint, race tests,
   integration tests, coverage ≥ {{COVERAGE_MIN}} at total/package/file
   granularity, generate-drift check, Go version gate).
2. `make ci-rehearse` passes.
3. `./cli.sh version` builds (first run) and prints the version; a second
   invocation does **not** rebuild; touching any `.go` file triggers rebuild.
4. `./cli.sh` (no args) prints the themed root help with splash banner, exit 2;
   `./cli.sh help` same output, exit 0; `NO_COLOR=1 ./cli.sh help | grep -c $'\x1b'`
   outputs 0.
5. `./cli.sh theme preview` renders 16 swatches, sample logs, a table, and a
   help block; honors a repo-root `config/data/theme.yaml` edit without rebuild
   (file resolution beats embedded copy).
6. `./cli.sh check` and `./cli.sh check --json` run all checks from §12 with
   correct exit codes (0 all-pass, 6 any-fail) and never print key material.
7. `./cli.sh config validate` against a deliberately broken YAML (unknown key,
   wrong type) prints JSON-Pointer-addressed problems and exits 3; valid file
   exits 0.
8. `./cli.sh demo` runs the nav TUI in a TTY; in a non-TTY pipe it exits with
   the `ErrNotInteractive` hint instead of hanging.
9. `--log-format=json` produces one valid JSON object per line with ordered
   `time`, `level`, `message` keys; `--log-format=pretty -v` shows `file=` and
   `symbol=` provenance.
10. `OTEL_EXPORTER_OTLP_ENDPOINT` unset → no telemetry connections attempted
    (verify: no goroutine leaks, instant shutdown).
11. `go generate ./...` is idempotent (`git diff --exit-code` clean).
12. `AGENTS.md`, `CONTRIBUTING.md`, `CLAUDE.md`, `.cursor/rules/agents.mdc`,
    `README.md` (brief: what the CLI is, how to build, command table) all
    exist with the content mandated in §15.
13. Canonical-encoding property tests pass: equal values yield byte-identical
    `canonjson.Marshal` / `yamlcanon.Marshal` output regardless of map
    insertion order; `check --json` output has lexicographically sorted keys.
14. Hashing golden test passes: `hashutil.SumString("")` equals the first 12
    hex chars of the empty SHA-256 digest (`e3b0c44298fc`); `SumFile` on a
    large temp file matches `sha256sum | cut -c1-12` and does not read the
    whole file into memory.
15. All timestamps in logs and emitted files are UTC RFC3339; grep finds no
    `time.Now()` outside `internal/platform/timeutil`.
16. At generation time, `go list -m -u all` reports no newer stable versions
    for direct dependencies (all deps added via `@latest`).
17. `.github/workflows/{{TARGET_DIR}}.yml` exists at the repo root per §14.5,
    is path-filtered to the module directory (plus `config/data/theme.yaml` and
    itself), runs `make ci`, validates with `actionlint` (or careful manual
    YAML review if actionlint is unavailable), and references only
    latest-stable action majors (verified at generation time).
18. Every test mandated in §14.6 exists in its designated tier and passes:
    serde round-trips on real temp files, schema validation against real
    fixture files with pointer assertions, config discovery/load on real temp
    trees, embed-generator generate+compile loop (`-tags=integration`),
    telemetry against the in-process OTLP collector on `127.0.0.1:0`, `check`
    against PATH shims (including the timeout shim), `.env` parsing on real
    files, theme resolution order, CLI end-to-end binary execution
    (`-tags=integration`), and `cli.sh` rebuild semantics
    (`-tags=integration`).
19. Hermeticity holds: the default-tier suite passes with external networking
    unavailable, with `HOME`/`XDG_CONFIG_HOME` pointed at empty temp dirs,
    and grep finds no hardcoded port numbers in test code (`:0` only).

---

## 20. Suggested build order (advisory)

1. `go.mod`, `VERSION`, `scripts/checkgoversion`, skeleton Makefile.
2. `platform/version`, `platform/color` (palette, enable logic, Sprint/FG/BG).
3. `platform/log` (pretty + JSON handlers) — test against `StripANSI`.
4. `platform/hashutil`, `platform/timeutil`, `platform/serde/canonjson`,
   `platform/serde/yamlcanon` (pure, no internal deps — easy early coverage).
5. `platform/serde/jsonschema` + `platform/serde/yamlstrict`.
6. `platform/embed/assets` generator + runtime FS; author generated placeholder
   `static/files/misc/banner.ans`.
7. Theme resolution wired into color + log.
8. `internal/platform/paths` + `internal/platform/configloader` (discover/load/validate).
9. `internal/cli` dispatch, exit codes, root help, `RenderHelp`; `version`.
10. `platform/env`, `platform/telemetry`.
11. `check`, `config validate`, `theme preview`.
12. `platform/tui` + `tui/nav`; `demo`.
13. `cli.sh`, full Makefile gates, `.golangci.yml`, `scripts/coveragegate`.
14. GitHub Actions workflow (§14.5) at the repo root.
15. `AGENTS.md` + docs; final `make ci` + `make ci-rehearse`.

---

## Appendix A. Clarification answers

These answers were supplied before implementation and are normative for this
bootstrap.

1. The CLI terminal theme is the `terminal` section of the shared repository-root
   `config/data/theme.yaml`; there is no module-local `go-core/config/` theme.
2. The default 16-slot terminal palette is derived from the existing brand
   light-mode autumn colors by choosing nearby xterm-256 indices.
3. The Go module path is `github.com/overplane/overplane`.
4. `otel_service` is `overplane`.
5. Frontmatter author is `Mayank Lahiri`.
6. Frontmatter created date is `2026-06-09`.
7. Coverage gates exclude generated files only, specifically `assets_gen.go`.
8. `ErrNotInteractive` maps to exit code 6.
9. The theme upward walk skips non-CLI theme files at debug level; only an
   explicit `OVERPLANE_THEME` path logs a warning/error when invalid.
10. Do not keep a separate CLI project config; the CLI validates the shared
    repo config files under `config/data/`.
11. `go-core/.cursor/rules/agents.mdc` is the module-scoped Cursor rule.
12. Initial `VERSION` is `0.0.1`.
13. Add an Apache-2.0 `LICENSE` file inside `go-core/`.
14. Use a single implementation commit after the acceptance gates pass.
15. After acceptance passes, set this spec frontmatter `status` to `closed`.

Implementation adjustment: config loading is split between `go-core/internal/platform/configloader`
and per-build generated `internal/config` packages.
All project-wide static YAML lives under `config/data/`, all JSON Schema lives
under `config/schema/`, and sitebuild/infrabuild own generated config types.
