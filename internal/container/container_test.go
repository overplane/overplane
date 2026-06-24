package container

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// fakeRunner records every invocation and replies from a canned script keyed
// by subcommand.
type fakeRunner struct {
	calls   []fakeCall
	results map[string]fakeResult
}

type fakeCall struct {
	engine     Engine
	subcommand string
	args       []string
	env        map[string]string
}

type fakeResult struct {
	stdout string
	stderr string
	err    error
}

func (f *fakeRunner) Run(
	_ context.Context, engine Engine, subcommand string, args []string,
	env map[string]string, progress io.Writer, _ func(int),
) (commandResult, error) {
	f.calls = append(f.calls, fakeCall{engine: engine, subcommand: subcommand, args: args, env: env})
	res := f.results[subcommand]
	if progress != nil {
		for _, line := range strings.Split(strings.TrimRight(res.stdout, "\n"), "\n") {
			if line != "" {
				writeDimLine(progress, line)
			}
		}
	}
	return commandResult{stdout: res.stdout, stderr: res.stderr}, res.err
}

func (f *fakeRunner) RunStreaming(
	_ context.Context, engine Engine, subcommand string, args []string,
	_ io.Reader, _ io.Writer, _ io.Writer, env map[string]string, _ func(int),
) error {
	f.calls = append(f.calls, fakeCall{engine: engine, subcommand: subcommand, args: args, env: env})
	return f.results[subcommand].err
}

func (f *fakeRunner) callsFor(subcommand string) []fakeCall {
	var out []fakeCall
	for _, c := range f.calls {
		if c.subcommand == subcommand {
			out = append(out, c)
		}
	}
	return out
}

func newFakeClient(t *testing.T, engine Engine, results map[string]fakeResult) (*cliEngineClient, *fakeRunner) {
	t.Helper()
	runner := &fakeRunner{results: results}
	c, err := newEngineClient(engine, runner)
	if err != nil {
		t.Fatal(err)
	}
	return c.(*cliEngineClient), runner
}

func testBuildSpec() BuildSpec {
	return BuildSpec{
		ContextDir:     "/ctx",
		DockerfilePath: "/ctx/Dockerfile",
		Tags:           []string{"overplane-demo:latest", "overplane-demo:bdeadbeef0123"},
		Labels: map[string]string{
			"overplane.project":    "demo",
			"overplane.build.hash": "deadbeef0123",
		},
		BuildArgs: map[string]string{
			"BASE_REF":      "debian:bookworm-slim",
			"CODEX_VERSION": "latest",
		},
	}
}

func TestBuildArgvDocker(t *testing.T) {
	c, runner := newFakeClient(t, EngineDocker, nil)
	if _, err := c.BuildLocalImage(context.Background(), testBuildSpec()); err != nil {
		t.Fatal(err)
	}
	builds := runner.callsFor("build")
	if len(builds) != 1 {
		t.Fatalf("build calls = %d", len(builds))
	}
	want := []string{
		"buildx", "build", "--file", "/ctx/Dockerfile",
		"--output", "type=docker,rewrite-timestamp=true",
		"--tag", "overplane-demo:latest", "--tag", "overplane-demo:bdeadbeef0123",
		"--build-arg", "BASE_REF=debian:bookworm-slim",
		"--build-arg", "CODEX_VERSION=latest",
		"--build-arg", "SOURCE_DATE_EPOCH=0",
		"--label", "overplane.build.hash=deadbeef0123",
		"--label", "overplane.project=demo",
		"/ctx",
	}
	if !reflect.DeepEqual(builds[0].args, want) {
		t.Fatalf("docker build argv:\n got %q\nwant %q", builds[0].args, want)
	}
	if builds[0].env["SOURCE_DATE_EPOCH"] != "0" {
		t.Fatalf("build env = %v", builds[0].env)
	}
}

func TestBuildArgvPodmanNoCache(t *testing.T) {
	c, runner := newFakeClient(t, EnginePodman, nil)
	spec := testBuildSpec()
	spec.NoCache = true
	if _, err := c.BuildLocalImage(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	args := runner.callsFor("build")[0].args
	want := []string{
		"build", "--file", "/ctx/Dockerfile",
		"--source-date-epoch=0", "--timestamp=0", "--rewrite-timestamp",
		"--no-cache",
		"--tag", "overplane-demo:latest", "--tag", "overplane-demo:bdeadbeef0123",
		"--build-arg", "BASE_REF=debian:bookworm-slim",
		"--build-arg", "CODEX_VERSION=latest",
		"--build-arg", "SOURCE_DATE_EPOCH=0",
		"--label", "overplane.build.hash=deadbeef0123",
		"--label", "overplane.project=demo",
		"/ctx",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("podman build argv:\n got %q\nwant %q", args, want)
	}
}

func TestBuildSpecValidation(t *testing.T) {
	c, _ := newFakeClient(t, EngineDocker, nil)
	for name, mutate := range map[string]func(*BuildSpec){
		"missing context": func(s *BuildSpec) { s.ContextDir = "" },
		"missing tags":    func(s *BuildSpec) { s.Tags = nil },
		"bad tag":         func(s *BuildSpec) { s.Tags = []string{"bad tag"} },
		"bad arg key":     func(s *BuildSpec) { s.BuildArgs = map[string]string{"lower": "x"} },
	} {
		t.Run(name, func(t *testing.T) {
			spec := testBuildSpec()
			mutate(&spec)
			if _, err := c.BuildLocalImage(context.Background(), spec); !errors.Is(err, ErrInvalidBuildSpec) {
				t.Fatalf("err = %v, want ErrInvalidBuildSpec", err)
			}
		})
	}
}

func TestRunArgvInteractive(t *testing.T) {
	c, runner := newFakeClient(t, EngineDocker, nil)
	opts := RunOptions{
		Env:         map[string]string{"OPENAI_API_KEY": "sk-secret", "TERM": "xterm"},
		NetworkMode: "host",
		User:        &UserSpec{UID: "1000", GID: "1000"},
		WorkDir:     "/home/dev",
		Stdin:       strings.NewReader(""),
		TTY:         true,
		Name:        "overplane-shell-demo-1",
		Labels:      map[string]string{"overplane.project": "demo", "overplane.shell": "true"},
	}
	if _, err := c.RunLocalImage(context.Background(), "overplane-demo:latest", []string{"bash", "-l"}, opts); err != nil {
		t.Fatal(err)
	}
	call := runner.callsFor("run")[0]
	want := []string{
		"run", "--rm", "-i", "-t",
		"--name", "overplane-shell-demo-1",
		"--network", "host",
		"--user", "1000:1000",
		"--workdir", "/home/dev",
		"--env", "OPENAI_API_KEY", "--env", "TERM",
		"--label", "overplane.project=demo", "--label", "overplane.shell=true",
		"overplane-demo:latest", "bash", "-l",
	}
	if !reflect.DeepEqual(call.args, want) {
		t.Fatalf("run argv:\n got %q\nwant %q", call.args, want)
	}
	// Secrets travel through the subprocess environment, never argv.
	for _, a := range call.args {
		if strings.Contains(a, "sk-secret") {
			t.Fatalf("secret value leaked into argv: %q", call.args)
		}
	}
	if call.env["OPENAI_API_KEY"] != "sk-secret" {
		t.Fatalf("env not forwarded to engine subprocess: %v", call.env)
	}
	for _, a := range call.args {
		if a == "-v" || a == "--mount" {
			t.Fatalf("unexpected mount flag in argv: %q", call.args)
		}
	}
}

func TestRunOptionValidation(t *testing.T) {
	c, _ := newFakeClient(t, EngineDocker, nil)
	if _, err := c.RunLocalImage(context.Background(), "img", nil,
		RunOptions{NetworkMode: "host evil"}); !errors.Is(err, ErrInvalidRunOptions) {
		t.Fatalf("err = %v", err)
	}
	if _, err := c.RunLocalImage(context.Background(), "img", nil,
		RunOptions{Env: map[string]string{"BAD KEY": "x"}}); !errors.Is(err, ErrInvalidRunOptions) {
		t.Fatalf("err = %v", err)
	}
}

const inspectJSON = `{"Id":"sha256:abc123","Size":2147483648,` +
	`"Created":"2026-06-10T10:00:00.5Z",` +
	`"Config":{"Labels":{"overplane.project":"demo","overplane.build.hash":"deadbeef0123"}}}`

func TestListLocalImages(t *testing.T) {
	c, runner := newFakeClient(t, EngineDocker, map[string]fakeResult{
		"image-ls":      {stdout: "overplane-demo:latest\noverplane-demo:bdeadbeef0123\n<none>:<none>\n"},
		"image-inspect": {stdout: inspectJSON},
	})
	images, err := c.ListLocalImages(context.Background(), ImageFilter{
		Labels: map[string]string{"overplane.project": "demo"},
	})
	if err != nil {
		t.Fatal(err)
	}
	lsArgs := runner.callsFor("image-ls")[0].args
	wantLS := []string{
		"image", "ls", "--format", "{{.Repository}}:{{.Tag}}",
		"--filter", "label=overplane.project=demo",
	}
	if !reflect.DeepEqual(lsArgs, wantLS) {
		t.Fatalf("image ls argv = %q, want %q", lsArgs, wantLS)
	}
	if len(images) != 2 {
		t.Fatalf("images = %d (dangling row must be skipped)", len(images))
	}
	img := images[0]
	if img.Ref != "overplane-demo:latest" || img.ID != "sha256:abc123" || img.Size != 2147483648 {
		t.Fatalf("image = %+v", img)
	}
	if !img.Created.Equal(time.Date(2026, 6, 10, 10, 0, 0, 5e8, time.UTC)) {
		t.Fatalf("created = %v", img.Created)
	}
	if img.Labels["overplane.build.hash"] != "deadbeef0123" {
		t.Fatalf("labels = %v", img.Labels)
	}
}

func TestVersionGates(t *testing.T) {
	t.Run("docker buildx ok", func(t *testing.T) {
		c, _ := newFakeClient(t, EngineDocker, map[string]fakeResult{
			"version":        {stdout: "Docker version 29.0.0"},
			"buildx-version": {stdout: "github.com/docker/buildx v0.31.0 abc"},
		})
		if err := c.Available(context.Background()); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("docker buildx too old", func(t *testing.T) {
		c, _ := newFakeClient(t, EngineDocker, map[string]fakeResult{
			"version":        {stdout: "Docker version 24.0.0"},
			"buildx-version": {stdout: "github.com/docker/buildx v0.11.2 abc"},
		})
		if err := c.Available(context.Background()); !errors.Is(err, ErrDaemonUnreachable) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("docker daemon down", func(t *testing.T) {
		c, _ := newFakeClient(t, EngineDocker, map[string]fakeResult{
			"version": {err: errors.New("cannot connect to the Docker daemon")},
		})
		err := c.Available(context.Background())
		if !errors.Is(err, ErrDaemonUnreachable) {
			t.Fatalf("err = %v", err)
		}
		var typed *Error
		if !errors.As(err, &typed) || typed.Hint() == "" {
			t.Fatalf("missing hint: %v", err)
		}
	})
	t.Run("podman flags present", func(t *testing.T) {
		c, _ := newFakeClient(t, EnginePodman, map[string]fakeResult{
			"version":    {stdout: "podman version 5.4.0"},
			"build-help": {stdout: "... --source-date-epoch ... --rewrite-timestamp ..."},
		})
		if err := c.Available(context.Background()); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("podman flags missing", func(t *testing.T) {
		c, _ := newFakeClient(t, EnginePodman, map[string]fakeResult{
			"version":    {stdout: "podman version 4.3.0"},
			"build-help": {stdout: "no reproducibility here"},
		})
		if err := c.Available(context.Background()); !errors.Is(err, ErrDaemonUnreachable) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestValidatePairMatrix(t *testing.T) {
	allowed := [][2]Engine{
		{EngineDocker, EngineDocker},
		{EnginePodman, EnginePodman},
		{EngineNerdctl, EngineDocker},
		{EngineNerdctl, EnginePodman},
		{EngineDocker, EngineK3s},
		{EnginePodman, EngineK3s},
	}
	isAllowed := func(b, r Engine) bool {
		for _, p := range allowed {
			if p[0] == b && p[1] == r {
				return true
			}
		}
		return false
	}
	engines := []Engine{EngineDocker, EnginePodman, EngineNerdctl, EngineK3s}
	for _, b := range engines {
		for _, r := range engines {
			err := ValidatePair(b, r)
			if isAllowed(b, r) && err != nil {
				t.Errorf("ValidatePair(%s, %s) = %v, want nil", b, r, err)
			}
			if !isAllowed(b, r) && !errors.Is(err, ErrUnsupportedCapability) {
				t.Errorf("ValidatePair(%s, %s) = %v, want ErrUnsupportedCapability", b, r, err)
			}
		}
	}
}

func TestStubEngines(t *testing.T) {
	for _, engine := range []Engine{EngineNerdctl, EngineK3s} {
		c, err := NewSingle(engine)
		if err != nil {
			t.Fatal(err)
		}
		if err := c.Available(context.Background()); !errors.Is(err, ErrEngineStub) {
			t.Fatalf("%s Available = %v, want ErrEngineStub", engine, err)
		}
		if _, err := c.BuildLocalImage(context.Background(), BuildSpec{}); !errors.Is(err, ErrEngineStub) {
			t.Fatalf("%s Build = %v", engine, err)
		}
		var typed *Error
		if asErr := c.KillLocalRunningContainer(context.Background(), "x"); !errors.As(asErr, &typed) ||
			typed.Hint() == "" {
			t.Fatalf("%s stub error missing hint", engine)
		}
	}
}

func TestParseEngine(t *testing.T) {
	for name, want := range map[string]Engine{
		"docker": EngineDocker, "podman": EnginePodman, "nerdctl": EngineNerdctl, "k3s": EngineK3s,
	} {
		got, err := ParseEngine(name)
		if err != nil || got != want {
			t.Fatalf("ParseEngine(%q) = %v, %v", name, got, err)
		}
	}
	if _, err := ParseEngine("lxc"); !errors.Is(err, ErrUnknownEngine) {
		t.Fatalf("err = %v", err)
	}
}

func TestParseRef(t *testing.T) {
	for _, ok := range []string{"debian:bookworm-slim", "overplane-demo:latest", "ghcr.io/a/b:1"} {
		if _, err := ParseRef(ok); err != nil {
			t.Fatalf("ParseRef(%q) = %v", ok, err)
		}
	}
	for _, bad := range []string{"", "has space", "-leading"} {
		if _, err := ParseRef(bad); !errors.Is(err, ErrInvalidRef) {
			t.Fatalf("ParseRef(%q) = %v, want ErrInvalidRef", bad, err)
		}
	}
}

func TestMultiClientRouting(t *testing.T) {
	if _, err := New(context.Background(), EngineDocker, EnginePodman); !errors.Is(err, ErrUnsupportedCapability) {
		t.Fatalf("disallowed pair must fail: %v", err)
	}
	c, err := New(context.Background(), EngineNerdctl, EngineDocker)
	if err != nil {
		t.Fatal(err)
	}
	if c.Engine() != EngineDocker {
		t.Fatalf("run engine = %v", c.Engine())
	}
	if _, err := c.BuildLocalImage(context.Background(), BuildSpec{
		ContextDir: "/ctx", DockerfilePath: "/ctx/Dockerfile", Tags: []string{"t:1"},
	}); !errors.Is(err, ErrEngineStub) {
		t.Fatalf("build must route to the nerdctl stub: %v", err)
	}
}

// TestCancellationTerminatesProcessGroup exercises the real runner against a
// PATH-shimmed engine that ignores nothing and sleeps; cancellation must
// terminate the subprocess group promptly (within WaitDelay).
func TestCancellationTerminatesProcessGroup(t *testing.T) {
	bin := t.TempDir()
	shim := filepath.Join(bin, "docker")
	if err := os.WriteFile(shim, []byte("#!/bin/sh\nsleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OVERPLANE_TEST_ENGINE_BIN", bin)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+"/usr/bin"+string(os.PathListSeparator)+"/bin")
	runner := newProcessCommandRunner(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := runner.Run(ctx, EngineDocker, "version", []string{"version"}, nil, nil, nil)
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("runner did not terminate promptly: %v", elapsed)
	}
}

// TestRunExitCodeMapping asserts a non-zero container exit status surfaces as
// ExitError and stdout is forwarded to the caller.
func TestRunExitCodeMapping(t *testing.T) {
	exitErr := exec.Command("sh", "-c", "exit 7").Run()
	c, _ := newFakeClient(t, EngineDocker, map[string]fakeResult{
		"run": {stdout: "out\n", err: exitErr},
	})
	var out bytes.Buffer
	_, err := c.RunLocalImage(context.Background(), "img:latest", []string{"false"}, RunOptions{Stdout: &out})
	var ee ExitError
	if !errors.As(err, &ee) || ee.Code != 7 {
		t.Fatalf("err = %v, want ExitError{7}", err)
	}
	if out.String() != "out\n" {
		t.Fatalf("stdout = %q", out.String())
	}
}

func TestDimProgressStreaming(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	runner := &fakeRunner{results: map[string]fakeResult{
		"build": {stdout: "step1\nstep2\n"},
	}}
	var progress bytes.Buffer
	res, err := runner.Run(context.Background(), EngineDocker, "build", []string{"build"}, nil, &progress, nil)
	if err != nil {
		t.Fatal(err)
	}
	if progress.String() != "step1\nstep2\n" {
		t.Fatalf("progress = %q", progress.String())
	}
	if res.stdout != "step1\nstep2\n" {
		t.Fatalf("stdout = %q", res.stdout)
	}
}
