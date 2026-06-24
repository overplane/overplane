package cli

import (
	"context"
	"strings"
	"testing"
)

func TestShellRequiresProject(t *testing.T) {
	setupAgentFixture(t, "")
	var errb strings.Builder
	r := &Runner{Err: &errb}
	if code := ExitCode(Dispatch(context.Background(), r, []string{"shell"})); code != 6 {
		t.Fatalf("shell without project exit = %d stderr=%s", code, errb.String())
	}
}

func TestShellRunsContainer(t *testing.T) {
	fx := setupAgentFixture(t, agentProjectYAML)
	primeShimImage(t, fx,
		[]string{"overplane-demo:latest", "overplane-demo:bhash-cafebabe"},
		agentShellTestImageLabels())
	t.Setenv("OPENAI_API_KEY", "sk-test-shell")
	t.Setenv("EDITOR", "vim")
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("LANG", "C.UTF-8")
	restore := agentEnsureInteractive
	agentEnsureInteractive = func(string) error { return nil }
	defer func() { agentEnsureInteractive = restore }()

	var out, errb strings.Builder
	r := &Runner{In: strings.NewReader(""), Out: &out, Err: &errb}
	if err := Dispatch(context.Background(), r, []string{"shell"}); err != nil {
		t.Fatalf("overplane shell: %v stderr=%s", err, errb.String())
	}
	if !strings.Contains(out.String(), "shell-ran env_OPENAI=sk-test-shell") {
		t.Fatalf("shell output should show passthrough env:\n%s", out.String())
	}
	calls := shimCalls(t, fx)
	if !strings.Contains(calls, "run ") {
		t.Fatalf("expected container run:\n%s", calls)
	}
}

func TestShellUsesPodmanRuntime(t *testing.T) {
	fx := setupAgentFixture(t, agentPodmanProjectYAML)
	primeShimImage(t, fx, []string{"overplane-demo:latest"}, agentShellTestImageLabels())
	t.Setenv("OPENAI_API_KEY", "sk-test")
	restore := agentEnsureInteractive
	agentEnsureInteractive = func(string) error { return nil }
	defer func() { agentEnsureInteractive = restore }()

	var out strings.Builder
	r := &Runner{In: strings.NewReader(""), Out: &out}
	if err := Dispatch(context.Background(), r, []string{"shell"}); err != nil {
		t.Fatalf("podman shell: %v", err)
	}
	calls := shimCalls(t, fx)
	if strings.Contains(calls, "buildx") {
		t.Fatalf("podman shell should not invoke docker buildx:\n%s", calls)
	}
	if !strings.Contains(calls, "run ") {
		t.Fatalf("expected podman run:\n%s", calls)
	}
}
