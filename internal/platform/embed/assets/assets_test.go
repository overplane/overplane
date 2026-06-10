package assets_test

import (
	"testing"

	"github.com/overplane/overplane/internal/platform/embed/assets"
)

func TestAssetsFS(t *testing.T) {
	keys := assets.Keys()
	if len(keys) == 0 {
		t.Fatal("no assets")
	}
	if _, err := assets.ReadFile("files/misc/banner.ans"); err != nil {
		t.Fatal(err)
	}
	sub, err := assets.Sub("files")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sub.Open("misc/banner.ans"); err != nil {
		t.Fatal(err)
	}
	if _, err := assets.FS.Open("missing"); err == nil {
		t.Fatal("expected missing asset error")
	}
}
