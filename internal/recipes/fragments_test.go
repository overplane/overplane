package recipes_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	assetfs "github.com/overplane/overplane/internal/platform/embed/assets"
)

// TestFragmentLint enforces the fragment contract on every embedded
// files/setup/*.sh asset: bash shebang, set -euo pipefail, parses under
// `bash -n`, carries no leftover aperture (APRT) tokens, and agent fragments
// take the version as $1 and end with a --version smoke check.
func TestFragmentLint(t *testing.T) {
	bash, bashErr := exec.LookPath("bash")
	var names []string
	for _, key := range assetfs.Keys() {
		if strings.HasPrefix(key, "files/setup/") && strings.HasSuffix(key, ".sh") {
			names = append(names, key)
		}
	}
	if len(names) < 11 {
		t.Fatalf("expected at least 11 setup fragments, found %d: %v", len(names), names)
	}
	for _, key := range names {
		t.Run(filepath.Base(key), func(t *testing.T) {
			data, err := assetfs.ReadFile(key)
			if err != nil {
				t.Fatal(err)
			}
			text := string(data)
			if !strings.HasPrefix(text, "#!/usr/bin/env bash\n") {
				t.Fatalf("missing bash shebang: %q", firstLine(text))
			}
			if !strings.Contains(text, "set -euo pipefail") {
				t.Fatal("missing set -euo pipefail")
			}
			if lower := strings.ToLower(text); strings.Contains(lower, "aprt") {
				t.Fatal("contains leftover aperture (APRT) token")
			}
			if bashErr == nil {
				path := filepath.Join(t.TempDir(), filepath.Base(key))
				if err := os.WriteFile(path, data, 0o755); err != nil {
					t.Fatal(err)
				}
				if out, err := exec.Command(bash, "-n", path).CombinedOutput(); err != nil {
					t.Fatalf("bash -n failed: %v\n%s", err, out)
				}
			}
			if strings.HasPrefix(filepath.Base(key), "agent-") {
				if !strings.Contains(text, "${1:?") {
					t.Fatal("agent fragment must take the version as $1")
				}
				if !strings.Contains(text, "--version") {
					t.Fatal("agent fragment must end with a --version smoke check")
				}
			}
		})
	}
	if bashErr != nil {
		t.Logf("bash not found; skipped bash -n parse checks: %v", bashErr)
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
