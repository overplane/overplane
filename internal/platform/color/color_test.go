package color_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/overplane/overplane/internal/platform/color"
)

func TestColorAndHelp(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	color.ResetForTest()
	p := color.Get()
	p[0] = 1
	color.Set(p)
	if color.FG(0) != "" || color.BG(0) != "" || color.Sprint(0, "x") != "x" {
		t.Fatal("color should be disabled")
	}
	var b bytes.Buffer
	if _, err := color.Fprint(&b, 0, "x"); err != nil || b.String() != "x" {
		t.Fatalf("fprint = %q %v", b.String(), err)
	}
	help := color.RenderHelp(color.HelpSpec{
		Command: "cmd", Usage: "cmd [flags]", Description: "desc",
		Flags:    []color.HelpFlag{{Name: "--x", Placeholder: "N", Description: "value"}},
		Examples: []string{"cmd --x 1"},
	})
	if !strings.Contains(help, "Flags") || !strings.Contains(help, "Examples") {
		t.Fatalf("bad help: %s", help)
	}
	tw := color.Table(&b)
	tw.AppendHeader(table.Row{"a"})
	tw.AppendRow(table.Row{"b"})
	tw.Render()
	if !strings.Contains(b.String(), "b") {
		t.Fatal("table did not render")
	}
}

func TestResolveTheme(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	color.ResetForTest()
	dir := t.TempDir()
	path := writeThemeFixture(t, dir)
	res, err := color.ResolveTheme(dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != path || res.Theme.Palette[15] != 16 {
		t.Fatalf("bad theme resolution: %#v", res)
	}
	t.Setenv("OVERPLANE_THEME", filepath.Join(dir, "missing.yaml"))
	if _, err := color.ResolveTheme(dir); err == nil {
		t.Fatal("expected explicit theme error")
	}
}

func writeThemeFixture(t *testing.T, dir string) string {
	t.Helper()
	writeColorTestFile(t, filepath.Join(dir, "config", "data", "global.yaml"),
		"project:\n  name: test\n  website: https://example.com\n")
	writeColorTestFile(t, filepath.Join(dir, "config", "schema", "global.schema.json"), "{}")
	writeColorTestFile(t, filepath.Join(dir, "config", "schema", "theme.schema.json"), "{}")
	path := filepath.Join(dir, "config", "data", "theme.yaml")
	writeColorTestFile(t, path, `fonts:
  families:
    body:
      family: Body
      source: system
  roles:
    body: body
    heading: body
    mono: body
colors:
  light:
    bg: '#000000'
    surface: '#000000'
    surface-raised: '#000000'
    text: '#ffffff'
    text-muted: '#ffffff'
    border: '#ffffff'
    accent: '#ffffff'
    accent-hover: '#ffffff'
  dark:
    bg: '#000000'
    surface: '#000000'
    surface-raised: '#000000'
    text: '#ffffff'
    text-muted: '#ffffff'
    border: '#ffffff'
    accent: '#ffffff'
    accent-hover: '#ffffff'
spacing:
  base: 1
radii:
  sm: 1
terminal:
  name: test
  palette: [1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16]
`)
	return path
}

func writeColorTestFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}
