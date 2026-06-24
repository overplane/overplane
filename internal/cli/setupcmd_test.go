package cli

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupRequiresProject(t *testing.T) {
	setupAgentFixture(t, "")
	var errb strings.Builder
	r := &Runner{Err: &errb}
	if code := ExitCode(Dispatch(context.Background(), r, []string{"setup"})); code != 6 {
		t.Fatalf("setup without project exit = %d stderr=%s", code, errb.String())
	}
}

func TestSetupRunsChecksBeforeBuild(t *testing.T) {
	fx := setupAgentFixture(t, agentProjectYAML)
	writeTestFile(t, filepath.Join(fx.bin, "docker"), "#!/bin/sh\nexit 1\n")
	var errb strings.Builder
	r := &Runner{Err: &errb}
	err := Dispatch(context.Background(), r, []string{"setup"})
	if code := ExitCode(err); code != 6 {
		t.Fatalf("setup with unavailable engine exit = %d err=%v stderr=%s", code, err, errb.String())
	}
	if err == nil || !strings.Contains(err.Error(), "checks failed") {
		t.Fatalf("expected check failure before build: err=%v stderr=%s", err, errb.String())
	}
	if calls := shimCalls(t, fx); strings.Contains(calls, "buildx build") {
		t.Fatalf("build should not run when checks fail:\n%s", calls)
	}
}

func TestSetupBuildsImage(t *testing.T) {
	fx := setupAgentFixture(t, agentProjectYAML)
	t.Setenv("OPENAI_API_KEY", "sk-test")
	var out strings.Builder
	r := &Runner{Out: &out}
	if code := ExitCode(Dispatch(context.Background(), r, []string{"setup"})); code != 0 {
		t.Fatalf("setup exit = %d out=%s", code, out.String())
	}
	calls := shimCalls(t, fx)
	if !strings.Contains(calls, "buildx build") {
		t.Fatalf("expected image build:\n%s", calls)
	}
	if !strings.Contains(out.String(), "overplane-demo:latest") {
		t.Fatalf("missing setup summary:\n%s", out.String())
	}
}

func TestSetupBuildsImageWithPodman(t *testing.T) {
	fx := setupAgentFixture(t, agentPodmanProjectYAML)
	t.Setenv("OPENAI_API_KEY", "sk-test")
	var out strings.Builder
	r := &Runner{Out: &out}
	if code := ExitCode(Dispatch(context.Background(), r, []string{"setup"})); code != 0 {
		t.Fatalf("setup exit = %d out=%s", code, out.String())
	}
	calls := shimCalls(t, fx)
	if !strings.Contains(calls, "build ") || strings.Contains(calls, "buildx") {
		t.Fatalf("expected podman image build:\n%s", calls)
	}
	if strings.Contains(calls, "docker") {
		t.Fatalf("podman setup should not invoke docker:\n%s", calls)
	}
}
