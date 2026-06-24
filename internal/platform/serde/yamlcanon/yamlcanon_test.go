package yamlcanon_test

import (
	"testing"

	"github.com/overplane/overplane/internal/platform/serde/yamlcanon"
	"gopkg.in/yaml.v3"
)

type sample struct {
	B string         `yaml:"b"`
	A map[string]int `yaml:"a"`
}

func TestMarshalPlainDocumented(t *testing.T) {
	type item struct {
		Name string `yaml:"name"`
	}
	type doc struct {
		Title string `yaml:"title"`
		Items []item `yaml:"items"`
	}
	v := doc{Title: "demo", Items: []item{{Name: "a"}}}
	opts := yamlcanon.DocumentOptions{
		Comments: map[string]string{
			"title":      "Human title.",
			"items":      "Listed items.",
			"items.name": "Item name.",
		},
		KeyOrder:           map[string][]string{"": {"title", "items"}},
		TopLevelBlankLines: true,
	}
	got, err := yamlcanon.MarshalPlainDocumented(v, opts)
	if err != nil {
		t.Fatal(err)
	}
	want := "# Human title.\n" +
		"title: demo\n" +
		"\n" +
		"# Listed items.\n" +
		"items:\n" +
		"  -\n" +
		"\n" +
		"    # Item name.\n" +
		"    name: a\n"
	if string(got) != want {
		t.Fatalf("documented yaml mismatch:\n got: %q\nwant: %q", got, want)
	}
	var back doc
	if err := yaml.Unmarshal(got, &back); err != nil {
		t.Fatalf("round-trip parse: %v", err)
	}
	if back.Title != v.Title || len(back.Items) != 1 || back.Items[0] != v.Items[0] {
		t.Fatalf("round-trip mismatch: %+v", back)
	}
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
	if got := string(a); got != "a:\n  a: !!int 2\n  z: !!int 1\nb: !!str \"x\"\n" {
		t.Fatalf("unexpected yaml:\n%s", got)
	}
	if string(b[:12]) != "# generated\n" {
		t.Fatalf("missing banner: %q", b)
	}
}

func TestMarshalPlain(t *testing.T) {
	type inner struct {
		Desc string   `yaml:"description"`
		Tags []string `yaml:"tags"`
	}
	type doc struct {
		Name    string `yaml:"name"`
		Version int    `yaml:"version"`
		Meta    inner  `yaml:"meta"`
		Empty   []int  `yaml:"empty"`
	}
	v := doc{
		Name:    "my-project",
		Version: 1,
		Meta:    inner{Desc: "", Tags: []string{"bootstrap", "001-x", "true"}},
	}
	got, err := yamlcanon.MarshalPlain(v)
	if err != nil {
		t.Fatal(err)
	}
	want := "empty: []\n" +
		"meta:\n" +
		"  description: \"\"\n" +
		"  tags:\n" +
		"    - bootstrap\n" +
		"    - \"001-x\"\n" +
		"    - \"true\"\n" +
		"name: my-project\n" +
		"version: 1\n"
	if string(got) != want {
		t.Fatalf("plain yaml mismatch:\n got: %q\nwant: %q", got, want)
	}
	withBanner, err := yamlcanon.MarshalPlainWithBanner("hello", v)
	if err != nil {
		t.Fatal(err)
	}
	if string(withBanner) != "# hello\n"+want {
		t.Fatalf("banner mismatch: %q", withBanner)
	}
}

// TestMarshalPlainBlockSequences asserts mappings inside sequences render in
// the compact dash form and that scalars with interior colons stay plain,
// while edge-colon and space-bearing strings still quote. Output must
// round-trip through a real YAML parser unchanged.
func TestMarshalPlainBlockSequences(t *testing.T) {
	type item struct {
		Name    string `yaml:"name"`
		Version string `yaml:"version"`
	}
	type doc struct {
		Recipes []item `yaml:"recipes"`
		Base    string `yaml:"base"`
		Edge    string `yaml:"edge"`
		Spaced  string `yaml:"spaced"`
	}
	v := doc{
		Recipes: []item{{Name: "codex", Version: "latest"}, {Name: "claude-code", Version: "2.1.3"}},
		Base:    "debian:bookworm-slim",
		Edge:    "trailing:",
		Spaced:  "a: b",
	}
	got, err := yamlcanon.MarshalPlain(v)
	if err != nil {
		t.Fatal(err)
	}
	want := "base: debian:bookworm-slim\n" +
		"edge: \"trailing:\"\n" +
		"recipes:\n" +
		"  - name: codex\n" +
		"    version: latest\n" +
		"  - name: claude-code\n" +
		"    version: \"2.1.3\"\n" +
		"spaced: \"a: b\"\n"
	if string(got) != want {
		t.Fatalf("plain yaml mismatch:\n got: %q\nwant: %q", got, want)
	}
	var back doc
	if err := yaml.Unmarshal(got, &back); err != nil {
		t.Fatalf("round-trip parse: %v", err)
	}
	if back.Base != v.Base || back.Edge != v.Edge || back.Spaced != v.Spaced ||
		len(back.Recipes) != 2 || back.Recipes[0] != v.Recipes[0] || back.Recipes[1] != v.Recipes[1] {
		t.Fatalf("round-trip mismatch: %+v", back)
	}
}
