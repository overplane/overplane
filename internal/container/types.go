// Package container is the engine-agnostic container client used by every
// Overplane feature that builds or runs containers. Docker and podman are
// full CLI-driven implementations; nerdctl and k3s are stubs behind the same
// enum, capability routing, and build/run pair matrix so adding them later
// only touches newEngineClient. The exec boundary is an injectable
// commandRunner, so the package tests hermetically with a fake runner. It may
// import internal/platform/* only; build planning (Dockerfiles, embedded
// fragments) belongs to internal/recipes, which prepares contexts this
// package merely drives through the engine.
package container

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

// Engine identifies a container runtime backend.
type Engine int

const (
	EngineDocker Engine = iota
	EnginePodman
	EngineNerdctl
	EngineK3s
)

// String returns the engine command name.
func (e Engine) String() string {
	switch e {
	case EngineDocker:
		return "docker"
	case EnginePodman:
		return "podman"
	case EngineNerdctl:
		return "nerdctl"
	case EngineK3s:
		return "k3s"
	default:
		return "unknown"
	}
}

// ParseEngine maps an engine name (as used in overplane.yaml and the recipe
// registry) to its Engine value.
func ParseEngine(name string) (Engine, error) {
	switch strings.TrimSpace(name) {
	case "docker":
		return EngineDocker, nil
	case "podman":
		return EnginePodman, nil
	case "nerdctl":
		return EngineNerdctl, nil
	case "k3s":
		return EngineK3s, nil
	default:
		return 0, wrap(ErrUnknownEngine, fmt.Errorf("unknown engine %q", name),
			"use docker, podman, nerdctl, or k3s")
	}
}

// Capability identifies a supported engine feature.
type Capability int

const (
	CapBuild Capability = iota
	CapRun
)

// String renders the capability name.
func (c Capability) String() string {
	switch c {
	case CapBuild:
		return "build"
	case CapRun:
		return "run"
	default:
		return "unknown"
	}
}

var refRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/:@-]*$`)

// Ref is a validated image reference (name[:tag][@digest]).
type Ref string

// ParseRef validates an image reference string.
func ParseRef(raw string) (Ref, error) {
	s := strings.TrimSpace(raw)
	if s == "" || !refRE.MatchString(s) || strings.ContainsAny(s, " \t\n\r") {
		return "", wrap(ErrInvalidRef, fmt.Errorf("invalid image reference %q", raw),
			"provide name[:tag] using registry-safe characters")
	}
	return Ref(s), nil
}

// String returns the daemon-usable reference.
func (r Ref) String() string { return string(r) }

// Mount describes a host-to-container mount.
type Mount struct {
	HostPath      string
	ContainerPath string
	Mode          string // "ro" or "rw" (default)
}

// UserSpec selects the uid/gid run identity.
type UserSpec struct {
	UID string
	GID string
}

// Image represents a local image (one row per tag).
type Image struct {
	Ref     Ref
	ID      string
	Size    int64
	Created time.Time
	Labels  map[string]string
}

// Container represents a local container instance.
type Container struct {
	ID       string
	Image    Ref
	Name     string
	Status   string
	ExitCode int
	Labels   map[string]string
}

// ImageFilter narrows ListLocalImages results.
type ImageFilter struct {
	// Labels are label=value filters (all must match).
	Labels map[string]string
	// Reference filters by image reference pattern (engine `reference=`
	// filter semantics).
	Reference string
}

// BuildSpec configures a local image build. The build context is prepared by
// the caller (internal/recipes materializes fragments, the Dockerfile, and
// pinned mtimes); this package only drives the engine.
type BuildSpec struct {
	// ContextDir is the prepared build context directory.
	ContextDir string
	// DockerfilePath is the Dockerfile location (inside or outside the
	// context).
	DockerfilePath string
	// Tags are applied to the built image, in order.
	Tags []string
	// Labels are applied to the built image (sorted before argv assembly).
	Labels map[string]string
	// BuildArgs are Dockerfile ARG values (sorted before argv assembly).
	BuildArgs map[string]string
	// NoCache disables the engine layer cache (used by `agent setup --force`).
	NoCache bool
	// SourceDateEpoch pins image timestamps for reproducible builds;
	// defaults to "0".
	SourceDateEpoch string
	// Progress receives the engine's streamed build output (dim-themed by the
	// runner); nil discards it.
	Progress io.Writer
}

// BuildResult captures the outcome of BuildLocalImage.
type BuildResult struct {
	Image    Image
	Duration time.Duration
}

// RunOptions configures RunLocalImage.
type RunOptions struct {
	Env         map[string]string
	Mounts      []Mount
	NetworkMode string
	User        *UserSpec
	WorkDir     string
	Stdin       io.Reader
	Stdout      io.Writer
	Stderr      io.Writer
	TTY         bool
	Detach      bool
	Name        string
	Labels      map[string]string
	OnStart     func(pid int)
}

// Client is the engine-agnostic abstraction consumed by CLI subcommands and
// future agent runs.
type Client interface {
	Engine() Engine
	Capabilities() []Capability
	// Available probes the engine (`<engine> version`) and enforces the
	// build-toolchain version gates.
	Available(ctx context.Context) error
	BuildLocalImage(ctx context.Context, spec BuildSpec) (BuildResult, error)
	ListLocalImages(ctx context.Context, filter ImageFilter) ([]Image, error)
	// TagLocalImage applies an additional tag to an existing local image.
	TagLocalImage(ctx context.Context, src, dst Ref) error
	RunLocalImage(ctx context.Context, ref Ref, args []string, opts RunOptions) (Container, error)
	ListLocalRunningContainers(ctx context.Context) ([]Container, error)
	KillLocalRunningContainer(ctx context.Context, id string) error
}
