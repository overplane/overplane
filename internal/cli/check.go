package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/overplane/overplane/internal/platform/color"
	"github.com/overplane/overplane/internal/platform/serde/canonjson"
	"github.com/overplane/overplane/internal/project"
	"github.com/overplane/overplane/internal/recipes"
)

type checkCommand struct{ r *Runner }

type CheckResult struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
	Hint   string `json:"hint,omitempty"`
}

type Engine interface {
	Name() string
	Version(context.Context) (string, error)
}

type ExecRunner func(context.Context, string, ...string) ([]byte, error)

const checkHintWidth = 58

var runExec ExecRunner = func(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil && stderr.Len() > 0 {
		return out, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return out, err
}

type commandEngine struct{ name string }

func (e commandEngine) Name() string { return e.name }

func (e commandEngine) Version(ctx context.Context) (string, error) {
	if _, err := exec.LookPath(e.name); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	out, err := runExec(ctx, e.name, "version")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (c checkCommand) Name() string  { return "check" }
func (c checkCommand) Usage() string { return Binary + " check [--json]" }

func (c checkCommand) Run(ctx context.Context, args []string) error {
	if len(args) > 0 && isHelpToken(args[0]) {
		fmt.Fprint(c.r.Out, usage(color.HelpSpec{
			Command: "check",
			Usage:   Binary + " check [--json] [--quiet]",
			Description: "Run local environment checks without network calls. " +
				"Passes when at least one container engine works; API keys are optional.",
			Flags: []color.HelpFlag{
				{Name: "--json", Description: "emit canonical JSON"},
				{Name: "--quiet", Description: "produce no output; result is the exit code only"},
			},
		}))
		return nil
	}
	jsonOut, rest := wantsJSON(args)
	quiet, rest := wantsFlag(rest, "--quiet")
	if len(rest) > 0 {
		return UsageError("unknown check flag %q", rest[0])
	}
	results := runChecks(ctx, defaultChecks())
	switch {
	case quiet:
		// No output of any kind; callers read the exit code.
	case jsonOut:
		b, err := canonjson.MarshalIndent(results, "", "  ")
		if err != nil {
			return InternalError(err)
		}
		fmt.Fprintln(c.r.Out, string(b))
	default:
		renderCheckResults(c.r.Out, results)
	}
	if failures := checkFailures(results); len(failures) > 0 {
		return EnvError(errors.New("checks failed: " + strings.Join(failures, "; ")))
	}
	return nil
}

func renderCheckResults(w io.Writer, results []CheckResult) {
	t := color.Table(w)
	t.SetColumnConfigs([]table.ColumnConfig{{Number: 4, WidthMax: checkHintWidth}})
	t.AppendHeader(table.Row{"Name", "Status", "Detail", "Hint"})
	for _, r := range results {
		status := r.Status
		switch r.Status {
		case "ok":
			status = color.Sprint(3, status)
		case "warning":
			status = color.Sprint(2, status)
		default:
			status = color.Sprint(0, status)
		}
		t.AppendRow(table.Row{r.Name, status, r.Detail, r.Hint})
	}
	t.Render()
}

// checkFailures applies the aggregation rules of spec #0002 §6.2: at least
// one container engine must be ok, and a set-but-malformed API key fails;
// missing keys and git problems are warnings only.
func checkFailures(results []CheckResult) []string {
	var failures []string
	if !anyEngineOK(results) {
		failures = append(failures, "no working container engine (need docker or podman)")
	}
	for _, r := range results {
		if strings.HasPrefix(r.Name, "api-key:") && r.Status == "invalid" {
			failures = append(failures, r.Name+" is set but malformed")
		}
	}
	return failures
}

func anyEngineOK(results []CheckResult) bool {
	for _, r := range results {
		if strings.HasPrefix(r.Name, "engine:") && r.Status == "ok" {
			return true
		}
	}
	return false
}

type checksConfig struct {
	ContainerEngines []string
	APIKeys          []string
}

func defaultChecks() checksConfig {
	return checksConfig{
		ContainerEngines: []string{"docker", "podman"},
		APIKeys:          []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY"},
	}
}

// checksConfigForProject derives the engine and API-key probes for setup from
// a validated overplane.yaml and the embedded recipe registry.
func checksConfigForProject(cfg *project.Config, reg *recipes.Registry) checksConfig {
	if cfg == nil || cfg.Agent == nil {
		return defaultChecks()
	}
	ac := cfg.Agent.Container
	keys := reg.EnvPassthrough(ac)
	if len(keys) == 0 {
		keys = defaultChecks().APIKeys
	}
	return checksConfig{
		ContainerEngines: []string{ac.Runtime},
		APIKeys:          keys,
	}
}

// runProjectChecks runs the setup-time system checks for cfg without printing
// a table. The configured container runtime must be available; malformed API
// keys still fail.
func runProjectChecks(ctx context.Context, cfg *project.Config, reg *recipes.Registry) error {
	checkCfg := checksConfigForProject(cfg, reg)
	results := runChecks(ctx, checkCfg)
	if failures := setupCheckFailures(results, checkCfg.ContainerEngines[0]); len(failures) > 0 {
		return EnvError(errors.New("checks failed: " + strings.Join(failures, "; ")))
	}
	return nil
}

// setupCheckFailures requires the project's configured engine to be ok and
// applies the same malformed API-key rule as checkFailures.
func setupCheckFailures(results []CheckResult, runtime string) []string {
	var failures []string
	required := "engine:" + runtime
	found := false
	for _, r := range results {
		if r.Name == required {
			found = true
			if r.Status != "ok" {
				failures = append(failures, runtime+" engine not available ("+r.Status+": "+r.Detail+")")
			}
		}
		if strings.HasPrefix(r.Name, "api-key:") && r.Status == "invalid" {
			failures = append(failures, r.Name+" is set but malformed")
		}
	}
	if !found {
		failures = append(failures, "container engine check missing for "+runtime)
	}
	return failures
}

func runChecks(ctx context.Context, cfg checksConfig) []CheckResult {
	out := make([]CheckResult, 0, len(cfg.ContainerEngines)+len(cfg.APIKeys)+1)
	for _, name := range cfg.ContainerEngines {
		out = append(out, checkEngine(ctx, commandEngine{name: name}))
	}
	for _, name := range cfg.APIKeys {
		out = append(out, checkAPIKey(name))
	}
	out = append(out, checkGit(ctx))
	return out
}

func checkEngine(ctx context.Context, e Engine) CheckResult {
	if _, err := exec.LookPath(e.Name()); err != nil {
		return CheckResult{
			Name: "engine:" + e.Name(), Status: "not installed",
			Detail: "binary not on PATH", Hint: "install " + e.Name(),
		}
	}
	version, err := e.Version(ctx)
	if err != nil {
		return CheckResult{
			Name: "engine:" + e.Name(), Status: "daemon unavailable",
			Detail: err.Error(), Hint: e.Name() + " version must succeed",
		}
	}
	return CheckResult{
		Name: "engine:" + e.Name(), Status: "ok",
		Detail: "available", Hint: truncateHint(version, checkHintWidth),
	}
}

func checkAPIKey(name string) CheckResult {
	raw, ok := os.LookupEnv(name)
	v := strings.TrimSpace(raw)
	if !ok || v == "" {
		return CheckResult{Name: "api-key:" + name, Status: "warning", Detail: "not set", Hint: "export " + name}
	}
	if strings.ContainsAny(v, " \t\n\r'\"`") {
		return CheckResult{
			Name: "api-key:" + name, Status: "invalid",
			Detail: "set, " + fmt.Sprint(len(v)) + " chars",
			Hint:   keyPrefix(v) + "; remove whitespace and shell quotes",
		}
	}
	if name == "ANTHROPIC_API_KEY" && !strings.HasPrefix(v, "sk-ant-") {
		return CheckResult{
			Name: "api-key:" + name, Status: "warning",
			Detail: masked(v), Hint: keyPrefix(v) + "; expected sk-ant- prefix",
		}
	}
	if name == "OPENAI_API_KEY" && !strings.HasPrefix(v, "sk-") {
		return CheckResult{
			Name: "api-key:" + name, Status: "warning",
			Detail: masked(v), Hint: keyPrefix(v) + "; expected sk- prefix",
		}
	}
	return CheckResult{Name: "api-key:" + name, Status: "ok", Detail: masked(v), Hint: keyPrefix(v)}
}

func masked(v string) string {
	return fmt.Sprintf("set, %d chars", len(v))
}

func keyPrefix(v string) string {
	if len(v) > 5 {
		v = v[:5]
	}
	return "prefix=" + v
}

func truncateHint(s string, limit int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= limit {
		return s
	}
	if limit <= 3 {
		return s[:limit]
	}
	return s[:limit-3] + "..."
}

func checkGit(ctx context.Context) CheckResult {
	if _, err := exec.LookPath("git"); err != nil {
		return CheckResult{Name: "git", Status: "warning", Detail: "git not on PATH", Hint: "install git"}
	}
	if _, err := runExec(ctx, "git", "rev-parse", "--is-inside-work-tree"); err != nil {
		return CheckResult{Name: "git", Status: "warning", Detail: "not inside a work tree", Hint: "run from repo root"}
	}
	return CheckResult{Name: "git", Status: "ok", Detail: "inside a work tree", Hint: gitHint(ctx)}
}

func gitHint(ctx context.Context) string {
	branch := "unknown"
	if out, err := runExec(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		branch = strings.TrimSpace(string(out))
	}
	commit := "unknown"
	if out, err := runExec(ctx, "git", "rev-parse", "--short", "HEAD"); err == nil {
		commit = strings.TrimSpace(string(out))
	}
	return truncateHint("branch="+branch+" commit="+commit, checkHintWidth)
}
