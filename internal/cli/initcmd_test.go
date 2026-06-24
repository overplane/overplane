package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	oplog "github.com/overplane/overplane/internal/platform/log"
	"github.com/overplane/overplane/internal/platform/serde/yamlcanon"
	"github.com/overplane/overplane/internal/project"
)

// initRun dispatches init with optional extra flags against --dir dir.
func initRun(t *testing.T, dir string, extra ...string) (code int, stdout, stderr, logs string) {
	t.Helper()
	args := append([]string{"init", "--dir", dir}, extra...)
	var out, errb, logb bytes.Buffer
	logger, err := oplog.New(oplog.FormatJSON, "debug", &logb, false)
	if err != nil {
		t.Fatal(err)
	}
	ctx := oplog.WithContext(context.Background(), logger)
	r := &Runner{Out: &out, Err: &errb}
	code = ExitCode(Dispatch(ctx, r, args))
	return code, out.String(), errb.String(), logb.String()
}

func warnCount(logs string) int {
	return strings.Count(logs, `"level":"WARN"`)
}

// assertRelativeLogPaths checks that artifact paths in init logs are rendered
// relative to the init working dir, not absolute.
func assertRelativeLogPaths(t *testing.T, logs string) {
	t.Helper()
	for _, rel := range []string{
		`"path":"overplane.yaml"`,
	} {
		if !strings.Contains(logs, rel) {
			t.Fatalf("logs missing dir-relative path %s: %s", rel, logs)
		}
	}
}

// documentedInitYAML is the byte-exact overplane.yaml init writes for a project
// with the given directory basename and optional field overrides.
func documentedInitYAML(t *testing.T, dirBase string, overrides initOverrides) string {
	t.Helper()
	cfg, err := project.Default(dirBase)
	if err != nil {
		t.Fatal(err)
	}
	applyInitOverrides(cfg, overrides)
	opts, err := project.YAMLDocumentOptions()
	if err != nil {
		t.Fatal(err)
	}
	data, err := yamlcanon.MarshalPlainDocumentedWithBanner(configBanner, cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestInitFreshRun(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	dir := filepath.Join(t.TempDir(), "My Project")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr, logs := initRun(t, dir)
	if code != 0 {
		t.Fatalf("init exit = %d stderr=%s logs=%s", code, stderr, logs)
	}

	cfgData, err := os.ReadFile(filepath.Join(dir, project.FileName))
	if err != nil {
		t.Fatal(err)
	}
	want := documentedInitYAML(t, "My Project", initOverrides{})
	if string(cfgData) != want {
		t.Fatalf("overplane.yaml mismatch:\n got: %q\nwant: %q", cfgData, want)
	}
	if got := warnCount(logs); got != 1 {
		t.Fatalf("WARN count = %d (want 1: config): %s", got, logs)
	}
	assertRelativeLogPaths(t, logs)
	if !strings.Contains(strings.ToUpper(stdout), "NEXT STEP") || !strings.Contains(stdout, "overplane setup") {
		t.Fatalf("missing next-steps table: %s", stdout)
	}
	if strings.Contains(stdout, "001-bootstrap.md") {
		t.Fatalf("next steps must not mention seeded spec: %s", stdout)
	}
}

func TestInitOverrideFlags(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	dir := t.TempDir()
	overrides := initOverrides{
		name:        "custom-app",
		description: "A custom project.",
		cacheDir:    ".cache/custom",
		specsDir:    "specs",
	}
	code, _, stderr, _ := initRun(t, dir,
		"--name", overrides.name,
		"--description", overrides.description,
		"--cache_dir", overrides.cacheDir,
		"--specs_dir", overrides.specsDir,
	)
	if code != 0 {
		t.Fatalf("init exit = %d stderr=%s", code, stderr)
	}
	cfgData, err := os.ReadFile(filepath.Join(dir, project.FileName))
	if err != nil {
		t.Fatal(err)
	}
	want := documentedInitYAML(t, filepath.Base(dir), overrides)
	if string(cfgData) != want {
		t.Fatalf("overplane.yaml mismatch:\n got: %q\nwant: %q", cfgData, want)
	}
}

func TestInitOverridesIgnoredWhenPresent(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	dir := t.TempDir()
	if code, _, stderr, _ := initRun(t, dir); code != 0 {
		t.Fatalf("init failed: %s", stderr)
	}
	before, err := os.ReadFile(filepath.Join(dir, project.FileName))
	if err != nil {
		t.Fatal(err)
	}
	code, _, stderr, _ := initRun(t, dir,
		"--name", "ignored",
		"--description", "ignored",
		"--cache_dir", "ignored",
		"--specs_dir", "ignored",
	)
	if code != 0 {
		t.Fatalf("second init exit = %d stderr=%s", code, stderr)
	}
	after, err := os.ReadFile(filepath.Join(dir, project.FileName))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("overplane.yaml changed on re-run with override flags:\nbefore: %q\nafter: %q", before, after)
	}
}

func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			files = append(files, p)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, p := range files {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		st, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		out[p] = hex.EncodeToString(sum[:]) + "|" + st.ModTime().String()
	}
	return out
}

func TestInitIdempotent(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	dir := t.TempDir()
	if code, _, stderr, _ := initRun(t, dir); code != 0 {
		t.Fatalf("first init failed: %s", stderr)
	}
	before := snapshotTree(t, dir)

	code, stdout, stderr, logs := initRun(t, dir)
	if code != 0 {
		t.Fatalf("second init exit = %d stderr=%s", code, stderr)
	}
	if got := warnCount(logs); got != 0 {
		t.Fatalf("second run emitted %d WARN logs: %s", got, logs)
	}
	if !strings.Contains(logs, "already initialized") {
		t.Fatalf("missing idempotent summary: %s", logs)
	}
	if !strings.Contains(logs, `"action":"skipped"`) {
		t.Fatalf("missing DEBUG skip trail: %s", logs)
	}
	if !strings.Contains(strings.ToUpper(stdout), "NEXT STEP") {
		t.Fatalf("next-steps table missing on re-run: %s", stdout)
	}
	if after := snapshotTree(t, dir); len(after) != len(before) {
		t.Fatalf("file count changed: %d -> %d", len(before), len(after))
	} else {
		for p, sig := range before {
			if after[p] != sig {
				t.Fatalf("file changed on re-run: %s", p)
			}
		}
	}
}

func TestInitNonDestructive(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	dir := t.TempDir()
	custom := `dirs:
  cache: .cache/custom
  specs: specs
project:
  description: "Hand-written."
  name: custom
schema_version: 1
`
	writeTestFile(t, filepath.Join(dir, project.FileName), custom)
	writeTestFile(t, filepath.Join(dir, ".gitignore"), "node_modules/\n.cache/\n")

	code, _, stderr, logs := initRun(t, dir)
	if code != 0 {
		t.Fatalf("init exit = %d stderr=%s", code, stderr)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, project.FileName)); string(data) != custom {
		t.Fatalf("overplane.yaml rewritten: %q", data)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, ".gitignore")); string(data) != "node_modules/\n.cache/\n" {
		t.Fatalf(".gitignore rewritten: %q", data)
	}
	if got := warnCount(logs); got != 0 {
		t.Fatalf("WARN count = %d, want 0: %s", got, logs)
	}
}

func TestInitGitignoreRemovesOverplaneYAML(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, ".gitignore"), "dist/\noverplane.yaml\n")
	if code, _, stderr, _ := initRun(t, dir); code != 0 {
		t.Fatalf("init failed: %s", stderr)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil || string(data) != "dist/\n" {
		t.Fatalf(".gitignore = %q, %v", data, err)
	}
}

func TestInitValidationFailure(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, project.FileName), `dirs:
  cache: /abs/path
  specs: .overplane/specs
project:
  name: demo
schema_version: 1
bogus: true
`)
	code, _, stderr, _ := initRun(t, dir)
	if code != 3 {
		t.Fatalf("exit = %d, want 3; stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, "/dirs/cache") && !strings.Contains(stderr, "/:") {
		t.Fatalf("stderr missing pointer-addressed problems: %s", stderr)
	}
}

func TestInitDirFlag(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Run("missing dir is an error", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "missing", "nested")
		code, _, _, _ := initRun(t, dir)
		if code != 4 {
			t.Fatalf("exit = %d, want 4", code)
		}
	})
	t.Run("dir is a file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "afile")
		writeTestFile(t, path, "x")
		code, _, _, _ := initRun(t, path)
		if code != 4 {
			t.Fatalf("exit = %d, want 4", code)
		}
	})
	t.Run("stray args", func(t *testing.T) {
		var out, errb bytes.Buffer
		r := &Runner{Out: &out, Err: &errb}
		if code := ExitCode(Dispatch(context.Background(), r, []string{"init", "extra"})); code != 2 {
			t.Fatalf("exit = %d, want 2", code)
		}
	})
	t.Run("help", func(t *testing.T) {
		var out, errb bytes.Buffer
		r := &Runner{Out: &out, Err: &errb}
		if err := Dispatch(context.Background(), r, []string{"init", "--help"}); err != nil ||
			!strings.Contains(out.String(), "--dir") || !strings.Contains(out.String(), "--name") {
			t.Fatalf("init help: %v %s", err, out.String())
		}
	})
}

func TestConfigValidateOverplaneYAML(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	dir := t.TempDir()
	if code, _, stderr, _ := initRun(t, dir); code != 0 {
		t.Fatalf("init failed: %s", stderr)
	}
	path := filepath.Join(dir, project.FileName)
	var out, errb bytes.Buffer
	r := &Runner{Out: &out, Err: &errb}
	if err := Dispatch(context.Background(), r, []string{"config", "validate", path}); err != nil {
		t.Fatalf("config validate on generated file: %v stderr=%s", err, errb.String())
	}
	if !strings.Contains(out.String(), "valid") {
		t.Fatalf("missing valid confirmation: %s", out.String())
	}

	bad := filepath.Join(t.TempDir(), project.FileName)
	writeTestFile(t, bad, "dirs: {cache: /abs, specs: .overplane/specs}\nproject: {name: demo}\nschema_version: 1\n")
	out.Reset()
	errb.Reset()
	if code := ExitCode(Dispatch(context.Background(), r, []string{"config", "validate", bad})); code != 3 {
		t.Fatalf("exit = %d, want 3; stderr=%s", code, errb.String())
	}
	if !strings.Contains(errb.String(), "/dirs/cache") {
		t.Fatalf("stderr missing pointer: %s", errb.String())
	}
}

// initShims installs hermetic engine/git shims as the entire PATH and returns
// the shim dir. Each named shim simply succeeds and echoes a version string;
// the git shim answers the rev-parse forms the checks use.
func initShims(t *testing.T, names ...string) string {
	t.Helper()
	bin := t.TempDir()
	for _, name := range names {
		script := "#!/bin/sh\necho '" + name + " version 1.0.0'\n"
		if name == "git" {
			script = `#!/bin/sh
if [ "$1" = rev-parse ] && [ "$2" = --is-inside-work-tree ]; then
  echo true
elif [ "$1" = rev-parse ] && [ "$2" = --abbrev-ref ]; then
  echo main
elif [ "$1" = rev-parse ] && [ "$2" = --short ]; then
  echo abc1234
else
  echo ok
fi
`
		}
		writeTestFile(t, filepath.Join(bin, name), script)
	}
	t.Setenv("PATH", bin)
	return bin
}

func TestCheckAggregation(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	run := func(t *testing.T) int {
		t.Helper()
		var out, errb bytes.Buffer
		r := &Runner{Out: &out, Err: &errb}
		return ExitCode(Dispatch(context.Background(), r, []string{"check"}))
	}
	t.Run("one engine passes", func(t *testing.T) {
		initShims(t, "docker", "git")
		for _, key := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY"} {
			t.Setenv(key, "")
			os.Unsetenv(key)
		}
		if code := run(t); code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
	})
	t.Run("zero engines fail", func(t *testing.T) {
		initShims(t, "git")
		if code := run(t); code != 6 {
			t.Fatalf("exit = %d, want 6", code)
		}
	})
	t.Run("malformed key fails", func(t *testing.T) {
		initShims(t, "docker", "git")
		t.Setenv("OPENAI_API_KEY", "sk-bad key with spaces")
		if code := run(t); code != 6 {
			t.Fatalf("exit = %d, want 6", code)
		}
	})
}

func TestCheckQuiet(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	run := func(t *testing.T) (int, string, string) {
		t.Helper()
		var out, errb bytes.Buffer
		r := &Runner{Out: &out, Err: &errb}
		code := ExitCode(Dispatch(context.Background(), r, []string{"check", "--quiet"}))
		return code, out.String(), errb.String()
	}
	t.Run("pass is silent", func(t *testing.T) {
		initShims(t, "docker", "git")
		code, stdout, _ := run(t)
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		if stdout != "" {
			t.Fatalf("quiet check produced stdout: %q", stdout)
		}
	})
	t.Run("failure is exit code only", func(t *testing.T) {
		initShims(t, "git")
		code, stdout, _ := run(t)
		if code != 6 {
			t.Fatalf("exit = %d, want 6", code)
		}
		if stdout != "" {
			t.Fatalf("quiet check produced stdout: %q", stdout)
		}
	})
}
