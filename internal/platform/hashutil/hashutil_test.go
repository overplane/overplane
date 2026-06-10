package hashutil_test

import (
	"os"
	"strings"
	"testing"

	"github.com/overplane/overplane/internal/platform/hashutil"
)

func TestHashutil(t *testing.T) {
	if got := hashutil.EmptySHA256(); got != "e3b0c44298fc" {
		t.Fatalf("empty digest = %s", got)
	}
	h := hashutil.NewHasher()
	h.WriteString("hello")
	if got, want := h.Sum(), hashutil.SumString("hello"); got != want {
		t.Fatalf("hasher = %s, want %s", got, want)
	}
	path := t.TempDir() + "/payload"
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 4096)), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := hashutil.SumFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := hashutil.SumBytes([]byte(strings.Repeat("x", 4096))); got != want {
		t.Fatalf("file digest = %s, want %s", got, want)
	}
}
