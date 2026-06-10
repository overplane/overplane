package yamlcanon_test

import (
	"testing"

	"github.com/overplane/overplane/internal/platform/serde/yamlcanon"
)

type sample struct {
	B string         `yaml:"b"`
	A map[string]int `yaml:"a"`
}

func TestMarshalStable(t *testing.T) {
	v1 := sample{B: "x", A: map[string]int{"z": 1, "a": 2}}
	v2 := sample{A: map[string]int{"a": 2, "z": 1}, B: "x"}
	a, err := yamlcanon.Marshal(v1)
	if err != nil {
		t.Fatal(err)
	}
	b, err := yamlcanon.MarshalWithBanner("generated", v2)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(a); got != "a: \n  a: !!int 2\n  z: !!int 1\nb: !!str \"x\"\n" {
		t.Fatalf("unexpected yaml:\n%s", got)
	}
	if string(b[:12]) != "# generated\n" {
		t.Fatalf("missing banner: %q", b)
	}
}
