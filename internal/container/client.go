package container

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	oplog "github.com/overplane/overplane/internal/platform/log"
)

var (
	buildArgKeyRE = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)
	envKeyRE      = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	versionRE     = regexp.MustCompile(`(\d+)\.(\d+)`)
)

// New returns a capability-routing client for the build/run engine pair:
// builds and image queries go to buildEngine, container operations to
// runEngine. Identical engines share one client. The pair must be allowed by
// ValidatePair.
func New(ctx context.Context, buildEngine, runEngine Engine) (Client, error) {
	if err := ValidatePair(buildEngine, runEngine); err != nil {
		return nil, err
	}
	runner := newProcessCommandRunner(oplog.FromContext(ctx))
	build, err := newEngineClient(buildEngine, runner)
	if err != nil {
		return nil, err
	}
	if buildEngine == runEngine {
		return build, nil
	}
	run, err := newEngineClient(runEngine, runner)
	if err != nil {
		return nil, err
	}
	return &multiClient{build: build, run: run}, nil
}

// NewSingle returns a client for one engine handling both build and run.
func NewSingle(engine Engine) (Client, error) {
	return newEngineClient(engine, newProcessCommandRunner(nil))
}

// multiClient routes by capability: image builds/queries to the build
// engine, container lifecycle to the run engine.
type multiClient struct {
	build Client
	run   Client
}

func (m *multiClient) Engine() Engine { return m.run.Engine() }

func (m *multiClient) Capabilities() []Capability { return []Capability{CapBuild, CapRun} }

func (m *multiClient) Available(ctx context.Context) error {
	if err := m.build.Available(ctx); err != nil {
		return err
	}
	return m.run.Available(ctx)
}

func (m *multiClient) BuildLocalImage(ctx context.Context, spec BuildSpec) (BuildResult, error) {
	return m.build.BuildLocalImage(ctx, spec)
}

func (m *multiClient) ListLocalImages(ctx context.Context, filter ImageFilter) ([]Image, error) {
	return m.build.ListLocalImages(ctx, filter)
}

func (m *multiClient) TagLocalImage(ctx context.Context, src, dst Ref) error {
	return m.build.TagLocalImage(ctx, src, dst)
}

func (m *multiClient) RunLocalImage(
	ctx context.Context, ref Ref, args []string, opts RunOptions,
) (Container, error) {
	return m.run.RunLocalImage(ctx, ref, args, opts)
}

func (m *multiClient) ListLocalRunningContainers(ctx context.Context) ([]Container, error) {
	return m.run.ListLocalRunningContainers(ctx)
}

func (m *multiClient) KillLocalRunningContainer(ctx context.Context, id string) error {
	return m.run.KillLocalRunningContainer(ctx, id)
}

// stubEngineClient stands in for engines that are modeled but not yet
// implemented (nerdctl, k3s). Every operation returns ErrEngineStub with a
// hint; the enum, capability routing, and pair matrix are real so a future
// implementation only touches newEngineClient.
type stubEngineClient struct {
	engine Engine
}

func (s *stubEngineClient) stub(op string) error {
	return wrap(ErrEngineStub, fmt.Errorf("%s %s is not implemented yet", s.engine, op),
		"use docker or podman; "+s.engine.String()+" support is planned")
}

func (s *stubEngineClient) Engine() Engine { return s.engine }

func (s *stubEngineClient) Capabilities() []Capability {
	switch s.engine {
	case EngineNerdctl:
		return []Capability{CapBuild}
	case EngineK3s:
		return []Capability{CapRun}
	default:
		return nil
	}
}

func (s *stubEngineClient) Available(context.Context) error { return s.stub("availability probe") }

func (s *stubEngineClient) BuildLocalImage(context.Context, BuildSpec) (BuildResult, error) {
	return BuildResult{}, s.stub("build")
}

func (s *stubEngineClient) ListLocalImages(context.Context, ImageFilter) ([]Image, error) {
	return nil, s.stub("image listing")
}

func (s *stubEngineClient) TagLocalImage(context.Context, Ref, Ref) error {
	return s.stub("tag")
}

func (s *stubEngineClient) RunLocalImage(context.Context, Ref, []string, RunOptions) (Container, error) {
	return Container{}, s.stub("run")
}

func (s *stubEngineClient) ListLocalRunningContainers(context.Context) ([]Container, error) {
	return nil, s.stub("container listing")
}

func (s *stubEngineClient) KillLocalRunningContainer(context.Context, string) error {
	return s.stub("kill")
}

// versionAtLeast extracts the first major.minor pair in raw and compares it
// against the required gate.
func versionAtLeast(raw string, major, minor int) bool {
	m := versionRE.FindStringSubmatch(raw)
	if len(m) != 3 {
		return false
	}
	maj, _ := strconv.Atoi(m[1])
	minorVal, _ := strconv.Atoi(m[2])
	if maj != major {
		return maj > major
	}
	return minorVal >= minor
}
