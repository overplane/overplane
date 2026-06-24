package container

import "errors"

// Error is a typed container-layer error carrying a sentinel kind, an
// optional wrapped cause, and an actionable remediation hint.
type Error struct {
	kind  error
	cause error
	hint  string
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.cause == nil {
		return e.kind.Error()
	}
	return e.kind.Error() + ": " + e.cause.Error()
}

// Unwrap returns the underlying cause.
func (e *Error) Unwrap() error { return e.cause }

// Is reports whether target matches this error's sentinel kind.
func (e *Error) Is(target error) bool { return target == e.kind }

// Hint returns a remediation string for humans.
func (e *Error) Hint() string {
	if e == nil {
		return ""
	}
	return e.hint
}

func wrap(kind, cause error, hint string) error {
	return &Error{kind: kind, cause: cause, hint: hint}
}

var (
	// ErrUnknownEngine indicates an unrecognized engine name.
	ErrUnknownEngine = errors.New("container: unknown engine")
	// ErrEngineStub indicates an engine that is modeled but not implemented.
	ErrEngineStub = errors.New("container: engine stub")
	// ErrUnsupportedCapability indicates the engine lacks the requested
	// capability or the build/run pair is not allowed.
	ErrUnsupportedCapability = errors.New("container: unsupported capability")
	// ErrDaemonUnreachable indicates the engine binary or daemon is
	// unavailable, unhealthy, or too old.
	ErrDaemonUnreachable = errors.New("container: daemon unreachable")
	// ErrInvalidRef indicates image reference validation failure.
	ErrInvalidRef = errors.New("container: invalid image reference")
	// ErrInvalidBuildSpec indicates build spec validation failure.
	ErrInvalidBuildSpec = errors.New("container: invalid build spec")
	// ErrInvalidRunOptions indicates run option validation failure.
	ErrInvalidRunOptions = errors.New("container: invalid run options")
	// ErrBuildFailed indicates the engine reported a build failure.
	ErrBuildFailed = errors.New("container: build failed")
	// ErrRunFailed indicates the engine could not run the container (as
	// opposed to the containerized command exiting non-zero, reported via
	// ExitError).
	ErrRunFailed = errors.New("container: run failed")
	// ErrKillFailed indicates the engine kill command failed.
	ErrKillFailed = errors.New("container: kill failed")
	// ErrListFailed indicates an engine list/inspect command failed.
	ErrListFailed = errors.New("container: list failed")
)

// ExitError reports a non-zero exit status from a containerized command run
// in the foreground; the engine itself worked.
type ExitError struct {
	Code int
}

func (e ExitError) Error() string { return "container: command exited non-zero" }

// Hint returns remediation text (none: the command's own output explains).
func (e ExitError) Hint() string { return "" }
