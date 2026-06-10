package env

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	oplog "github.com/overplane/overplane/internal/platform/log"
)

const Prefix = "OVERPLANE"

func Load(ctx context.Context, startDir string) error {
	p, ok := findDotenv(startDir)
	if !ok {
		return nil
	}
	f, err := os.Open(p)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		k, v, ok := parseLine(sc.Text())
		if !ok {
			continue
		}
		if _, exists := os.LookupEnv(k); !exists {
			_ = os.Setenv(k, v)
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	oplog.FromContext(ctx).Debug("loaded env file", "path", p)
	return nil
}

func Normalize() {
	for _, kv := range os.Environ() {
		k, v, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(k, Prefix+"_") {
			_ = os.Setenv(k, strings.TrimSpace(v))
		}
	}
}

func String(name, def string) string {
	if v, ok := os.LookupEnv(Prefix + "_" + name); ok {
		return v
	}
	return def
}

func Bool(name string, def bool) bool {
	if v, ok := os.LookupEnv(Prefix + "_" + name); ok {
		b, err := strconv.ParseBool(v)
		if err == nil {
			return b
		}
	}
	return def
}

func Int(name string, def int) int {
	if v, ok := os.LookupEnv(Prefix + "_" + name); ok {
		i, err := strconv.Atoi(v)
		if err == nil {
			return i
		}
	}
	return def
}

func Passthrough(names []string) map[string]string {
	out := map[string]string{}
	for _, name := range names {
		if v, ok := os.LookupEnv(name); ok {
			out[name] = strings.TrimSpace(v)
		}
	}
	return out
}

func findDotenv(startDir string) (string, bool) {
	if startDir == "" {
		startDir, _ = os.Getwd()
	}
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", false
	}
	for {
		p := filepath.Join(dir, ".env")
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, true
		}
		if st, err := os.Stat(filepath.Join(dir, ".git")); err == nil && st.IsDir() {
			return "", false
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func parseLine(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	line = strings.TrimPrefix(line, "export ")
	k, v, ok := strings.Cut(line, "=")
	if !ok {
		return "", "", false
	}
	k = strings.TrimSpace(k)
	v = strings.TrimSpace(v)
	if i := strings.Index(v, " #"); i >= 0 {
		v = strings.TrimSpace(v[:i])
	}
	if len(v) >= 2 && v[0] == '\'' && v[len(v)-1] == '\'' {
		v = v[1 : len(v)-1]
	}
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		u, err := strconv.Unquote(v)
		if err == nil {
			v = u
		}
	}
	return k, v, k != ""
}
