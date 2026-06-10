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
