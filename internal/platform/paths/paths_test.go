package paths_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/overplane/overplane/internal/platform/paths"
)

func TestDiscoverRoot(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "config", "data", "global.yaml"), "project:\n  name: test\n")
	write(t, filepath.Join(root, "config", "schema", "global.schema.json"), `{"type":"object"}`)
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	resolved, err := paths.Resolve(nested)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Root != root {
		t.Fatalf("root = %s, want %s", resolved.Root, root)
	}
}

func write(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}
