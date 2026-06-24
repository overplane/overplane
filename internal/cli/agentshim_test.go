package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/overplane/overplane/internal/container"
)

// agentTestFixture holds an in-process engine shim wired into agentEngineClient.
type agentTestFixture struct {
	shim *agentEngineShim
	bin  string
}

func (f *agentTestFixture) calls() string { return f.shim.calls() }

func (f *agentTestFixture) primeImage(tags, labels []string) {
	f.shim.prime(tags, labels)
}

// agentEngineShim simulates docker/podman CLI behavior in-process for agent tests.
type agentEngineShim struct {
	mu          sync.Mutex
	engine      container.Engine
	callLog     []string
	tags        []string
	labels      []string
	runExit     int
	versionFail bool
}

func newAgentEngineShim(runtime string) *agentEngineShim {
	engine := container.EngineDocker
	if runtime == "podman" {
		engine = container.EnginePodman
	}
	return &agentEngineShim{engine: engine}
}

func (s *agentEngineShim) calls() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return strings.Join(s.callLog, "\n")
}

func (s *agentEngineShim) prime(tags, labels []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tags = append([]string(nil), tags...)
	s.labels = append([]string(nil), labels...)
}

func (s *agentEngineShim) record(args []string) {
	s.callLog = append(s.callLog, strings.Join(args, " "))
}

func (s *agentEngineShim) Run(
	_ context.Context,
	_ container.Engine,
	_ string,
	args []string,
	_ map[string]string,
	progress io.Writer,
	_ func(int),
) (container.ScriptRunResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record(args)
	if len(args) == 0 {
		return container.ScriptRunResult{}, errors.New("missing argv")
	}
	switch args[0] {
	case "version":
		if s.versionFail {
			return container.ScriptRunResult{}, errors.New("engine unavailable")
		}
		if s.engine == container.EnginePodman {
			return container.ScriptRunResult{Stdout: "podman version 5.4.0\n"}, nil
		}
		return container.ScriptRunResult{Stdout: "Docker version 26.1.4\n"}, nil
	case "buildx":
		return s.runBuildx(args, progress)
	case "build":
		return s.runBuild(args, progress)
	case "image":
		return s.runImage(args)
	case "tag":
		if len(args) < 3 {
			return container.ScriptRunResult{}, errors.New("tag: missing args")
		}
		s.tags = append(s.tags, args[2])
		return container.ScriptRunResult{}, nil
	default:
		return container.ScriptRunResult{}, fmt.Errorf("unsupported: %s", args[0])
	}
}

func (s *agentEngineShim) RunStreaming(
	_ context.Context,
	_ container.Engine,
	_ string,
	args []string,
	_ io.Reader,
	stdout io.Writer,
	_ io.Writer,
	env map[string]string,
	_ func(int),
) error {
	s.mu.Lock()
	s.record(args)
	exit := s.runExit
	s.mu.Unlock()
	openai := envOrUnset("OPENAI_API_KEY", env)
	if _, err := fmt.Fprintf(stdout, "shell-ran env_OPENAI=%s\n", openai); err != nil {
		return err
	}
	if exit != 0 {
		return exec.Command("sh", "-c", fmt.Sprintf("exit %d", exit)).Run()
	}
	return nil
}

func envOrUnset(key string, env map[string]string) string {
	if v, ok := env[key]; ok && v != "" {
		return v
	}
	if v := os.Getenv(key); v != "" {
		return v
	}
	return "unset"
}

func (s *agentEngineShim) runBuildx(args []string, progress io.Writer) (container.ScriptRunResult, error) {
	if len(args) > 1 && args[1] == "version" {
		return container.ScriptRunResult{Stdout: "github.com/docker/buildx v0.14.1\n"}, nil
	}
	s.resetImageState()
	s.collectTagsLabels(args)
	writeProgress(progress, "#1 DONE")
	return container.ScriptRunResult{Stdout: "#1 DONE\n"}, nil
}

func (s *agentEngineShim) runBuild(args []string, progress io.Writer) (container.ScriptRunResult, error) {
	if len(args) > 1 && args[1] == "--help" {
		return container.ScriptRunResult{Stdout: "  --source-date-epoch\n  --rewrite-timestamp\n"}, nil
	}
	s.resetImageState()
	s.collectTagsLabels(args)
	writeProgress(progress, "#1 DONE")
	return container.ScriptRunResult{Stdout: "#1 DONE\n"}, nil
}

func (s *agentEngineShim) resetImageState() {
	s.tags = nil
	s.labels = nil
}

func (s *agentEngineShim) collectTagsLabels(args []string) {
	prev := ""
	for _, a := range args {
		switch prev {
		case "--tag":
			s.tags = append(s.tags, a)
		case "--label":
			s.labels = append(s.labels, a)
		}
		prev = a
	}
}

func (s *agentEngineShim) runImage(args []string) (container.ScriptRunResult, error) {
	if len(args) < 2 {
		return container.ScriptRunResult{}, errors.New("image: missing subcommand")
	}
	switch args[1] {
	case "ls":
		return container.ScriptRunResult{Stdout: s.imageList(args[2:])}, nil
	case "inspect":
		return container.ScriptRunResult{Stdout: s.imageInspect()}, nil
	default:
		return container.ScriptRunResult{}, fmt.Errorf("image: %s", args[1])
	}
}

func (s *agentEngineShim) imageList(args []string) string {
	if len(s.tags) == 0 {
		return ""
	}
	labelFilters, refFilter := parseImageFilters(args)
	if !labelLinesMatch(labelFilters, s.labels) {
		return ""
	}
	var out []string
	for _, tag := range s.tags {
		if refFilter != "" && tag != refFilter {
			continue
		}
		out = append(out, tag)
	}
	return strings.Join(out, "\n") + func() string {
		if len(out) > 0 {
			return "\n"
		}
		return ""
	}()
}

func parseImageFilters(args []string) (labels []string, ref string) {
	prev := ""
	for _, a := range args {
		if prev == "--filter" {
			switch {
			case strings.HasPrefix(a, "label="):
				labels = append(labels, strings.TrimPrefix(a, "label="))
			case strings.HasPrefix(a, "reference="):
				ref = strings.TrimPrefix(a, "reference=")
			}
		}
		prev = a
	}
	return labels, ref
}

func labelLinesMatch(want, have []string) bool {
	if len(want) == 0 {
		return true
	}
	set := make(map[string]struct{}, len(have))
	for _, line := range have {
		set[line] = struct{}{}
	}
	for _, w := range want {
		if _, ok := set[w]; !ok {
			return false
		}
	}
	return true
}

func (s *agentEngineShim) imageInspect() string {
	labels := map[string]string{}
	for _, line := range s.labels {
		key, val, ok := strings.Cut(line, "=")
		if ok {
			labels[key] = val
		}
	}
	b, _ := json.Marshal(map[string]any{
		"Id":      "sha256:0123456789abcdef0123",
		"Size":    int64(1234567890),
		"Created": "2026-01-02T03:04:05Z",
		"Config":  map[string]any{"Labels": labels},
	})
	return string(b) + "\n"
}

func writeProgress(w io.Writer, line string) {
	if w == nil {
		return
	}
	_, _ = fmt.Fprintln(w, line)
}

func installAgentEngineHook(t *testing.T, shim *agentEngineShim) {
	t.Helper()
	agentEngineClientHook = func(ctx context.Context, runtimeName string) (container.Client, error) {
		engine, err := container.ParseEngine(runtimeName)
		if err != nil {
			return nil, InternalError(err)
		}
		client, err := container.NewScriptClient(engine, shim)
		if err != nil {
			return nil, InternalError(err)
		}
		if err := client.Available(ctx); err != nil {
			return nil, EnvError(withHint(err))
		}
		return client, nil
	}
	t.Cleanup(func() { agentEngineClientHook = nil })
}

// writeCheckEngineShim installs a minimal engine binary for `overplane check`
// and setup preflight probes (version only).
func writeCheckEngineShim(t *testing.T, binDir, runtime string) {
	t.Helper()
	script := `#!/bin/sh
case "$1" in
version) echo "Docker version 26.1.4"; exit 0 ;;
esac
exit 1
`
	if runtime == "podman" {
		script = `#!/bin/sh
case "$1" in
version) echo "podman version 5.4.0"; exit 0 ;;
build)
  if [ "$2" = --help ]; then
    echo "  --source-date-epoch"
    echo "  --rewrite-timestamp"
    exit 0
  fi
  ;;
esac
exit 1
`
	}
	writeTestFile(t, filepath.Join(binDir, runtime), script)
}
