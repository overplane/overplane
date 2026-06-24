package container

import (
	"context"
	"io"
)

// ScriptRunResult is the stdout/stderr captured from a scripted engine command.
type ScriptRunResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// ScriptRunner is the injectable exec boundary for tests that simulate docker
// or podman in-process instead of spawning a real engine CLI.
type ScriptRunner interface {
	Run(
		ctx context.Context,
		engine Engine,
		subcommand string,
		args []string,
		env map[string]string,
		progress io.Writer,
		onStart func(pid int),
	) (ScriptRunResult, error)
	RunStreaming(
		ctx context.Context,
		engine Engine,
		subcommand string,
		args []string,
		stdin io.Reader,
		stdout io.Writer,
		stderr io.Writer,
		env map[string]string,
		onStart func(pid int),
	) error
}

type scriptRunnerAdapter struct{ inner ScriptRunner }

func (a scriptRunnerAdapter) Run(
	ctx context.Context,
	engine Engine,
	subcommand string,
	args []string,
	env map[string]string,
	progress io.Writer,
	onStart func(pid int),
) (commandResult, error) {
	res, err := a.inner.Run(ctx, engine, subcommand, args, env, progress, onStart)
	return commandResult{stdout: res.Stdout, stderr: res.Stderr, exitCode: res.ExitCode}, err
}

func (a scriptRunnerAdapter) RunStreaming(
	ctx context.Context,
	engine Engine,
	subcommand string,
	args []string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
	env map[string]string,
	onStart func(pid int),
) error {
	return a.inner.RunStreaming(ctx, engine, subcommand, args, stdin, stdout, stderr, env, onStart)
}

// NewScriptClient returns a Client that drives the engine through runner.
func NewScriptClient(engine Engine, runner ScriptRunner) (Client, error) {
	return newEngineClient(engine, scriptRunnerAdapter{inner: runner})
}
