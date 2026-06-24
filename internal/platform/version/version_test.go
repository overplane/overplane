package version_test

import (
	"strings"
	"testing"

	"github.com/overplane/overplane/internal/platform/version"
)

func TestVersion(t *testing.T) {
	if got := version.String("overplane"); !strings.Contains(got, "overplane v") {
		t.Fatalf("bad version: %s", got)
	}
	if got := version.Runtime(); !strings.Contains(got, "/") {
		t.Fatalf("bad runtime: %s", got)
	}
}

func TestBuildDate(t *testing.T) {
	orig := version.Date
	defer func() { version.Date = orig }()
	version.Date = "2026-06-10T22:01:34Z"
	if got := version.BuildDate(); got != "2026-06-10" {
		t.Fatalf("BuildDate = %q, want 2026-06-10", got)
	}
	version.Date = "unknown"
	if got := version.BuildDate(); got != "unknown" {
		t.Fatalf("BuildDate = %q, want unknown", got)
	}
}
