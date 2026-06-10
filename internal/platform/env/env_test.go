package env

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadNormalizeAndPassthrough(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	data := "export OVERPLANE_LOG_LEVEL=' debug '\nKEEP=from-file\nQUOTED=\"a\\nb\"\n"
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KEEP", "process")
	if err := Load(context.Background(), child); err != nil {
		t.Fatal(err)
	}
	Normalize()
	if got := String("LOG_LEVEL", "info"); got != "debug" {
		t.Fatalf("level = %q", got)
	}
	if os.Getenv("KEEP") != "process" {
		t.Fatal("process env overridden")
	}
	t.Setenv("OVERPLANE_FLAG", "true")
	t.Setenv("OVERPLANE_COUNT", "42")
	if !Bool("FLAG", false) || Int("COUNT", 0) != 42 {
		t.Fatal("typed getters failed")
	}
	if got := Passthrough([]string{"KEEP"})["KEEP"]; got != "process" {
		t.Fatalf("passthrough = %q", got)
	}
}
