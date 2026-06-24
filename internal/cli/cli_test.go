package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDispatchHelpVersionConfigThemeDemo(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var out, errb bytes.Buffer
	r := &Runner{Out: &out, Err: &errb}
	if code := ExitCode(Dispatch(context.Background(), r, nil)); code != 2 {
		t.Fatalf("no args exit = %d", code)
	}
	if !strings.Contains(out.String(), "\x1b[38;2;") {
		t.Fatalf("no args should print banner to stdout: stdout=%q stderr=%q", out.String(), errb.String())
	}
	out.Reset()
	if err := Dispatch(context.Background(), r, []string{"help"}); err != nil ||
		!strings.Contains(out.String(), "Commands") {
		t.Fatalf("help failed: %v %s", err, out.String())
	}
	out.Reset()
	if err := Dispatch(context.Background(), r, []string{"version"}); err != nil ||
		!strings.Contains(out.String(), "overplane v") {
		t.Fatalf("version failed: %v %s", err, out.String())
	}
	for _, args := range [][]string{{"theme"}, {"theme", "preview"}} {
		out.Reset()
		if err := Dispatch(context.Background(), r, args); err != nil ||
			!strings.Contains(out.String(), "Theme:") {
			t.Fatalf("theme %v failed: %v %s", args, err, out.String())
		}
	}
	if code := ExitCode(Dispatch(context.Background(), r, []string{"demo"})); code != 6 {
		t.Fatalf("demo non-tty exit = %d", code)
	}
}

func TestThemeSetRequiresMonorepo(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	dir := t.TempDir()
	oldwd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldwd) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	r := &Runner{Out: &out, Err: &errb}
	err := Dispatch(context.Background(), r, []string{"theme", "set", "hand-drawn"})
	if ExitCode(err) != 6 {
		t.Fatalf("theme set outside monorepo exit = %d, want 6: %v stderr=%s", ExitCode(err), err, errb.String())
	}
}

func TestConfigValidateAndCheckJSON(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	dir := t.TempDir()
	writeCLIConfigFixture(t, dir)
	oldwd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldwd) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	writeCheckFixture(t, dir)
	var out, errb bytes.Buffer
	r := &Runner{Out: &out, Err: &errb}
	if err := Dispatch(context.Background(), r, []string{"config", "validate"}); err != nil {
		t.Fatalf("config validate: %v stderr=%s", err, errb.String())
	}
	out.Reset()
	if err := Dispatch(context.Background(), r, []string{"check", "--json"}); err != nil {
		t.Fatalf("check: %v stderr=%s out=%s", err, errb.String(), out.String())
	}
	if !strings.Contains(out.String(), `"name"`) || strings.Contains(out.String(), "sk-ant-test") {
		t.Fatalf("bad check json: %s", out.String())
	}
	if strings.Contains(out.String(), `"name": "theme"`) {
		t.Fatalf("check output should not include theme: %s", out.String())
	}
	for _, want := range []string{
		`"detail": "available"`,
		`"hint": "Docker version 26.1.4, build abcdef0123456789 verylong"`,
		`"hint": "podman version 5.1.2"`,
		`"hint": "prefix=sk-an"`,
		`"hint": "prefix=sk-te"`,
		`"hint": "prefix=gemin"`,
		`"hint": "branch=main commit=abc1234"`,
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("check output missing %s: %s", want, out.String())
		}
	}
}

func writeCLIConfigFixture(t *testing.T, dir string) {
	t.Helper()
	writeTestFile(t, filepath.Join(dir, "config", "data", "global.yaml"),
		"project:\n  name: test\n  website: https://example.com\n")
	writeTestFile(t, filepath.Join(dir, "config", "data", "theme.yaml"), validThemeFixture)
	writeTestFile(t, filepath.Join(dir, "config", "data", "infra.yaml"), validInfraFixture)
	for _, name := range []string{"global", "theme", "infra"} {
		writeTestFile(t, filepath.Join(dir, "config", "schema", name+".schema.json"), "{}")
	}
}

const validThemeFixture = `fonts:
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
`

const validInfraFixture = `infra:
  terraform:
    version: latest
    state_bucket: state
    state_bucket_path: state
    state_bucket_region: us-east-2
  corp:
    id: test
    region_short: use2
    ops_name: ops
  github:
    owner: overplane
    repo: overplane-master
  aws:
    region: us-east-2
  domains:
    - name: example.com
      subdomains:
        - name: www
          bucket: site
  web:
    buckets:
      - name: site
        asset_path: assets
        is_spa: false
  environments:
    - name: production
      stage: prod
      bucket: site
`

func writeCheckFixture(t *testing.T, dir string) {
	t.Helper()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(
		t,
		filepath.Join(bin, "docker"),
		"#!/bin/sh\necho 'Docker version 26.1.4, build abcdef0123456789 verylong'\n",
	)
	writeTestFile(t, filepath.Join(bin, "podman"), "#!/bin/sh\necho 'podman version 5.1.2'\n")
	writeTestFile(t, filepath.Join(bin, "git"), `#!/bin/sh
if [ "$1" = rev-parse ] && [ "$2" = --is-inside-work-tree ]; then
  echo true
elif [ "$1" = rev-parse ] && [ "$2" = --abbrev-ref ]; then
  echo main
elif [ "$1" = rev-parse ] && [ "$2" = --short ]; then
  echo abc1234
else
  echo ok
fi
`)
	t.Setenv("PATH", isolatedToolPATH(bin))
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test")
	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("GEMINI_API_KEY", "gemini-test")
}

// isolatedToolPATH exposes only the shim directory plus the system paths
// needed to execute #!/usr/bin/env bash scripts, without a real container CLI.
func isolatedToolPATH(binDir string) string {
	return binDir + string(os.PathListSeparator) + "/usr/bin" + string(os.PathListSeparator) + "/bin"
}

func writeTestFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o755); err != nil {
		t.Fatal(err)
	}
}
