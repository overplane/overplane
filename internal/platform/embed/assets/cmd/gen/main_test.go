package main

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCollectRenderAndGzip(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"static/files/misc"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"static/files/misc/a.bin": "abc",
	}
	for p, data := range files {
		if err := os.WriteFile(filepath.Join(root, p), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := collect(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].key == "" {
		t.Fatalf("bad entries: %#v", entries)
	}
	var gz bytes.Buffer
	zr, err := gzip.NewReader(bytes.NewReader(entries[0].data))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gz.ReadFrom(zr); err != nil {
		t.Fatal(err)
	}
	src, err := render(entries)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "generatedAssets") {
		t.Fatalf("bad render: %s", src)
	}
	if key, err := keyFor(root, filepath.Join(root, "static/files/misc/a.bin")); err != nil || key != "files/misc/a.bin" {
		t.Fatalf("key=%q err=%v", key, err)
	}
}
