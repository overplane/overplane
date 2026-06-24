package container

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/overplane/overplane/internal/platform/color"
)

// commandResult captures subprocess execution details.
type commandResult struct {
	stdout   string
	stderr   string
	exitCode int
}

// commandRunner is the injectable exec boundary: production uses
// processCommandRunner; tests substitute a fake to assert argv and feed
// canned output without a real daemon.
type commandRunner interface {
	// Run executes `<engine> <args...>` buffered, streaming each output line
	// to progress (dim) when non-nil, otherwise into the logger.
	Run(
		ctx context.Context,
		engine Engine,
		subcommand string,
		args []string,
		env map[string]string,
		progress io.Writer,
		onStart func(pid int),
	) (commandResult, error)
	// RunStreaming executes `<engine> <args...>` interactively, wiring the
	// caller's stdio directly and leaving the child in the foreground process
	// group so TTY and signal behavior stay correct.
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

type processCommandRunner struct {
	logger *slog.Logger
}

func newProcessCommandRunner(logger *slog.Logger) commandRunner {
	return &processCommandRunner{logger: logger}
}

func (r *processCommandRunner) Run(
	ctx context.Context,
	engine Engine,
	subcommand string,
	args []string,
	env map[string]string,
	progress io.Writer,
	onStart func(pid int),
) (commandResult, error) {
	cmd := newEngineCommand(ctx, engine, args, env)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return commandResult{}, err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return commandResult{}, err
	}
	var stdoutBuf, stderrBuf bytes.Buffer
	if err := cmd.Start(); err != nil {
		return commandResult{}, err
	}
	if onStart != nil && cmd.Process != nil {
		onStart(cmd.Process.Pid)
	}
	progress = serialProgressWriter(progress)
	stdoutCh := make(chan error, 1)
	stderrCh := make(chan error, 1)
	go func() {
		stdoutCh <- r.streamOutput(engine, subcommand, "stdout", stdoutPipe, &stdoutBuf, progress)
	}()
	go func() {
		stderrCh <- r.streamOutput(engine, subcommand, "stderr", stderrPipe, &stderrBuf, progress)
	}()
	err = cmd.Wait()
	stdoutErr := <-stdoutCh
	stderrErr := <-stderrCh
	if streamErr := firstStreamErr(stdoutErr, stderrErr); streamErr != nil {
		return commandResult{}, streamErr
	}
	return finishResult(cmd, &stdoutBuf, &stderrBuf, err)
}

// testEngineBinEnv, when set to a directory, forces engine subprocesses to
// execute the named shim at $dir/<engine> instead of resolving via PATH.
// CLI tests set this so CI runners with a real docker/podman never win lookup.
const testEngineBinEnv = "OVERPLANE_TEST_ENGINE_BIN"

func engineExecutable(engine Engine) string {
	if dir := os.Getenv(testEngineBinEnv); dir != "" {
		return filepath.Join(dir, engine.String())
	}
	return engine.String()
}

// newEngineCommand builds the engine subprocess with its own process group
// and group-terminating cancellation so context cancellation reaps children.
func newEngineCommand(ctx context.Context, engine Engine, args []string, env map[string]string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, engineExecutable(engine), args...)
	cmd.Env = append(os.Environ(), commandEnv(env)...)
	setProcessGroup(cmd)
	cmd.WaitDelay = 5 * time.Second
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return terminateProcessGroup(cmd.Process.Pid)
	}
	return cmd
}

func finishResult(cmd *exec.Cmd, stdoutBuf, stderrBuf *bytes.Buffer, err error) (commandResult, error) {
	res := commandResult{stdout: stdoutBuf.String(), stderr: stderrBuf.String()}
	if cmd.ProcessState != nil {
		res.exitCode = cmd.ProcessState.ExitCode()
	}
	if err != nil {
		if res.stderr != "" {
			return res, fmt.Errorf("%w: %s", err, lastLine(res.stderr))
		}
		return res, err
	}
	return res, nil
}

func (r *processCommandRunner) RunStreaming(
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
	_ = subcommand
	cmd := exec.CommandContext(ctx, engineExecutable(engine), args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = append(os.Environ(), commandEnv(env)...)
	// Interactive streaming commands stay in the foreground process group so
	// the terminal remains usable and signal/TTY behavior is correct.
	if err := cmd.Start(); err != nil {
		return err
	}
	if onStart != nil && cmd.Process != nil {
		onStart(cmd.Process.Pid)
	}
	return cmd.Wait()
}

func (r *processCommandRunner) streamOutput(
	engine Engine,
	subcommand string,
	stream string,
	reader io.Reader,
	dest *bytes.Buffer,
	progress io.Writer,
) error {
	br := bufio.NewReader(reader)
	for {
		line, err := br.ReadString('\n')
		if line != "" {
			_, _ = dest.WriteString(line)
			clean := strings.TrimRight(line, "\r\n")
			if progress != nil {
				writeDimLine(progress, clean)
			} else {
				r.logLine(engine, subcommand, stream, clean)
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

func (r *processCommandRunner) logLine(engine Engine, subcommand, stream, line string) {
	if r.logger == nil || line == "" || stream != "stderr" {
		return
	}
	r.logger.Debug(line,
		slog.String("engine", engine.String()),
		slog.String("subcommand", subcommand),
		slog.String("stream", stream),
	)
}

// writeDimLine writes one line of engine progress output, dimmed when color
// is enabled and plain under NO_COLOR.
func writeDimLine(w io.Writer, line string) {
	if line == "" {
		return
	}
	if color.Enabled() {
		_, _ = fmt.Fprintf(w, "\x1b[2m%s\x1b[0m\n", line)
		return
	}
	_, _ = fmt.Fprintln(w, line)
}

type lockedWriter struct {
	w  io.Writer
	mu sync.Mutex
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

func serialProgressWriter(w io.Writer) io.Writer {
	if w == nil {
		return nil
	}
	return &lockedWriter{w: w}
}

func firstStreamErr(streamErrs ...error) error {
	for _, streamErr := range streamErrs {
		if !isIgnorableStreamErr(streamErr) {
			return streamErr
		}
	}
	return nil
}

func isIgnorableStreamErr(err error) bool {
	if err == nil || errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "file already closed")
}

func commandEnv(in map[string]string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, pair := range sortedPairs(in) {
		out = append(out, pair[0]+"="+pair[1])
	}
	return out
}

func sortedPairs(in map[string]string) [][2]string {
	if len(in) == 0 {
		return nil
	}
	keys := make([]string, 0, len(in))
	for k := range in {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([][2]string, 0, len(in))
	for _, k := range keys {
		out = append(out, [2]string{k, in[k]})
	}
	return out
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}
