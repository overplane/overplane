package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	assetfs "github.com/overplane/overplane/internal/platform/embed/assets"
)

// agentProjectYAML is a valid #0003-era overplane.yaml for a project named
// "demo" with one configured agent.
const agentProjectYAML = `dirs: {cache: .cache/overplane, specs: .overplane/specs}
project: {name: demo}
schema_version: 1
agent:
  container:
    agent_recipes:
      - name: codex
        version: latest
    base_image: debian:bookworm-slim
    env_passthrough: [EDITOR]
    extra_packages: {}
    runtime: docker
    setup_recipe: debian
`

const agentPodmanProjectYAML = `dirs: {cache: .cache/overplane, specs: .overplane/specs}
project: {name: demo}
schema_version: 1
agent:
  container:
    agent_recipes:
      - name: codex
        version: latest
    base_image: debian:bookworm-slim
    env_passthrough: [EDITOR]
    runtime: podman
    setup_recipe: debian
    extra_packages: {}
`

// setupAgentFixture creates a project dir with overplane.yaml, wires an
// in-process engine shim into agent commands, installs minimal engine
// binaries for setup/check preflight, chdirs into the project, and returns
// the fixture.
func setupAgentFixture(t *testing.T, projectYAML string) *agentTestFixture {
	t.Helper()
	t.Setenv("NO_COLOR", "1")
	dir := t.TempDir()
	if projectYAML != "" {
		writeTestFile(t, filepath.Join(dir, "overplane.yaml"), projectYAML)
	}
	runtime := projectRuntime(projectYAML)
	bin := filepath.Join(dir, "bin")
	if runtime != "" {
		writeCheckEngineShim(t, bin, runtime)
	}
	t.Setenv("PATH", isolatedToolPATH(bin))
	shim := newAgentEngineShim(runtime)
	installAgentEngineHook(t, shim)
	oldwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(oldwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return &agentTestFixture{shim: shim, bin: bin}
}

func projectRuntime(projectYAML string) string {
	if projectYAML == "" {
		return "docker"
	}
	if strings.Contains(projectYAML, "runtime: podman") {
		return "podman"
	}
	return "docker"
}

func shimCalls(t *testing.T, fx *agentTestFixture) string {
	t.Helper()
	return fx.calls()
}

// primeShimImage simulates a previously built image in the shim state.
func primeShimImage(t *testing.T, fx *agentTestFixture, tags, labels []string) {
	t.Helper()
	fx.primeImage(tags, labels)
}

func dispatchAgent(r *Runner, args ...string) error {
	return Dispatch(context.Background(), r, append([]string{"agent"}, args...))
}

func TestAgentHelpSurfaces(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var out, errb bytes.Buffer
	r := &Runner{In: strings.NewReader(""), Out: &out, Err: &errb}
	if code := ExitCode(dispatchAgent(r)); code != 2 {
		t.Fatalf("bare agent exit = %d", code)
	}
	for _, args := range [][]string{
		{"help"}, {"setup", "--help"}, {"shell", "-h"}, {"list-images", "help"},
	} {
		out.Reset()
		if err := dispatchAgent(r, args...); err != nil {
			t.Fatalf("agent %v: %v", args, err)
		}
		if !strings.Contains(out.String(), "agent") {
			t.Fatalf("agent %v help output: %q", args, out.String())
		}
	}
	if code := ExitCode(dispatchAgent(r, "bogus")); code != 2 {
		t.Fatalf("unknown subcommand exit = %d", code)
	}
	if code := ExitCode(dispatchAgent(r, "setup", "stray")); code != 2 {
		t.Fatalf("stray arg exit = %d", code)
	}
}

func TestAgentRequiresProject(t *testing.T) {
	setupAgentFixture(t, "") // no overplane.yaml
	var out, errb bytes.Buffer
	r := &Runner{In: strings.NewReader(""), Out: &out, Err: &errb}
	for _, sub := range []string{"setup", "shell", "list-images"} {
		err := dispatchAgent(r, sub)
		if code := ExitCode(err); code != 6 {
			t.Fatalf("agent %s without project exit = %d (%v)", sub, code, err)
		}
		if !strings.Contains(err.Error(), "overplane init") {
			t.Fatalf("agent %s error should hint init: %v", sub, err)
		}
	}
}

func TestAgentValidationFailureExit3(t *testing.T) {
	bad := strings.Replace(agentProjectYAML, "runtime: docker", "runtime: lxc", 1)
	setupAgentFixture(t, bad)
	var out, errb bytes.Buffer
	r := &Runner{In: strings.NewReader(""), Out: &out, Err: &errb}
	err := dispatchAgent(r, "setup")
	if code := ExitCode(err); code != 3 {
		t.Fatalf("invalid config exit = %d (%v)", code, err)
	}
	if !strings.Contains(errb.String(), "/agent/container/runtime") {
		t.Fatalf("stderr should carry the JSON pointer: %s", errb.String())
	}
}

func TestAgentSetupBuildThenCacheHit(t *testing.T) {
	fx := setupAgentFixture(t, agentProjectYAML)
	var out, errb bytes.Buffer
	r := &Runner{In: strings.NewReader(""), Out: &out, Err: &errb}

	if err := dispatchAgent(r, "setup"); err != nil {
		t.Fatalf("first setup: %v stderr=%s", err, errb.String())
	}
	calls := shimCalls(t, fx)
	if !strings.Contains(calls, "buildx build") {
		t.Fatalf("first setup should build:\n%s", calls)
	}
	for _, want := range []string{
		"overplane-demo:latest", "Build hash", "Recipe", "codex@latest", "overplane shell",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("summary missing %q:\n%s", want, out.String())
		}
	}
	firstBuilds := strings.Count(calls, "buildx build")

	out.Reset()
	if err := dispatchAgent(r, "setup"); err != nil {
		t.Fatalf("second setup: %v stderr=%s", err, errb.String())
	}
	calls = shimCalls(t, fx)
	if got := strings.Count(calls, "buildx build"); got != firstBuilds {
		t.Fatalf("second setup must be a cache hit; builds %d -> %d\n%s", firstBuilds, got, calls)
	}

	// --force rebuilds with --no-cache despite the cached image.
	out.Reset()
	if err := dispatchAgent(r, "setup", "--force"); err != nil {
		t.Fatalf("forced setup: %v stderr=%s", err, errb.String())
	}
	calls = shimCalls(t, fx)
	if got := strings.Count(calls, "buildx build"); got != firstBuilds+1 {
		t.Fatalf("--force should rebuild; builds = %d", got)
	}
	if !strings.Contains(calls, "--no-cache") {
		t.Fatalf("--force should pass --no-cache:\n%s", calls)
	}
}

func TestAgentSetupEngineUnavailable(t *testing.T) {
	fx := setupAgentFixture(t, agentProjectYAML)
	fx.shim.versionFail = true
	var out, errb bytes.Buffer
	r := &Runner{In: strings.NewReader(""), Out: &out, Err: &errb}
	err := dispatchAgent(r, "setup")
	if code := ExitCode(err); code != 6 {
		t.Fatalf("engine unavailable exit = %d (%v)", code, err)
	}
	if !strings.Contains(err.Error(), "docker info") {
		t.Fatalf("error should carry the engine hint: %v", err)
	}
}

func agentShellTestImageLabels() []string {
	return []string{
		"overplane.project=demo",
		"overplane.build.hash=cafebabe",
		"overplane.build.recipe=debian",
		"overplane.build.base=debian:bookworm-slim",
		"overplane.version=test",
	}
}

func TestAgentShellRunsContainer(t *testing.T) {
	fx := setupAgentFixture(t, agentProjectYAML)
	primeShimImage(t, fx,
		[]string{"overplane-demo:latest", "overplane-demo:bhash-cafebabe"},
		agentShellTestImageLabels())
	t.Setenv("OPENAI_API_KEY", "sk-test-shell")
	t.Setenv("EDITOR", "vim")
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("LANG", "C.UTF-8")
	restore := agentEnsureInteractive
	agentEnsureInteractive = func(string) error { return nil }
	defer func() { agentEnsureInteractive = restore }()

	var out, errb bytes.Buffer
	r := &Runner{In: strings.NewReader(""), Out: &out, Err: &errb}
	if err := dispatchAgent(r, "shell"); err != nil {
		t.Fatalf("agent shell: %v stderr=%s", err, errb.String())
	}
	if !strings.Contains(out.String(), "shell-ran env_OPENAI=sk-test-shell") {
		t.Fatalf("shell output should show passthrough env:\n%s", out.String())
	}
	calls := shimCalls(t, fx)
	runLine := ""
	for _, line := range strings.Split(calls, "\n") {
		if strings.HasPrefix(line, "run ") {
			runLine = line
		}
	}
	if runLine == "" {
		t.Fatalf("no run invocation:\n%s", calls)
	}
	for _, want := range []string{
		"--rm", "-i", "-t", "--network host", "--env EDITOR", "--env OPENAI_API_KEY",
		"--env TERM", "--label overplane.project=demo", "--label overplane.shell=true",
		"overplane-demo:latest bash -l",
	} {
		if !strings.Contains(runLine, want) {
			t.Fatalf("run argv missing %q:\n%s", want, runLine)
		}
	}
	if strings.Contains(runLine, " -v ") || strings.Contains(runLine, "sk-test-shell") {
		t.Fatalf("run argv must not contain mounts or secret values:\n%s", runLine)
	}
}

func TestAgentShellExitCodePassthrough(t *testing.T) {
	fx := setupAgentFixture(t, agentProjectYAML)
	primeShimImage(t, fx,
		[]string{"overplane-demo:latest"}, agentShellTestImageLabels())
	fx.shim.runExit = 7
	restore := agentEnsureInteractive
	agentEnsureInteractive = func(string) error { return nil }
	defer func() { agentEnsureInteractive = restore }()
	var out, errb bytes.Buffer
	r := &Runner{In: strings.NewReader(""), Out: &out, Err: &errb}
	if code := ExitCode(dispatchAgent(r, "shell")); code != 7 {
		t.Fatalf("shell exit passthrough = %d", code)
	}
}

func TestAgentShellMissingImage(t *testing.T) {
	setupAgentFixture(t, agentProjectYAML)
	restore := agentEnsureInteractive
	agentEnsureInteractive = func(string) error { return nil }
	defer func() { agentEnsureInteractive = restore }()
	var out, errb bytes.Buffer
	r := &Runner{In: strings.NewReader(""), Out: &out, Err: &errb}
	err := dispatchAgent(r, "shell")
	if code := ExitCode(err); code != 6 {
		t.Fatalf("missing image exit = %d (%v)", code, err)
	}
	if !strings.Contains(err.Error(), "setup") {
		t.Fatalf("missing image error should hint setup: %v", err)
	}
}

func TestAgentShellNonInteractive(t *testing.T) {
	setupAgentFixture(t, agentProjectYAML)
	var out, errb bytes.Buffer
	r := &Runner{In: strings.NewReader(""), Out: &out, Err: &errb}
	// Test processes have no TTY, so the real guard rejects.
	if code := ExitCode(dispatchAgent(r, "shell")); code != 6 {
		t.Fatalf("non-tty shell exit = %d", code)
	}
}

func TestAgentListImages(t *testing.T) {
	fx := setupAgentFixture(t, agentProjectYAML)
	var out, errb bytes.Buffer
	r := &Runner{In: strings.NewReader(""), Out: &out, Err: &errb}

	// Zero images is success with a hint.
	if err := dispatchAgent(r, "list-images"); err != nil {
		t.Fatalf("empty list-images: %v", err)
	}
	if !strings.Contains(out.String(), "agent setup") {
		t.Fatalf("empty listing should hint setup:\n%s", out.String())
	}

	primeShimImage(t, fx,
		[]string{"overplane-demo:bhash-cafebabe", "overplane-demo:latest"},
		agentShellTestImageLabels())
	out.Reset()
	if err := dispatchAgent(r, "list-images"); err != nil {
		t.Fatalf("list-images: %v stderr=%s", err, errb.String())
	}
	text := out.String()
	for _, want := range []string{
		"overplane-demo:latest", "overplane-demo:bhash-cafebabe", "0123456789ab",
		"1.23GB", "2026-01-02T03:04:05Z", "debian", "cafebabe",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("table missing %q:\n%s", want, text)
		}
	}
	latestIdx := strings.Index(text, "overplane-demo:latest")
	hashIdx := strings.Index(text, "overplane-demo:bhash-cafebabe")
	if latestIdx < 0 || hashIdx < latestIdx {
		t.Fatalf(":latest row must come first:\n%s", text)
	}

	out.Reset()
	if err := dispatchAgent(r, "list-images", "--json"); err != nil {
		t.Fatalf("list-images --json: %v", err)
	}
	var records []map[string]any
	if err := json.Unmarshal(out.Bytes(), &records); err != nil {
		t.Fatalf("bad json: %v\n%s", err, out.String())
	}
	if len(records) != 2 || records[0]["tag"] != "overplane-demo:latest" {
		t.Fatalf("json records = %+v", records)
	}
	if records[0]["build_hash"] != "cafebabe" || records[0]["size_bytes"] != float64(1234567890) {
		t.Fatalf("json record fields = %+v", records[0])
	}
}

// TestConfigValidateRecipes covers the §7 sibling change: `config validate`
// understands the recipes.yaml basename and the discovered-set mode picks up
// config/data/recipes.yaml when present.
func TestConfigValidateRecipes(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	dir := t.TempDir()
	writeCLIConfigFixture(t, dir)
	registry, err := assetfs.ReadFile("files/recipes/recipes.yaml")
	if err != nil {
		t.Fatal(err)
	}
	recipesPath := filepath.Join(dir, "config", "data", "recipes.yaml")
	writeTestFile(t, recipesPath, string(registry))
	oldwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(oldwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	r := &Runner{In: strings.NewReader(""), Out: &out, Err: &errb}
	if err := Dispatch(context.Background(), r, []string{"config", "validate"}); err != nil {
		t.Fatalf("discovered-set validate: %v stderr=%s", err, errb.String())
	}
	if !strings.Contains(out.String(), "recipes.yaml valid") {
		t.Fatalf("discovered set should include recipes.yaml:\n%s", out.String())
	}
	out.Reset()
	if err := Dispatch(context.Background(), r,
		[]string{"config", "validate", recipesPath}); err != nil {
		t.Fatalf("explicit validate: %v stderr=%s", err, errb.String())
	}

	// A broken registry (pair referencing an unknown runtime) fails with
	// exit 3.
	broken := strings.Replace(string(registry), "- {build: docker, run: docker}",
		"- {build: docker, run: lxc}", 1)
	writeTestFile(t, recipesPath, broken)
	err = Dispatch(context.Background(), r, []string{"config", "validate", recipesPath})
	if code := ExitCode(err); code != 3 {
		t.Fatalf("broken registry exit = %d (%v)", code, err)
	}
}
