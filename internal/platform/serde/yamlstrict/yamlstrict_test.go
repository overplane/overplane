package yamlstrict_test

import (
	"testing"

	"github.com/overplane/overplane/internal/platform/serde/yamlstrict"
)

type typed struct {
	Name string `yaml:"name"`
}

func TestDecodeStrictAndNormalize(t *testing.T) {
	m, err := yamlstrict.DecodeMap([]byte("name: overplane\nitems:\n  - a\n"))
	if err != nil {
		t.Fatal(err)
	}
	if m["name"] != "overplane" {
		t.Fatalf("bad map: %#v", m)
	}
	var out typed
	if err := yamlstrict.DecodeStrict([]byte("name: ok\n"), &out); err != nil {
		t.Fatal(err)
	}
	if err := yamlstrict.DecodeStrict([]byte("extra: no\n"), &out); err == nil {
		t.Fatal("expected unknown field error")
	}
}
