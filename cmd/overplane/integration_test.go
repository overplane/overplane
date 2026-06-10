//go:build integration

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBinaryExecutionIntegration(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "overplane")
	cmd := exec.Command("go", "build", "-trimpath", "-o", bin, ".")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	run := exec.Command(bin, "--version")
	run.Env = append(os.Environ(), "NO_COLOR=1")
	got, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("version: %v\n%s", err, got)
	}
	if !strings.Contains(string(got), "overplane v") {
		t.Fatalf("bad version: %s", got)
	}
}

func TestCLIShIntegration(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, "cli.sh")
	run := func(args ...string) string {
		cmd := exec.Command(script, args...)
		cmd.Dir = t.TempDir()
		cmd.Env = append(os.Environ(), "NO_COLOR=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cli.sh %v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	run("version")
	info1, err := os.Stat(filepath.Join(root, "dist", "overplane"))
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond)
	run("version")
	info2, err := os.Stat(filepath.Join(root, "dist", "overplane"))
	if err != nil {
		t.Fatal(err)
	}
	if !info1.ModTime().Equal(info2.ModTime()) {
		t.Fatal("second wrapper invocation rebuilt unexpectedly")
	}
	help := run("help")
	if strings.Contains(help, "build date unknown") || strings.Contains(help, "commit dev") {
		t.Fatalf("wrapper help did not include build metadata: %s", help)
	}
}
