---
# Advisory frontmatter (aperture spec-metadata schema, extended).
description: Bootstrap a production-quality Go CLI scaffold with git-style subcommands,
  themed 256-color terminal output, structured logging, JSON-schema-validated YAML
  config, embedded assets, TUI infrastructure, OTEL metrics, env loading, system
  checks, and strict quality gates.
id: '#0001'
parent: []
status: active  # implemented; schema has no terminal state (see Appendix A item 15)
tags:
  - bootstrap
  - cli
  - go
title: Go CLI bootstrap
# --- advisory extensions (not part of the base schema) ---
author: Mayank Lahiri
created: '2026-06-09'
spec_version: 1
---

# Spec #0001 — Go CLI bootstrap

This is a **self-contained build prompt** for an AI coding agent. Executing it
should produce a complete, compiling, tested, lint-clean Go CLI scaffold in the
target directory. The architecture is modeled on a proven production CLI
(git-style subcommands, stdlib-first, themed terminal output, strict gates).
Every requirement below is normative unless marked *advisory*.

> **Status: implemented.** This document has been updated post-implementation
> to describe the **as-built** state of **overplane** (this module). Deliberate deviations from
> the original prompt are folded into the section bodies and summarized in
> Appendix B. The original clarification answers remain in Appendix A.

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
```

Where a token appears inside a path, identifier, or code snippet (e.g.
`cmd/{{BINARY}}/main.go`, `{{ENV_PREFIX}}_LOG_LEVEL`), the agent substitutes
the literal value (e.g. `cmd/opl/main.go`, `OPL_LOG_LEVEL`). Tokens are
always UPPERCASE; GitHub Actions expressions like `${{ github.ref }}` (§14.5)
are not spec tokens and must be left verbatim.

---

## 1. Goal and non-goals

**Goal.** Generate the directory containing this spec's `.overplane/` folder
(the **overplane** module root) as a single Go module that builds
one static CLI binary `{{BINARY}}`, with all the platform subsystems specified
below, ready for feature commands to be added on top.

**Non-goals.** No business logic. No network services. No cobra/viper/urfave.
No web UI. The deliverable is a *scaffold of production quality*, not a demo.

---

## 2. Hard constraints

1. **Go {{GO_VERSION}}** (latest stable). `go.mod` declares `go {{GO_VERSION}}.0`
   and an explicit `toolchain` line for the newest patch release. A CI gate
   script fails the build if the local toolchain minor version differs.
2. **Stdlib-first.** Allowed third-party dependencies, exhaustively (as-built
   module paths in parentheses):
   - `github.com/santhosh-tekuri/jsonschema/v6` (JSON Schema draft 2020-12)
   - `gopkg.in/yaml.v3` (YAML with `KnownFields`)
   - `github.com/jedib0t/go-pretty/v6` (tables only)
   - `github.com/charmbracelet/{bubbletea,huh,lipgloss}` (TUI only).
     `bubbles` ended up an **indirect** dependency only: the `tui/nav`
     navigator is hand-rolled bubbletea (§11.4), not `bubbles/list`.
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
./                                    # overplane module root ({{MODULE}})
├── cmd/{{BINARY}}/main.go            # thin entrypoint: flags, logging, telemetry, dispatch
├── internal/
│   ├── cli/                          # command registry, dispatch, per-command handlers
│   │   ├── cli.go                    # Dispatch, Runner, registry, root help wiring
│   │   ├── errors.go                 # ExitError, ExitCode, per-code constructors
│   │   ├── subcommands.go            # nested subcommand router + usage printers
│   │   ├── version.go                # `version` command
│   │   ├── check.go                  # system checks command
│   │   ├── configcmd.go              # `config validate` command
│   │   ├── themecmd.go               # `theme preview` command
│   │   └── democmd.go                # example TUI command (`demo`)
│   ├── platform/
│   │   ├── clihelp/                  # root-help renderer (banner/header/usage/commands/flags)
│   │   ├── color/                    # palette, ANSI, tables, per-command help, theme resolution
│   │   ├── log/                      # slog: pretty + JSON handlers
│   │   ├── tui/                      # shared interactive shortcuts (huh wrappers)
│   │   │   └── nav/                  # hand-rolled list+detail bubbletea navigator
│   │   ├── telemetry/                # OTEL providers (noop unless endpoint set)
│   │   │   └── telemetrytest/        # in-process OTLP/gRPC collector for tests (§18)
│   │   ├── env/                      # .env loader + normalization
│   │   ├── paths/                    # repo-root discovery → absolute Paths set (§8)
│   │   ├── configloader/             # generic schema-validated YAML loader (§8)
│   │   ├── hashutil/                 # SHA-256 short-hash utilities (§6.5)
│   │   ├── timeutil/                 # datetime utilities, injectable clock (§6.6)
│   │   ├── version/                  # Version/Commit/Date vars set via -ldflags
│   │   ├── serde/
│   │   │   ├── jsonschema/           # schema compile + validate → []Problem
│   │   │   ├── yamlstrict/           # strict YAML decode helpers
│   │   │   ├── canonjson/            # stable JSON encoding, sorted keys (§6.3)
│   │   │   └── yamlcanon/            # deterministic YAML emission (§6.4)
│   │   └── embed/assets/             # generated gzip asset map + virtual fs.FS
│   │       └── cmd/gen/main.go       # go:generate asset compiler
├── static/                           # embed source tree (input to the generator)
│   └── files/
│       └── misc/banner.ans           # ANSI splash banner (binary, pre-colored truecolor art)
├── .github/workflows/                # CI for the mirrored public repo (post-bootstrap)
│   ├── ci.yml                        # make ci on push/PR in the public repo
│   ├── release.yml                   # goreleaser on v* tags
│   └── npm-publish.yml               # npm package publish
├── .goreleaser.yaml                  # release builds for the public repo (post-bootstrap)
├── npm/                              # npm distribution wrapper (post-bootstrap)
├── cli.sh                            # build-if-stale wrapper (§13)
├── Makefile                          # quality gates (§14)
├── .golangci.yml                     # strict lint config (§14)
├── scripts/
│   ├── checkgoversion                # Go toolchain gate
│   └── coveragegate                  # coverage report (§14.3)
├── AGENTS.md                         # agent instructions (§15)
├── CONTRIBUTING.md                   # human conventions (condensed from §15)
├── CLAUDE.md / .cursor/rules/agents.mdc  # one-liners pointing at AGENTS.md
├── LICENSE                           # Apache-2.0
├── go.mod / go.sum
└── VERSION                           # semver, single line
```

**Note on config:** the module keeps no module-local `config/` directory and
there is **no `pkg/config` package**: an *enclosing repository* (when overplane
is vendored inside a larger tree) may provide shared static YAML under
`config/data/` and JSON Schema under `config/schema/`; the CLI discovers such a
root by upward walk (`internal/platform/paths`) and loads files through the
generic schema-validated loader (`internal/platform/configloader`, §8). Nothing
in overplane depends on any sibling directory beyond this optional discovery.

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

Default palette (used when no theme resolves) — the built-in
`overplane-autumn` set, `{214, 130, 166, 64, 67, 136, 95, 244, 180, 94, 101,
172, 173, 58, 230, 238}`:

| Slot | xterm | Role |
|---|---|---|
| 0 | 214 | titles, provenance (file/symbol) |
| 1 | 130 | step identifiers |
| 2 | 166 | warnings, examples |
| 3 | 64 | success |
| 4 | 67 | info level, flag names |
| 5 | 136 | accents |
| 6 | 95 | debug |
| 7 | 244 | dim/secondary text |
| 8 | 180 | placeholders |
| 9–13 | 94, 101, 172, 173, 58 | rotating slots for hashed key coloring (§5.2) |
| 14 | 230 | near-white |
| 15 | 238 | dark |

(All 16 slots participate in FNV-1a hashed key coloring; 9–13 have no other
dedicated role.)

### 4.3 Theme: `config/data/theme.yaml`

An enclosing repository may provide a theme file at `config/data/theme.yaml`
(discovered by upward walk, §8.2). The file is a **shared project theme** and
may carry additional sections for other consumers; the CLI is concerned only
with its `terminal:` section. The file is schema-validated against the
adjacent `config/schema/theme.schema.json` (strict;
`additionalProperties: false` at every level). The `terminal:` shape:

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

**Resolution order** (first hit wins), implemented by `color.ResolveTheme` /
`color.ApplyResolvedTheme` and executed once at process start:

1. `{{ENV_PREFIX}}_THEME` env var → explicit path to a full theme YAML (error
   if invalid).
2. Walk upward from CWD looking for `config/data/theme.yaml` (repo-root
   discovery, same upward-walk used elsewhere, §8.2).
3. Built-in `overplane-autumn` palette fallback if no repository config can be
   discovered.

A discovered theme that fails schema validation falls back to the built-in
terminal palette (and `main` logs a warning). An explicit `{{ENV_PREFIX}}_THEME`
error fails resolution. After resolution, `color.Set` is called with the
palette. **As-built note:** the `terminal.log` slot overrides are parsed and
schema-validated but are **not yet wired into the log package** — log level
slots are currently fixed (§5.2).

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
- **Level**: uppercase, colored via fixed palette slots — as-built mapping:
  error→slot 2, warn→slot 0, info→slot 4, debug→slot 7. (Theme `log.*` slot
  overrides are schema-accepted but not yet applied, §4.3.)
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
   recursively), validate against the JSON Schema. **All object levels in
   every schema set `"additionalProperties": false`.**
2. Decode into the typed Go struct. As-built, `configloader.Load[T]` does this
   by round-tripping the schema-validated instance through `encoding/json`
   (schema strictness already rejects unknown keys); `yamlstrict.DecodeStrict`
   (`yaml.Decoder` with `KnownFields(true)`) remains available for direct
   strict decoding.
3. Optional third pass: semantic validation in Go (cross-field rules), e.g.
   `color.Theme.Validate`.

`serde/yamlstrict` provides the shared helpers: `NormalizeYAML`,
`DecodeStrict(data []byte, out any) error`, `DecodeMap(data []byte)`.

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
root. Shared config YAML and schemas discovered in an enclosing repository
(§8) are runtime files and are not embedded in this asset map.

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

## 8. Shared config: `internal/platform/paths` + `internal/platform/configloader`

There is no `pkg/config`. Shared static config is handled by two app-agnostic
platform packages.

### 8.1 Files

When an enclosing repository provides them, the CLI can discover and validate:

- `config/data/global.yaml` validated by `config/schema/global.schema.json`
- `config/data/theme.yaml` validated by `config/schema/theme.schema.json`
- `config/data/infra.yaml` validated by `config/schema/infra.schema.json`

### 8.2 Discovery: `internal/platform/paths`

```go
type Paths struct { Root, ConfigDataDir, ConfigSchemaDir string; GlobalFile, ThemeFile, InfraFile string; GlobalSchema, ThemeSchema, InfraSchema string; /* + derived sibling-tree dirs */ }
func Resolve(rootOrStart string) (*Paths, error)
```

`Resolve("")` walks upward from CWD; the discovered root is the first
directory containing **both** `config/data/global.yaml` and
`config/schema/global.schema.json`. The returned `Paths` also carries derived
locations for sibling trees of the enclosing repository; overplane itself only
reads the config files and schemas listed above.

### 8.3 Loading: `internal/platform/configloader`

```go
type ValidationError struct{ Problems []jsonschema.Problem }
func Load[T any](yamlText, schemaText, schemaName string) (*T, error)
func LoadBytes[T any](yamlData, schemaData []byte, schemaName string) (*T, error)
func Validate(yamlText, schemaText, schemaName string) ([]jsonschema.Problem, error)
func ValidateBytes(yamlData, schemaData []byte, schemaName string) ([]jsonschema.Problem, error)
```

Generic, schema-driven: normalize YAML, validate against the schema document,
then decode the validated instance into `T` via a JSON round-trip (§6.2).
There are no hand-written `LoadGlobal`/`LoadTheme`/`LoadInfra` loaders in
overplane; the CLI only validates the shared files (`config validate`, §9.4).

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

func Dispatch(ctx context.Context, r *Runner, args []string) error
```

- As-built there is **only `ConfiglessCommand`** — no `ConfigCommand`
  interface exists yet because no bootstrap command needs a loaded config
  (commands that need paths call `paths.Resolve` themselves). Add a
  config-bearing interface when the first such command appears.
- `commandRegistry(r *Runner) map[string]commandInfo` returns a literal map of
  name → `{name, one-line description, command value}`; descriptions feed the
  root help.
- Dispatch: empty args → root usage to **stdout**, exit 2 (`main` suppresses
  the duplicate error line in this case). `help|-h|--help` → root usage to
  stdout, exit 0. Unknown command → error + usage to stderr, exit 2. Every
  dispatch is wrapped in a `cli.<command>` span when telemetry is attached.
- Nested groups via a shared router:
  `runSubcommandGroup(ctx, args, errW, group string, missingIsError bool, usageFn func(), handlers map[string]subcommandHandler) error`
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

**Root help** (`{{BINARY}}`, `{{BINARY}} help`) is rendered by the dedicated
`internal/platform/clihelp` package (`RenderRoot(w, RootSpec)`), not inline in
`internal/cli`:

1. Title line, themed: `{{BINARY}} - {{DESCRIPTION}} v{version} ({GOOS}/{GOARCH})`
2. Dim meta line: build date (calendar date only, no time) + commit + a
   friendly note with the binary's on-disk size ("I am a X.X MB
   executable.").
3. Splash banner (§10) — below the meta line, above Usage.
4. Sections: `Usage`, `Commands` (name colored, aligned, sorted, one-line
   description), `Global flags`. Column padding is owned by `clihelp`; never
   hand-roll root-help spacing in command code.

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
| `check [--json]` | configless | system checks, §12 |
| `config validate [path]` | configless | validate the discovered `config/data/{global,theme,infra}.yaml` files (§8.1) or one explicit supported config path (schema chosen by basename); prints `<path> valid` per file; exit 0/3 |
| `theme preview` | configless | render the resolved theme: header `Theme: <name> (<source>)`, 16 palette swatches (slot index, xterm index, colored block, role name), sample log lines at each level, a sample table, sample help block |
| `demo` | configless | example TUI (§11.4) proving the shared TUI stack end-to-end; prints `selected=<id>` on selection |

---

## 10. Splash banner

- `static/files/misc/banner.ans`: a **binary ANSI art file** containing raw
  pre-colored escape sequences. As-built this is the real committed artwork
  and it uses **24-bit truecolor** sequences — the §4.1 xterm-256-only rule
  applies to *program-generated* output, not to this pre-rendered asset.
- The banner is embedded via the asset map and printed **verbatim whenever the
  asset is present — including under `NO_COLOR`** (deliberate decision; the
  art is meaningless without its escapes, and tests assert its presence). The
  §19 `NO_COLOR` output-purity criterion therefore exempts the banner.
- A plain one-line fallback (`{{PROJECT}} / {{LICENSE}}`) is printed only when
  the embedded asset is missing; there is no separate hardcoded ASCII block
  above the art.
- Shown **only** on root help (no-args invocation and `help|-h|--help`); never
  on normal command runs or `--version`.

---

## 11. TUI: `internal/platform/tui` (+ `tui/nav`)

### 11.1 Principle

Commands never construct bubbletea programs or huh forms directly. All
interactive flows go through this package so complex TUIs are written once and
reused. New interactive needs = new shared shortcut here, not inline code.

### 11.2 Shortcuts (`tui/shortcuts.go`)

```go
type Option struct{ Value, Label, Description string }
type HintError struct{ Err error; HintText string }      // Error/Unwrap/Hint

func ConfirmYesNo(ctx context.Context, title, desc string, defaultYes bool) (bool, error)
func PromptString(ctx context.Context, title, desc, placeholder string, validate func(string) error) (string, error)
func Select(ctx context.Context, title string, options []Option) (Option, error)
func MultiSelect(ctx context.Context, title string, options []Option, initial []string) ([]Option, error)
func ChoosePath(ctx context.Context, title, startDir string, onlyDirs bool) (string, error)
func OpenEditor(ctx context.Context, path string) error   // $VISUAL then $EDITOR
func EnsureInteractive(hint string) error                  // TTY precheck for commands
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

As-built: a **hand-rolled bubbletea model** (no `bubbles/list`) rendering a
boxed item table plus a detail pane below it, themed from the palette, with a
footer line showing a live clock and load average (refreshed by a tick). Keys:
↑/↓ or j/k move, Home/End jump, Enter select, q/esc/ctrl-c quit (no filter
key). The `demo` command feeds it sample items, guards non-TTY via
`tui.EnsureInteractive` (mapped to exit 6), and prints `selected=<id>` for the
chosen item.

### 11.5 Testability

As-built, shortcuts gate on a TTY check (`requireTTY`) before constructing any
huh form, so non-PTY tests exercise the `ErrNotInteractive`/hint path and
command tests assert exit code 6 for `demo` in a pipe. There are no injectable
`var runForm`-style hooks yet; add them when a command needs its interactive
happy path covered without a PTY.

---

## 12. System checks: `check` command

`{{BINARY}} check` runs all checks, prints a themed table (Name, Status,
Detail, Hint), and exits 0 if all pass, 6 if any fail. Statuses `not
installed`, `daemon unavailable`, `missing`, and `invalid` fail the run;
`warning` does not. `--json` emits a canonical JSON array instead
(`canonjson.MarshalIndent`, §6.3) of `{name, status, detail, hint}` objects.

**Checks (presence/format only — no network calls).** The engine/API-key lists
are compiled-in defaults (`docker`/`podman`; `ANTHROPIC_API_KEY`,
`OPENAI_API_KEY`, `GEMINI_API_KEY`); there is no `checks.*` config section
yet.

1. **Container engines** (`engine:<name>`) — locate binary on `PATH`, then run
   `<engine> version` with a 3-second timeout (`exec.CommandContext`).
   Statuses: `ok` / `not installed` / `daemon unavailable`. Implemented behind
   `type Engine interface { Name() string; Version(ctx) (string, error) }`
   with an injectable package-level exec runner (`runExec`) for tests; the
   reported engine version string lands in the hint column.
2. **API keys** (`api-key:<VAR>`) —
   - present and non-empty after `strings.TrimSpace` → else `missing`
   - no interior whitespace or shell-quote characters → else `invalid`
   - known-prefix format checks (`sk-ant-` for `ANTHROPIC_API_KEY`, `sk-` for
     `OPENAI_API_KEY`); a wrong prefix is a **warning**, not a failure.
   Detail shows only a masked length (`set, 51 chars`). **Decision:** the hint
   column additionally shows the first 5 characters as `prefix=sk-an` to make
   misconfigured keys diagnosable; full key material is never printed.
3. **Git** — `git` on `PATH` and CWD inside a work tree; both conditions are
   warnings only, and the success hint shows `branch=… commit=…`.

There is **no theme check** (the original item 4); theme problems surface via
`theme preview` and the startup warning instead.

---

## 13. `cli.sh`

Bash wrapper at the module root, committed executable (`#!/usr/bin/env bash`,
`set -euo pipefail`):

- Binary location: `dist/{{BINARY}}`.
- **Build-if-stale**: rebuild when the binary is missing, or
  `{{ENV_PREFIX}}_FORCE_BUILD=1`, or any of the following is newer than the
  binary (`find -newer`): `*.go`, `go.mod`, `go.sum`, `VERSION`,
  `static/**`, or any `*.yaml`/`*.json` under an enclosing repository's
  `config/data/` and `config/schema/` when present (so shared config/schema
  edits invalidate the build too).
- Build command mirrors `make build`: `CGO_ENABLED=0 go build -trimpath
  -ldflags "<-s -w -buildid= -X version.{Version,Commit,Date}=…>"
  -buildvcs=false -o dist/{{BINARY}} ./cmd/{{BINARY}}`, stamping version from
  `VERSION`, commit from git, and date honoring `SOURCE_DATE_EPOCH` — wrapper
  builds carry full build metadata, identical to Makefile builds.
- Then `exec dist/{{BINARY}} "$@"` — argv and exit code pass through untouched.
- Must work when invoked from any CWD (resolve script dir via
  `BASH_SOURCE` before building; run the binary with the **caller's** original
  CWD restored).

---

## 14. Quality gates

### 14.1 Makefile targets

| Target | Behavior |
|---|---|
| `make` / `make ci` | gate-go-version → gate-generate (assets drift check) → gate-fmt (gofmt + goimports rewrite, then `git diff --exit-code`) → gate-vet → gate-lint → gate-test (unit + cheap integration-realistic tests, `-race`) → gate-integration (`-tags=integration`: toolchain/binary-exec tests, §14.6) → gate-coverage → build |
| `make ci-rehearse` | `git clone --local` the repo into a temp dir and re-run the full `ci` pipeline there with `CI=true`; local pass + CI fail is treated as a process bug |
| `make build` | native `dist/{{BINARY}}` |
| `make dev` | rebuild loop (`entr` if present, else sleep poll) + run |
| `make dist` | clean + ci + cross-compile linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64 + `SHA256SUMS` |
| `make clean` | remove `dist/`, `.coverage/`, `.tmp/` |

The Makefile pins `GOTOOLCHAIN` to the exact toolchain patch version and
installs helper tools (goimports, golangci-lint) into a module-local
`.tmp/bin` prepended to `PATH`.

Release build flags: `CGO_ENABLED=0 -trimpath -ldflags "-s -w -buildid= -X {{MODULE}}/internal/platform/version.Version=$(VERSION) -X ...Commit=$(COMMIT) -X ...Date=$(DATE)" -buildvcs=false`, honoring `SOURCE_DATE_EPOCH` for reproducibility.

### 14.2 Lint: `.golangci.yml`

golangci-lint v2, enabled linters: `copyloopvar`, `depguard`, `dupl`,
`errorlint`, `exhaustive`, `funlen`, `gocritic`, `gocyclo` (min 10), `gosec`,
`lll` (120), `misspell`, `nakedret`, `nolintlint`, `prealloc`, `revive`;
`errcheck` is disabled. Depguard rule (lax list-mode): deny stdlib `log`
repo-wide. As-built exclusions: revive `package-comments`/`exported`, a fixed
set of gosec rules (G204/301/302/304/306/702/703), gocyclo in tests and for a
named list of long dispatch/encoder functions, and funlen for `run`/`write`.
`make ci` always installs the latest golangci-lint v2 into `.tmp/bin` (it does
not reuse a preinstalled binary).

### 14.3 Coverage: `scripts/coveragegate`

Merge unit + integration cover profiles into `.coverage/coverage.out` and run
`scripts/coveragegate` over the result, excluding generated `assets_gen.go`.

**As-built decision:** the gate is **report-only** — it prints
`coverage report complete: total N% (floor M%)` and fails only if the profile
is missing; it does not currently enforce the total/package/file floors from
the original prompt. `COVERAGE_MIN ?= {{COVERAGE_MIN}}` remains the declared
floor in the Makefile; restoring hard enforcement is future work, and the
floor must never be lowered to make CI pass once enforced.

### 14.4 Go version gate: `scripts/checkgoversion`

Compare `go env GOVERSION` minor against `go.mod`; fail with a clear message
on mismatch.

### 14.5 GitHub Actions CI

`make ci` is the single source of truth; workflows add no extra gates (no
separate lint action, no coverage artifact). The module owns its workflows
under `.github/workflows/` at the module root, shipping with the public
`github.com/overplane/overplane` repository:

- `ci.yml` — `make ci` on push/PR to main, with `permissions: contents: read`
  and per-ref concurrency.
- `release.yml` — GoReleaser (`.goreleaser.yaml`) on `v*` tags: cross-compiled,
  upx-packed bare executables.
- `npm-publish.yml` — publishes the `npm/` wrapper package.

When overplane is vendored inside a larger repository, that repository may run
its own minimal workflow invoking `make ci` in this module; such workflows are
owned by the enclosing repository and are outside this spec.

Rule: any local-pass/CI-fail divergence is a process bug (§14.1 `ci-rehearse`
exists to catch it).

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

**Integration-style tests as-built** (extend the same style to every new
module):

| Area | Tier | Test |
|---|---|---|
| `serde/canonjson`, `serde/yamlcanon` | default | write canonical output to real temp files, re-read, re-marshal → byte-identical; `hashutil.SumFile` of two semantically-equal-but-differently-ordered inputs → same short hash |
| `serde/jsonschema` + `yamlstrict` | default | validate real YAML fixture files (valid + invalid: unknown key, wrong type, bad enum) against real schemas; assert exact JSON Pointers in `Problem`s |
| `paths` + `configloader` | default | discovery + load against real temp project trees: nested CWD upward walk, valid config, schema violations with pointer assertions |
| embed FS generator | default | run the real `cmd/gen` against a temp fixture tree (incl. binary data) and assert the emitted `assets_gen.go` round-trips byte-for-byte via gzip decode (no `go build` compile step — the committed generated file is compiled by every build anyway); runtime FS tests run against the committed assets |
| telemetry | default | the `telemetrytest` **in-process OTLP/gRPC collector** on `127.0.0.1:0`; `t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", addr)`; `telemetry.Init`; emit spans; assert receipt; also assert the disabled path (env unset → noop, instant shutdown, no connections) |
| `check` | default | fake engine shim scripts written into a temp dir prepended to `PATH` (`t.Setenv`); API-key checks via `t.Setenv`; the exec runner is also injectable (`runExec`) |
| `env` | default | real `.env` files in temp dirs: parsing edge cases (quotes, `export`, comments), upward discovery stop at `.git`, and precedence (process env always wins) |
| theme resolution | default | real files exercising the §4.3 order: `{{ENV_PREFIX}}_THEME` path beats discovered `config/data/theme.yaml` beats built-in fallback; invalid theme file falls through |
| CLI end-to-end | integration | `cmd/overplane/integration_test.go` builds the real binary with `go build` into a temp dir and execs it (`--version`), asserting output and exit codes |
| `cli.sh` | integration | shell-driven: first run builds, second run does not rebuild (binary mtime unchanged), wrapper help carries real build metadata (version/commit/date stamped) |

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

The as-built `AGENTS.md` additionally mandates `internal/platform/clihelp` for
all root-help rendering (banner placement, column padding).

Also generated: `CONTRIBUTING.md` (human-facing condensed version) and
`.cursor/rules/agents.mdc` + `CLAUDE.md` at the module root, each a one-liner
pointing at `AGENTS.md` as canonical.

---

## 16. `cmd/{{BINARY}}/main.go` contract

Thin `main`, as-built order:

1. Load `.env` then normalize env (§17) — first, so env values can serve as
   flag defaults.
2. Parse global flags (§5.5) with stdlib `flag`, stopping at the first
   non-flag arg (subcommand). `--version` short-circuits here.
3. Open `--log-file` if set (append, 0644); `log.Configure(...)` writing to
   stderr or the file; `color.ApplyResolvedTheme("")` (§4.3), logging a
   warning with a hint on failure.
4. Install signal handling: `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)`;
   `telemetry.Init(ctx)` with deferred `Shutdown`.
5. `cli.Dispatch(ctx, runner, rest)`; map returned error → exit code via
   `cli.ExitCode` (suppressing the redundant error line for the bare no-args
   usage case); `os.Exit` from `main` only (a
   `run(args []string, stdout, stderr io.Writer) int` inner function keeps
   `main` testable).

---

## 17. Env loading and normalization: `internal/platform/env`

- **`.env` autoload** (`func Load(ctx, startDir) error`): look for `.env` in
  CWD then walk upward (stop once a `.git` directory is seen without a
  `.env`). Parse with a small **stdlib-only** implementation: `KEY=value`
  lines, `#` comments (full-line and trailing ` #`), blank lines, optional
  `export ` prefix, single quotes verbatim, double quotes via
  `strconv.Unquote`. Loaded values **never override** variables already
  present in the process environment.
- **Normalization** (`func Normalize()`): trim whitespace from the value of
  every var matching `{{ENV_PREFIX}}_*`. Typed getters `String(name, def)`,
  `Bool(name, def)`, `Int(name, def)` look up `{{ENV_PREFIX}}_<NAME>`.
  (No debug-level warning on empty values as-built.)
- **Passthrough**: helper `Passthrough(names []string) map[string]string`
  copies present vars, trimming whitespace — the hook future
  subprocess/container features will use. There is **no `env.passthrough`
  config section yet**; the allowlist becomes config-driven when the first
  consumer lands.
- Respected external standards: `NO_COLOR`, `CLICOLOR_FORCE`, `VISUAL`,
  `EDITOR`, `ACCESSIBLE`, `OTEL_*`.

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
func (p *Providers) Shutdown(ctx context.Context) error
func Global() *Providers
```

- **Enabled iff `OTEL_EXPORTER_OTLP_ENDPOINT` is non-empty** (an
  `http(s)://` scheme prefix is stripped; the gRPC connection is insecure);
  otherwise noop tracer + meter providers, zero overhead, no warnings.
- OTLP/gRPC trace + metric exporters; resource attrs:
  `service.name={{OTEL_SERVICE}}`, `service.version` (from version pkg),
  `service.instance.id` (hostname). W3C tracecontext + baggage propagation.
- Every command dispatch wrapped in a span `cli.<command>`.
- **As-built scope:** no metric instruments are registered yet. The
  `{{BINARY}}.<area>.<thing>` naming convention
  (`{{BINARY}}.check.runs` counter, `{{BINARY}}.check.duration` histogram,
  `{{BINARY}}.cli.dispatches` counter) remains the plan for the first feature
  that needs metrics; the meter provider plumbing is in place.
- `telemetrytest` provides an **in-process OTLP/gRPC collector** implementing
  the trace + metrics service protos, listening on `127.0.0.1:0`, exposing
  received spans/metrics for assertions — used by the §14.6 telemetry test and
  reusable by future features.

---

## 19. Acceptance criteria (as accepted)

The bootstrap was accepted against the following, adjusted from the original
prompt where decisions in Appendix B changed the contract:

1. `make ci` passes from a clean checkout (fmt, vet, lint, race tests,
   integration tests, coverage report (§14.3), generate-drift check, Go
   version gate).
2. `make ci-rehearse` passes.
3. `./cli.sh version` builds (first run) and prints the version; a second
   invocation does **not** rebuild; touching any watched input (`*.go`,
   `VERSION`, discovered config/schema YAML/JSON) triggers rebuild.
4. `./cli.sh` (no args) prints the themed root help with splash banner, exit 2;
   `./cli.sh help` same output, exit 0. With `NO_COLOR=1`, all
   *program-generated* output is escape-free; the pre-rendered banner is
   exempt (§10).
5. `./cli.sh theme preview` renders 16 swatches, sample logs, a table, and a
   help block; honors a discovered `config/data/theme.yaml` edit without
   rebuild (file resolution beats the built-in default).
6. `./cli.sh check` and `./cli.sh check --json` run all checks from §12 with
   correct exit codes (0 all-pass/warnings-only, 6 any-fail) and never print
   full key material (masked length + 5-char prefix only).
7. `./cli.sh config validate` against a deliberately broken YAML (unknown key,
   wrong type) prints JSON-Pointer-addressed problems and exits 3; valid files
   exit 0.
8. `./cli.sh demo` runs the nav TUI in a TTY; in a non-TTY pipe it exits 6
   with the `ErrNotInteractive` hint instead of hanging.
9. `--log-format=json` produces one valid JSON object per line with ordered
   `time`, `level`, `message` keys; `--log-format=pretty -v` shows `file=` and
   `symbol=` provenance.
10. `OTEL_EXPORTER_OTLP_ENDPOINT` unset → no telemetry connections attempted
    (no goroutine leaks, instant shutdown).
11. `go generate ./...` is idempotent (`git diff --exit-code` clean).
12. `AGENTS.md`, `CONTRIBUTING.md`, `CLAUDE.md`, `.cursor/rules/agents.mdc`,
    `README.md` (brief: what the CLI is, how to build, command table) all
    exist with the content mandated in §15.
13. Canonical-encoding property tests pass: equal values yield byte-identical
    `canonjson.Marshal` / `yamlcanon.Marshal` output regardless of map
    insertion order; `check --json` output has lexicographically sorted keys.
14. Hashing golden test passes: `hashutil.SumString("")` equals the first 12
    hex chars of the empty SHA-256 digest (`e3b0c44298fc`); `SumFile` streams
    (no full-file read).
15. All timestamps in logs and emitted files are UTC RFC3339; no `time.Now()`
    outside `internal/platform/timeutil`.
16. All direct dependencies were added via `go get @latest` at generation
    time.
17. `.github/workflows/ci.yml` exists at the module root per §14.5 and runs
    `make ci`.
18. Every test in the §14.6 table exists in its designated tier and passes.
19. Hermeticity holds for the default tier: no external network access, no
    fixed ports (`127.0.0.1:0` only), hermetic env via `t.Setenv`.

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
14. GitHub Actions workflows (§14.5) under the module's `.github/workflows/`.
15. `AGENTS.md` + docs; final `make ci` + `make ci-rehearse`.

---

## Appendix A. Clarification answers

These answers were supplied before implementation and are normative for this
bootstrap.

1. The CLI terminal theme is the `terminal` section of a discovered shared
   `config/data/theme.yaml`; there is no module-local `config/` theme.
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
    config files discovered under `config/data/`.
11. `.cursor/rules/agents.mdc` is the module-scoped Cursor rule.
12. Initial `VERSION` is `0.0.1`.
13. Add an Apache-2.0 `LICENSE` file at the module root.
14. Use a single implementation commit after the acceptance gates pass.
15. After acceptance passes, set this spec frontmatter `status` to `closed`.
    **Superseded:** the spec-metadata schema's `status` enum is
    `draft | active | deprecated | superseded` — there is no `closed`. The
    spec remains `active` (implemented, not deprecated) until a later spec
    supersedes it.

---

## Appendix B. As-built record (deviations and decisions)

Summary of where the implementation deliberately diverged from the original
prompt. Details are folded into the section bodies above.

1. **Config architecture (§3, §8).** No `pkg/config`. Enclosing-repository
   discovery lives in `internal/platform/paths`; generic schema-validated YAML
   loading in `internal/platform/configloader` (`Load[T]`/`Validate`, JSON
   round-trip decode).
2. **Root help renderer (§9.3).** Extracted into `internal/platform/clihelp`
   (`RenderRoot`/`RootSpec`); banner sits below the build-date line, above
   Usage.
3. **Command interfaces (§9.1).** Only `ConfiglessCommand` exists; no command
   needed a loaded config, so `ConfigCommand` was not built.
4. **Banner (§10).** Real committed truecolor ANSI art, printed verbatim even
   under `NO_COLOR`; plain one-line fallback only when the asset is missing.
   No separate ASCII block.
5. **Log level slots (§5.2, §4.3).** Fixed mapping error→2, warn→0, info→4,
   debug→7; theme `terminal.log` overrides are schema-validated but not yet
   applied.
6. **`check` (§12).** Engine interface is `Version(ctx) (string, error)`;
   statuses include `missing`/`invalid`/`warning`; hint column shows a 5-char
   key prefix for diagnosability; no theme check; engine/key lists are
   compiled-in (no `checks.*` config).
7. **TUI nav (§11.4).** Hand-rolled bubbletea table+detail navigator (no
   `bubbles/list`, no filter key); footer with clock and load average. `demo`
   prints the selection instead of chaining `ConfirmYesNo`.
8. **Coverage gate (§14.3).** Report-only as-built; floors not yet enforced.
9. **Telemetry metrics (§18).** Span-per-dispatch only; no metric instruments
   registered yet.
10. **CI (§14.5).** Module-owned workflows (`ci.yml`, `release.yml` +
    GoReleaser/upx, `npm-publish.yml`) live under the module's `.github/` and
    ship with the public `github.com/overplane/overplane` repository
    (post-bootstrap additions, together with `npm/` packaging).
11. **`cli.sh` (§13).** Staleness inputs extended to `VERSION` and a
    discovered enclosing repository's `config/data/` + `config/schema/`;
    wrapper builds stamp the same ldflags metadata as `make build`.
12. **Env (§17).** No empty-value debug warning; `Passthrough` helper exists
    but the `env.passthrough` config allowlist is deferred until a consumer
    exists.
13. **Tests (§14.6).** Embed-generator test runs in the default tier as an
    in-process round-trip (no `go build` compile loop); the integration tier
    contains the binary-exec and `cli.sh` tests.
14. **Versioning.** `VERSION` advanced past the initial `0.0.1` (release
    cycles started).
