//go:build integration

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBinaryExecutionIntegration(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "overplane")
	cmd := exec.Command("go", "build", "-trimpath", "-o", bin, ".")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	run := exec.Command(bin, "--version")
	run.Env = append(os.Environ(), "NO_COLOR=1")
	got, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("version: %v\n%s", err, got)
	}
	if !strings.Contains(string(got), "overplane v") {
		t.Fatalf("bad version: %s", got)
	}
	testInitIntegration(t, bin)
	testAgentIntegration(t, bin)
}

// agentEngineShim is a minimal fake docker CLI for binary-exec agent tests:
// version probes succeed, builds record state, image listing and inspection
// serve that state.
const agentEngineShim = `#!/bin/bash
set -u
dir="$AGENT_SHIM_DIR"
printf '%s\n' "$*" >> "$dir/calls.log"
case "$1" in
version) echo "Docker version 26.1.4"; exit 0 ;;
buildx)
  if [ "$2" = version ]; then echo "github.com/docker/buildx v0.14.1"; exit 0; fi
  : > "$dir/tags"; : > "$dir/labels"
  prev=""
  for a in "$@"; do
    case "$prev" in
    --tag) echo "$a" >> "$dir/tags" ;;
    --label) echo "$a" >> "$dir/labels" ;;
    esac
    prev="$a"
  done
  exit 0 ;;
image)
  sub="$2"; shift 2
  if [ "$sub" = ls ]; then
    [ -f "$dir/tags" ] || exit 0
    ok=1; prev=""
    for a in "$@"; do
      if [ "$prev" = --filter ]; then
        case "$a" in
        label=*) grep -qxF "${a#label=}" "$dir/labels" || ok=0 ;;
        esac
      fi
      prev="$a"
    done
    [ "$ok" = 1 ] && cat "$dir/tags"
    exit 0
  fi
  if [ "$sub" = inspect ]; then
    labels=$(awk -F= '{printf "\"%s\":\"%s\",", $1, $2}' "$dir/labels" | sed 's/,$//')
    printf '{"Id":"sha256:abc","Size":1,"Created":"2026-01-02T03:04:05Z","Config":{"Labels":{%s}}}\n' "$labels"
    exit 0
  fi
  exit 1 ;;
tag) echo "$3" >> "$dir/tags"; exit 0 ;;
esac
exit 1
`

// testAgentIntegration exercises the agent group against a PATH-shimmed
// engine in an initialized temp project: usage exit 2, then a fresh setup
// build followed by a cache hit.
func testAgentIntegration(t *testing.T, bin string) {
	t.Helper()
	shims := t.TempDir()
	if err := os.WriteFile(filepath.Join(shims, "docker"), []byte(agentEngineShim), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shims, "git"), []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	shimState := t.TempDir()
	env := []string{
		"PATH=" + shims + ":/usr/bin:/bin",
		"OVERPLANE_TEST_ENGINE_BIN=" + shims,
		"NO_COLOR=1",
		"HOME=" + t.TempDir(),
		"AGENT_SHIM_DIR=" + shimState,
	}
	runIn := func(wantExit int, args ...string) string {
		cmd := exec.Command(bin, args...)
		cmd.Dir = dir
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else if err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
		if code != wantExit {
			t.Fatalf("%v exit = %d, want %d\n%s", args, code, wantExit, out)
		}
		return string(out)
	}
	out := runIn(2, "agent")
	if !strings.Contains(out, "agent <subcommand>") {
		t.Fatalf("bare agent should print usage:\n%s", out)
	}
	runIn(0, "init", "--dir", dir)
	runIn(0, "agent", "setup")
	calls, err := os.ReadFile(filepath.Join(shimState, "calls.log"))
	if err != nil {
		t.Fatal(err)
	}
	builds := strings.Count(string(calls), "buildx build")
	if builds != 1 {
		t.Fatalf("fresh setup builds = %d\n%s", builds, calls)
	}
	runIn(0, "agent", "setup")
	calls, err = os.ReadFile(filepath.Join(shimState, "calls.log"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(calls), "buildx build"); got != builds {
		t.Fatalf("second setup must cache-hit; builds %d -> %d\n%s", builds, got, calls)
	}
}

// testInitIntegration runs the built binary's init twice in a temp dir with a
// hermetic shimmed PATH and asserts the four artifacts plus idempotency.
func testInitIntegration(t *testing.T, bin string) {
	t.Helper()
	shims := t.TempDir()
	writeShim := func(name, script string) {
		if err := os.WriteFile(filepath.Join(shims, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeShim("docker", "#!/bin/sh\necho 'Docker version 26.1.4'\n")
	writeShim("git", `#!/bin/sh
if [ "$1" = rev-parse ] && [ "$2" = --is-inside-work-tree ]; then
  echo true
elif [ "$1" = rev-parse ] && [ "$2" = --abbrev-ref ]; then
  echo main
elif [ "$1" = rev-parse ] && [ "$2" = --short ]; then
  echo abc1234
else
  echo ok
fi
`)
	dir := t.TempDir()
	initOnce := func() string {
		cmd := exec.Command(bin, "init", "--dir", dir)
		cmd.Env = []string{"PATH=" + shims, "NO_COLOR=1", "HOME=" + t.TempDir()}
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("init: %v\n%s", err, out)
		}
		return string(out)
	}
	first := initOnce()
	for _, artifact := range []string{
		"overplane.yaml",
	} {
		if _, err := os.Stat(filepath.Join(dir, artifact)); err != nil {
			t.Fatalf("missing artifact %s after init: %v\n%s", artifact, err, first)
		}
	}
	cfgBefore, err := os.ReadFile(filepath.Join(dir, "overplane.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	second := initOnce()
	cfgAfter, err := os.ReadFile(filepath.Join(dir, "overplane.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(cfgBefore) != string(cfgAfter) {
		t.Fatalf("re-run changed overplane.yaml:\nbefore: %q\nafter: %q", cfgBefore, cfgAfter)
	}
	if !strings.Contains(strings.ToUpper(second), "NEXT STEP") {
		t.Fatalf("re-run missing next-steps table: %s", second)
	}
}

func TestCLIShIntegration(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, "cli.sh")
	run := func(args ...string) string {
		cmd := exec.Command(script, args...)
		cmd.Dir = t.TempDir()
		cmd.Env = append(os.Environ(), "NO_COLOR=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cli.sh %v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	run("version")
	info1, err := os.Stat(filepath.Join(root, "dist", "overplane"))
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond)
	run("version")
	info2, err := os.Stat(filepath.Join(root, "dist", "overplane"))
	if err != nil {
		t.Fatal(err)
	}
	if !info1.ModTime().Equal(info2.ModTime()) {
		t.Fatal("second wrapper invocation rebuilt unexpectedly")
	}
	help := run("help")
	if strings.Contains(help, "build date unknown") || strings.Contains(help, "commit dev") {
		t.Fatalf("wrapper help did not include build metadata: %s", help)
	}
}
