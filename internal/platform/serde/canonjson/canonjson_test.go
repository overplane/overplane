package canonjson_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/overplane/overplane/internal/platform/serde/canonjson"
)

func TestMarshalStable(t *testing.T) {
	a := map[string]any{"b": 2, "a": map[string]any{"d": true, "c": "x"}}
	b := map[string]any{"a": map[string]any{"c": "x", "d": true}, "b": 2}
	ab, err := canonjson.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	bb, err := canonjson.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	if string(ab) != string(bb) || string(ab) != `{"a":{"c":"x","d":true},"b":2}` {
		t.Fatalf("unstable JSON: %s vs %s", ab, bb)
	}
	path := t.TempDir() + "/data.json"
	if err := os.WriteFile(path, ab, 0o644); err != nil {
		t.Fatal(err)
	}
	var round any
	if err := json.Unmarshal(ab, &round); err != nil {
		t.Fatal(err)
	}
	rb, err := canonjson.MarshalIndent(round, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(rb) {
		t.Fatal("indented output invalid")
	}
}
