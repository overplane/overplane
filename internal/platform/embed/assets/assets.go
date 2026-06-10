//go:generate go run ./cmd/gen -root ../../../.. -out assets_gen.go

package assets

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"
)

var FS fs.FS = assetFS{}

func ReadFile(name string) ([]byte, error) {
	return fs.ReadFile(FS, name)
}

func Sub(prefix string) (fs.FS, error) {
	return fs.Sub(FS, prefix)
}

func Keys() []string {
	keys := make([]string, 0, len(generatedAssets))
	for k := range generatedAssets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

type assetFS struct{}

func (assetFS) Open(name string) (fs.File, error) {
	name = strings.TrimPrefix(path.Clean("/"+name), "/")
	if name == "." {
		return &dirFile{name: ".", entries: children("")}, nil
	}
	if b, ok := generatedAssets[name]; ok {
		data, err := gunzip(b)
		if err != nil {
			return nil, err
		}
		return &file{name: path.Base(name), r: bytes.NewReader(data)}, nil
	}
	entries := children(name)
	if len(entries) > 0 {
		return &dirFile{name: path.Base(name), entries: entries}, nil
	}
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

func gunzip(data []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}

func children(prefix string) []fs.DirEntry {
	seen := map[string]bool{}
	if prefix != "" {
		prefix += "/"
	}
	for k := range generatedAssets {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		rest := strings.TrimPrefix(k, prefix)
		name, _, _ := strings.Cut(rest, "/")
		if name != "" {
			seen[name] = strings.Contains(rest, "/")
		}
	}
	out := make([]fs.DirEntry, 0, len(seen))
	for name, isDir := range seen {
		out = append(out, dirEntry{name: name, dir: isDir})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

type file struct {
	name string
	r    *bytes.Reader
}

func (f *file) Stat() (fs.FileInfo, error) {
	return fileInfo{name: f.name, size: int64(f.r.Len())}, nil
}
func (f *file) Read(p []byte) (int, error) { return f.r.Read(p) }
func (f *file) Close() error               { return nil }

type dirFile struct {
	name    string
	entries []fs.DirEntry
	off     int
}

func (d *dirFile) Stat() (fs.FileInfo, error) { return fileInfo{name: d.name, dir: true}, nil }
func (d *dirFile) Read([]byte) (int, error)   { return 0, errors.New("is a directory") }
func (d *dirFile) Close() error               { return nil }
func (d *dirFile) ReadDir(n int) ([]fs.DirEntry, error) {
	if n <= 0 || d.off+n > len(d.entries) {
		return d.entries[d.off:], nil
	}
	out := d.entries[d.off : d.off+n]
	d.off += n
	return out, nil
}

type dirEntry struct {
	name string
	dir  bool
}

func (e dirEntry) Name() string { return e.name }
func (e dirEntry) IsDir() bool  { return e.dir }
func (e dirEntry) Type() fs.FileMode {
	if e.dir {
		return fs.ModeDir
	}
	return 0
}
func (e dirEntry) Info() (fs.FileInfo, error) { return fileInfo{name: e.name, dir: e.dir}, nil }

type fileInfo struct {
	name string
	size int64
	dir  bool
}

func (i fileInfo) Name() string { return i.name }
func (i fileInfo) Size() int64  { return i.size }
func (i fileInfo) Mode() fs.FileMode {
	if i.dir {
		return fs.ModeDir | 0o555
	}
	return 0o444
}
func (i fileInfo) ModTime() time.Time { return time.Unix(0, 0).UTC() }
func (i fileInfo) IsDir() bool        { return i.dir }
func (i fileInfo) Sys() any           { return nil }
