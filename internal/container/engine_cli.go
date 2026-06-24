package container

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// cliEngineClient drives docker or podman through their CLIs.
type cliEngineClient struct {
	engine Engine
	runner commandRunner
}

func newEngineClient(engine Engine, runner commandRunner) (Client, error) {
	switch engine {
	case EngineDocker, EnginePodman:
		return &cliEngineClient{engine: engine, runner: runner}, nil
	case EngineNerdctl, EngineK3s:
		return &stubEngineClient{engine: engine}, nil
	default:
		return nil, wrap(ErrUnknownEngine, fmt.Errorf("unknown engine %d", engine),
			"use docker, podman, nerdctl, or k3s")
	}
}

func (c *cliEngineClient) Engine() Engine { return c.engine }

func (c *cliEngineClient) Capabilities() []Capability { return []Capability{CapBuild, CapRun} }

// Available probes `<engine> version` and enforces the reproducible-build
// version gates: docker needs Buildx/BuildKit >= 0.13, podman needs build
// support for --source-date-epoch and --rewrite-timestamp.
func (c *cliEngineClient) Available(ctx context.Context) error {
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := c.run(checkCtx, "version", []string{"version"}); err != nil {
		return wrap(ErrDaemonUnreachable, err, c.unreachableHint())
	}
	return c.validateVersionGates(checkCtx)
}

func (c *cliEngineClient) validateVersionGates(ctx context.Context) error {
	switch c.engine { //nolint:exhaustive // gates exist for the implemented engines only.
	case EngineDocker:
		res, err := c.run(ctx, "buildx-version", []string{"buildx", "version"})
		if err != nil {
			return wrap(ErrDaemonUnreachable, err,
				"Buildx/BuildKit >= 0.13 is required. Ubuntu: sudo apt-get install -y docker-buildx-plugin; "+
					"verify with: docker buildx version")
		}
		if !versionAtLeast(res.stdout, 0, 13) {
			return wrap(ErrDaemonUnreachable,
				fmt.Errorf("buildx version too old: %s", strings.TrimSpace(res.stdout)),
				"upgrade Docker Buildx/BuildKit to >= 0.13 (Ubuntu: sudo apt-get install -y docker-buildx-plugin)")
		}
	case EnginePodman:
		help, err := c.run(ctx, "build-help", []string{"build", "--help"})
		if err != nil {
			return wrap(ErrDaemonUnreachable, err, "verify podman build works: podman build --help")
		}
		text := help.stdout + "\n" + help.stderr
		if !strings.Contains(text, "--source-date-epoch") || !strings.Contains(text, "--rewrite-timestamp") {
			return wrap(ErrDaemonUnreachable,
				errors.New("podman build lacks reproducible-build flags"),
				"upgrade Podman/Buildah to versions supporting --source-date-epoch and --rewrite-timestamp "+
					"(Podman 5.x / Buildah >= 1.32)")
		}
	}
	return nil
}

func (c *cliEngineClient) BuildLocalImage(ctx context.Context, spec BuildSpec) (BuildResult, error) {
	if err := validateBuildSpec(spec); err != nil {
		return BuildResult{}, err
	}
	start := time.Now()
	args := c.buildCommandArgs(spec)
	env := map[string]string{"SOURCE_DATE_EPOCH": sourceDateEpoch(spec)}
	if _, err := c.runner.Run(ctx, c.engine, "build", args, env, spec.Progress, nil); err != nil {
		return BuildResult{}, wrap(ErrBuildFailed, err, buildFailureHint(err))
	}
	result := BuildResult{Duration: time.Since(start)}
	if len(spec.Tags) > 0 {
		if img, err := c.inspectImage(ctx, spec.Tags[0]); err == nil {
			img.Ref = Ref(spec.Tags[0])
			result.Image = img
		}
	}
	return result, nil
}

func (c *cliEngineClient) buildCommandArgs(spec BuildSpec) []string {
	epoch := sourceDateEpoch(spec)
	var args []string
	switch c.engine { //nolint:exhaustive // stubs never reach this method.
	case EngineDocker:
		args = []string{"buildx", "build", "--file", spec.DockerfilePath,
			"--output", "type=docker,rewrite-timestamp=true"}
	case EnginePodman:
		args = []string{"build", "--file", spec.DockerfilePath,
			"--source-date-epoch=" + epoch, "--timestamp=" + epoch, "--rewrite-timestamp"}
	}
	if spec.NoCache {
		args = append(args, "--no-cache")
	}
	for _, tag := range spec.Tags {
		args = append(args, "--tag", tag)
	}
	buildArgs := map[string]string{"SOURCE_DATE_EPOCH": epoch}
	for k, v := range spec.BuildArgs {
		buildArgs[k] = v
	}
	for _, pair := range sortedPairs(buildArgs) {
		args = append(args, "--build-arg", pair[0]+"="+pair[1])
	}
	for _, pair := range sortedPairs(spec.Labels) {
		args = append(args, "--label", pair[0]+"="+pair[1])
	}
	return append(args, spec.ContextDir)
}

func (c *cliEngineClient) ListLocalImages(ctx context.Context, filter ImageFilter) ([]Image, error) {
	args := []string{"image", "ls", "--format", "{{.Repository}}:{{.Tag}}"}
	for _, pair := range sortedPairs(filter.Labels) {
		args = append(args, "--filter", "label="+pair[0]+"="+pair[1])
	}
	if filter.Reference != "" {
		args = append(args, "--filter", "reference="+filter.Reference)
	}
	res, err := c.run(ctx, "image-ls", args)
	if err != nil {
		return nil, wrap(ErrListFailed, err, "check whether the daemon is running and accessible")
	}
	seen := map[string]bool{}
	var out []Image
	for _, line := range strings.Split(res.stdout, "\n") {
		ref := strings.TrimSpace(line)
		ref = strings.TrimPrefix(ref, "localhost/") // podman prefixes local images
		if ref == "" || strings.Contains(ref, "<none>") || seen[ref] {
			continue
		}
		seen[ref] = true
		img, err := c.inspectImage(ctx, ref)
		if err != nil {
			return nil, wrap(ErrListFailed, err, "image vanished during listing; retry")
		}
		img.Ref = Ref(ref)
		out = append(out, img)
	}
	return out, nil
}

func (c *cliEngineClient) inspectImage(ctx context.Context, ref string) (Image, error) {
	res, err := c.run(ctx, "image-inspect", []string{"image", "inspect", "--format", "{{json .}}", ref})
	if err != nil {
		return Image{}, err
	}
	out := strings.TrimSpace(res.stdout)
	if out == "" {
		return Image{}, errors.New("empty inspect output")
	}
	var row struct {
		ID      string `json:"Id"`
		Size    int64  `json:"Size"`
		Created string `json:"Created"`
		Labels  map[string]string
		Config  struct {
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
	}
	if err := json.Unmarshal([]byte(out), &row); err != nil {
		return Image{}, err
	}
	created, _ := time.Parse(time.RFC3339Nano, row.Created)
	labels := row.Config.Labels
	if len(labels) == 0 {
		labels = row.Labels
	}
	return Image{ID: row.ID, Size: row.Size, Created: created.UTC(), Labels: labels}, nil
}

func (c *cliEngineClient) TagLocalImage(ctx context.Context, src, dst Ref) error {
	if _, err := c.run(ctx, "tag", []string{"tag", src.String(), dst.String()}); err != nil {
		return wrap(ErrBuildFailed, err, "verify the source image exists: "+src.String())
	}
	return nil
}

func (c *cliEngineClient) RunLocalImage(
	ctx context.Context, ref Ref, args []string, opts RunOptions,
) (Container, error) {
	if err := validateRunOptions(opts); err != nil {
		return Container{}, err
	}
	cmdArgs := runCommandArgs(ref, args, opts)
	if opts.Stdin != nil || opts.TTY {
		return c.runStreamingImage(ctx, ref, cmdArgs, opts)
	}
	return c.runBufferedImage(ctx, ref, cmdArgs, opts)
}

// runCommandArgs assembles the `<engine> run` argv. Env values travel
// through the engine subprocess environment (`--env KEY` form), never
// through argv, so secrets stay out of process listings.
func runCommandArgs(ref Ref, args []string, opts RunOptions) []string {
	cmdArgs := []string{"run", "--rm"}
	if opts.Detach {
		cmdArgs = append(cmdArgs, "-d")
	}
	if opts.Stdin != nil {
		cmdArgs = append(cmdArgs, "-i")
	}
	if opts.TTY {
		cmdArgs = append(cmdArgs, "-t")
	}
	if opts.Name != "" {
		cmdArgs = append(cmdArgs, "--name", opts.Name)
	}
	if opts.NetworkMode != "" {
		cmdArgs = append(cmdArgs, "--network", opts.NetworkMode)
	}
	if opts.User != nil {
		cmdArgs = append(cmdArgs, "--user", opts.User.UID+":"+opts.User.GID)
	}
	if opts.WorkDir != "" {
		cmdArgs = append(cmdArgs, "--workdir", opts.WorkDir)
	}
	for _, pair := range sortedPairs(opts.Env) {
		cmdArgs = append(cmdArgs, "--env", pair[0])
	}
	for _, m := range opts.Mounts {
		mode := m.Mode
		if mode == "" {
			mode = "rw"
		}
		cmdArgs = append(cmdArgs, "-v", m.HostPath+":"+m.ContainerPath+":"+mode)
	}
	for _, pair := range sortedPairs(opts.Labels) {
		cmdArgs = append(cmdArgs, "--label", pair[0]+"="+pair[1])
	}
	cmdArgs = append(cmdArgs, ref.String())
	return append(cmdArgs, args...)
}

func (c *cliEngineClient) runStreamingImage(
	ctx context.Context, ref Ref, cmdArgs []string, opts RunOptions,
) (Container, error) {
	err := c.runner.RunStreaming(ctx, c.engine, "run", cmdArgs,
		opts.Stdin, opts.Stdout, opts.Stderr, opts.Env, opts.OnStart)
	if err != nil {
		c.cleanupTimedOutContainer(opts, err)
		if code, ok := exitCodeOf(err); ok {
			return Container{Image: ref, Status: "exited", ExitCode: code}, ExitError{Code: code}
		}
		return Container{}, wrap(ErrRunFailed, err, "check image availability and run arguments")
	}
	return Container{Image: ref, Status: "exited"}, nil
}

func (c *cliEngineClient) runBufferedImage(
	ctx context.Context, ref Ref, cmdArgs []string, opts RunOptions,
) (Container, error) {
	res, err := c.runner.Run(ctx, c.engine, "run", cmdArgs, opts.Env, nil, opts.OnStart)
	if opts.Stdout != nil {
		_, _ = io.WriteString(opts.Stdout, res.stdout)
	}
	if opts.Stderr != nil {
		_, _ = io.WriteString(opts.Stderr, res.stderr)
	}
	if err != nil {
		c.cleanupTimedOutContainer(opts, err)
		if engineRunFailure(res.exitCode) {
			return Container{}, wrap(ErrRunFailed, err, "check image availability and run arguments")
		}
		if code, ok := exitCodeOf(err); ok {
			return Container{Image: ref, Status: "exited", ExitCode: code}, ExitError{Code: code}
		}
		return Container{}, wrap(ErrRunFailed, err, "check image availability and run arguments")
	}
	container := Container{Image: ref, Status: "exited"}
	if opts.Detach {
		container.ID = strings.TrimSpace(res.stdout)
		container.Status = "running"
	}
	return container, nil
}

// engineRunFailure reports whether the exit code denotes an engine-level run
// failure (docker/podman reserve 125-127) rather than the containerized
// command's own status.
func engineRunFailure(code int) bool { return code >= 125 && code <= 127 }

func exitCodeOf(err error) (int, bool) {
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() > 0 {
		return ee.ExitCode(), true
	}
	return 0, false
}

// cleanupTimedOutContainer best-effort stops then kills a named container
// whose run was cancelled or timed out.
func (c *cliEngineClient) cleanupTimedOutContainer(opts RunOptions, runErr error) {
	if opts.Name == "" || runErr == nil {
		return
	}
	if !errors.Is(runErr, context.DeadlineExceeded) && !errors.Is(runErr, context.Canceled) &&
		!strings.Contains(strings.ToLower(runErr.Error()), "context deadline exceeded") {
		return
	}
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer stopCancel()
	_, _ = c.run(stopCtx, "run-timeout-stop", []string{"stop", "--time", "8", opts.Name})
	killCtx, killCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer killCancel()
	_, _ = c.run(killCtx, "run-timeout-kill", []string{"kill", opts.Name})
}

func (c *cliEngineClient) ListLocalRunningContainers(ctx context.Context) ([]Container, error) {
	res, err := c.run(ctx, "ps", []string{"ps", "--format", "{{json .}}"})
	if err != nil {
		return nil, wrap(ErrListFailed, err, "ensure the daemon can list running containers")
	}
	var out []Container
	for _, line := range strings.Split(strings.TrimSpace(res.stdout), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var row struct {
			ID     string `json:"ID"`
			Image  string `json:"Image"`
			Names  string `json:"Names"`
			Status string `json:"Status"`
		}
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			continue
		}
		out = append(out, Container{ID: row.ID, Image: Ref(row.Image), Name: row.Names, Status: row.Status})
	}
	return out, nil
}

func (c *cliEngineClient) KillLocalRunningContainer(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" || strings.ContainsAny(id, " \t\n\r") {
		return wrap(ErrInvalidRunOptions, fmt.Errorf("invalid container id %q", id),
			"pass a single container id or name")
	}
	if _, err := c.run(ctx, "kill", []string{"kill", id}); err != nil {
		return wrap(ErrKillFailed, err, "verify the container id and daemon state")
	}
	return nil
}

func (c *cliEngineClient) run(ctx context.Context, subcommand string, args []string) (commandResult, error) {
	return c.runner.Run(ctx, c.engine, subcommand, args, nil, nil, nil)
}

func (c *cliEngineClient) unreachableHint() string {
	switch c.engine {
	case EngineDocker:
		return "is Docker running? Try: docker info"
	case EnginePodman:
		return "is Podman available? Try: podman info"
	default:
		return "verify daemon installation and permissions"
	}
}

// buildFailureHint classifies an engine build failure into an actionable
// remediation.
func buildFailureHint(err error) string {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "unauthorized"), strings.Contains(msg, "denied"), strings.Contains(msg, "auth"):
		return "registry authentication failed; run the engine's login command for your registry"
	case strings.Contains(msg, "apt"), strings.Contains(msg, "apk"), strings.Contains(msg, "failed to fetch"):
		return "OS package installation failed; verify package names in extra_packages and repository availability"
	case strings.Contains(msg, "network"), strings.Contains(msg, "timeout"), strings.Contains(msg, "tls"):
		return "network error during build; check registry and network connectivity"
	case strings.Contains(msg, "qemu"), strings.Contains(msg, "binfmt"), strings.Contains(msg, "platform"):
		return "platform mismatch; overplane builds native-platform images only"
	default:
		return "inspect the streamed build output above for the failing step"
	}
}

func validateBuildSpec(spec BuildSpec) error {
	switch {
	case strings.TrimSpace(spec.ContextDir) == "":
		return wrap(ErrInvalidBuildSpec, errors.New("missing context dir"), "prepare a build context first")
	case strings.TrimSpace(spec.DockerfilePath) == "":
		return wrap(ErrInvalidBuildSpec, errors.New("missing Dockerfile path"), "prepare a build context first")
	case len(spec.Tags) == 0:
		return wrap(ErrInvalidBuildSpec, errors.New("missing image tags"), "provide at least one tag")
	}
	for _, tag := range spec.Tags {
		if _, err := ParseRef(tag); err != nil {
			return wrap(ErrInvalidBuildSpec, err, "fix the image tag "+tag)
		}
	}
	for k := range spec.BuildArgs {
		if !buildArgKeyRE.MatchString(k) {
			return wrap(ErrInvalidBuildSpec, fmt.Errorf("invalid build arg key %q", k),
				"build arg keys must match ^[A-Z_][A-Z0-9_]*$")
		}
	}
	return nil
}

func validateRunOptions(opts RunOptions) error {
	if strings.ContainsAny(opts.NetworkMode, " \t\n\r") {
		return wrap(ErrInvalidRunOptions, fmt.Errorf("invalid network mode %q", opts.NetworkMode),
			"network mode must be a single daemon-supported token")
	}
	for k := range opts.Env {
		if !envKeyRE.MatchString(k) {
			return wrap(ErrInvalidRunOptions, fmt.Errorf("invalid environment variable name %q", k),
				"environment variable names must match ^[A-Za-z_][A-Za-z0-9_]*$")
		}
	}
	return nil
}

func sourceDateEpoch(spec BuildSpec) string {
	if spec.SourceDateEpoch == "" {
		return "0"
	}
	return spec.SourceDateEpoch
}
