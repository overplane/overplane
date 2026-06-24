//go:build e2e

// Real-daemon end-to-end tests (#0003 §9): opt-in via -tags=e2e / `make
// e2e`, excluded from `make ci`. They build the default debian agent image
// with a real docker or podman daemon and probe the installed toolchain, so
// they need network access and tens of minutes on a cold cache.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// detectEngine returns the first working real engine, or "".
func detectEngine() string {
	for _, engine := range []string{"docker", "podman"} {
		if _, err := exec.LookPath(engine); err != nil {
			continue
		}
		cmd := exec.Command(engine, "version")
		if err := cmd.Run(); err == nil {
			return engine
		}
	}
	return ""
}

const e2eProjectName = "overplane-e2e"

func e2eProjectYAML(engine string) string {
	return fmt.Sprintf(`dirs: {cache: .cache/overplane, specs: .overplane/specs}
project: {name: %s}
schema_version: 1
agent:
  container:
    agent_recipes:
      - name: codex
        version: latest
      - name: claude-code
        version: latest
      - name: gemini-cli
        version: latest
    base_image: debian:bookworm-slim
    env_passthrough: []
    extra_packages: {}
    runtime: %s
    setup_recipe: debian
`, e2eProjectName, engine)
}

func TestAgentRealDaemonE2E(t *testing.T) {
	engine := detectEngine()
	if engine == "" {
		t.Skip("no docker or podman daemon available")
	}
	bin := filepath.Join(t.TempDir(), "overplane")
	build := exec.Command("go", "build", "-trimpath", "-o", bin, ".")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "overplane.yaml"),
		[]byte(e2eProjectYAML(engine)), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupE2EImages(engine) })

	run := func(timeout time.Duration, args ...string) string {
		t.Helper()
		cmd := exec.Command(bin, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "NO_COLOR=1")
		done := make(chan struct{})
		var out []byte
		var err error
		go func() { out, err = cmd.CombinedOutput(); close(done) }()
		select {
		case <-done:
		case <-time.After(timeout):
			_ = cmd.Process.Kill()
			<-done
			t.Fatalf("%v timed out after %s\n%s", args, timeout, out)
		}
		if err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
		return string(out)
	}

	// Fresh build of the default debian image (network + cold cache heavy).
	setupOut := run(40*time.Minute, "agent", "setup")
	latest := "overplane-" + e2eProjectName + ":latest"
	if !strings.Contains(setupOut, latest) {
		t.Fatalf("setup summary missing %s:\n%s", latest, setupOut)
	}

	// Immediate re-run must be a cache hit (fast, no build).
	start := time.Now()
	run(2*time.Minute, "agent", "setup")
	if elapsed := time.Since(start); elapsed > time.Minute {
		t.Fatalf("cache-hit setup took %s; expected a fast no-build run", elapsed)
	}

	// list-images sees the project-labeled rows with both tags.
	listOut := run(2*time.Minute, "agent", "list-images", "--json")
	jsonStart := strings.Index(listOut, "[")
	if jsonStart < 0 {
		t.Fatalf("no JSON in list-images output:\n%s", listOut)
	}
	var records []map[string]any
	if err := json.Unmarshal([]byte(listOut[jsonStart:]), &records); err != nil {
		t.Fatalf("bad list-images json: %v\n%s", err, listOut)
	}
	if len(records) < 2 || records[0]["tag"] != latest {
		t.Fatalf("list-images records = %+v", records)
	}
	if records[0]["recipe"] != "debian" || records[0]["build_hash"] == "" {
		t.Fatalf("list-images labels = %+v", records[0])
	}

	// Non-interactive probe of the toolchain and the three agent CLIs
	// (the `agent shell -c`-style check from the spec).
	probe := exec.Command(engine, "run", "--rm", "--network", "host", latest,
		"bash", "-lc",
		"codex --version && claude --version && gemini --version && "+
			"z3 --version && go version && rustc --version && node --version && "+
			"! command -v gm")
	probeOut, err := probe.CombinedOutput()
	if err != nil {
		t.Fatalf("toolchain probe: %v\n%s", err, probeOut)
	}
	for _, want := range []string{"Z3 version", "go version go", "rustc "} {
		if !strings.Contains(string(probeOut), want) {
			t.Fatalf("probe output missing %q:\n%s", want, probeOut)
		}
	}
}

// cleanupE2EImages best-effort removes the images the test built.
func cleanupE2EImages(engine string) {
	out, err := exec.Command(engine, "image", "ls", "--format", "{{.Repository}}:{{.Tag}}",
		"--filter", "label=overplane.project="+e2eProjectName).Output()
	if err != nil {
		return
	}
	for _, ref := range strings.Fields(string(out)) {
		_ = exec.Command(engine, "rmi", "--force", ref).Run()
	}
}
