package project_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/overplane/overplane/internal/platform/configloader"
	assetfs "github.com/overplane/overplane/internal/platform/embed/assets"
	"github.com/overplane/overplane/internal/platform/paths"
	"github.com/overplane/overplane/internal/platform/serde/yamlcanon"
	"github.com/overplane/overplane/internal/project"
)

func TestDefaultValidatesAgainstSchema(t *testing.T) {
	cfg, err := project.Default("My Project!!")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Project.Name != "my-project" {
		t.Fatalf("sanitized name = %q", cfg.Project.Name)
	}
	if cfg.Dirs.Specs != ".overplane/specs" || cfg.Dirs.Cache != ".cache/overplane" {
		t.Fatalf("default dirs = %+v", cfg.Dirs)
	}
	if cfg.SchemaVersion != 1 {
		t.Fatalf("schema_version = %d", cfg.SchemaVersion)
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	problems, err := configloader.ValidateBytes(data, project.SchemaBytes(), project.FileName)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) > 0 {
		t.Fatalf("default config fails its own schema: %v", problems)
	}
	if more := cfg.Validate(); len(more) > 0 {
		t.Fatalf("default config fails semantic checks: %v", more)
	}
}

func TestSanitizeName(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"My Project!!", "my-project"},
		{"hello_world.v2", "hello_world.v2"},
		{"--weird--", "weird"},
		{"...", ""},
		{"", ""},
		{"CamelCase", "camelcase"},
	} {
		if got := project.SanitizeName(tc.in); got != tc.want {
			t.Errorf("SanitizeName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestLoadHappyPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, project.FileName)
	write(t, path, `dirs:
  cache: .cache/overplane
  specs: .overplane/specs
model:
  default: ""
project:
  description: "A test project."
  name: demo
schema_version: 1
`)
	cfg, err := project.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Project.Name != "demo" || cfg.Project.Description != "A test project." {
		t.Fatalf("loaded config = %+v", cfg)
	}
}

var loadFailureCases = []struct {
	name    string
	yaml    string
	pointer string
}{
	{
		name: "unknown key",
		yaml: `dirs: {cache: .cache/overplane, specs: .overplane/specs}
project: {name: demo}
schema_version: 1
bogus: true
`,
		pointer: "/",
	},
	{
		name: "wrong type",
		yaml: `dirs: {cache: .cache/overplane, specs: .overplane/specs}
project: {name: demo}
schema_version: "one"
`,
		pointer: "/schema_version",
	},
	{
		name: "bad name pattern",
		yaml: `dirs: {cache: .cache/overplane, specs: .overplane/specs}
project: {name: "Bad Name"}
schema_version: 1
`,
		pointer: "/project/name",
	},
	{
		name: "absolute cache dir",
		yaml: `dirs: {cache: /tmp/cache, specs: .overplane/specs}
project: {name: demo}
schema_version: 1
`,
		pointer: "/dirs/cache",
	},
	{
		name: "escaping specs dir",
		yaml: `dirs: {cache: .cache/overplane, specs: ../specs}
project: {name: demo}
schema_version: 1
`,
		pointer: "/dirs/specs",
	},
	{
		name: "identical dirs",
		yaml: `dirs: {cache: .overplane/specs, specs: .overplane/specs}
project: {name: demo}
schema_version: 1
`,
		pointer: "/dirs/cache",
	},
	{
		name: "wrong schema version",
		yaml: `dirs: {cache: .cache/overplane, specs: .overplane/specs}
project: {name: demo}
schema_version: 2
`,
		pointer: "/schema_version",
	},
}

func TestLoadFailures(t *testing.T) {
	for _, tc := range loadFailureCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, project.FileName)
			write(t, path, tc.yaml)
			_, err := project.Load(path)
			if err == nil {
				t.Fatal("expected validation failure")
			}
			var ve configloader.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("expected ValidationError, got %T: %v", err, err)
			}
			found := false
			for _, p := range ve.Problems {
				if p.Pointer == tc.pointer {
					found = true
				}
			}
			if !found {
				t.Fatalf("no problem at pointer %q: %+v", tc.pointer, ve.Problems)
			}
		})
	}
}

const validAgentBlock = `agent:
  container:
    agent_recipes:
      - name: codex
        version: latest
    base_image: debian:bookworm-slim
    env_passthrough: [EDITOR]
    extra_packages:
      debian: [graphicsmagick]
    runtime: docker
    setup_recipe: debian
`

const agentBaseYAML = "dirs: {cache: .cache/overplane, specs: .overplane/specs}\n" +
	"project: {name: demo}\nschema_version: 1\n"

// agentFailureCases are #0003 agent-section documents that must be rejected
// at the given JSON Pointer.
var agentFailureCases = []struct {
	name    string
	yaml    string
	pointer string
}{
	{
		name: "bad runtime enum",
		yaml: agentBaseYAML + `agent:
  container:
    agent_recipes: []
    base_image: debian:bookworm-slim
    runtime: lxc
    setup_recipe: debian
`,
		pointer: "/agent/container/runtime",
	},
	{
		name: "bad setup recipe enum",
		yaml: agentBaseYAML + `agent:
  container:
    agent_recipes: []
    base_image: debian:bookworm-slim
    runtime: docker
    setup_recipe: gentoo
`,
		pointer: "/agent/container/setup_recipe",
	},
	{
		name: "bad agent name enum",
		yaml: agentBaseYAML + `agent:
  container:
    agent_recipes:
      - name: clippy
    base_image: debian:bookworm-slim
    runtime: docker
    setup_recipe: debian
`,
		pointer: "/agent/container/agent_recipes/0/name",
	},
	{
		name: "bad package name",
		yaml: agentBaseYAML + `agent:
  container:
    agent_recipes: []
    base_image: debian:bookworm-slim
    extra_packages:
      debian: ["Bad Package"]
    runtime: docker
    setup_recipe: debian
`,
		pointer: "/agent/container/extra_packages/debian/0",
	},
	{
		name: "bad env var name",
		yaml: agentBaseYAML + `agent:
  container:
    agent_recipes: []
    base_image: debian:bookworm-slim
    env_passthrough: [lower_case]
    runtime: docker
    setup_recipe: debian
`,
		pointer: "/agent/container/env_passthrough/0",
	},
	{
		name: "unknown extra_packages family",
		yaml: agentBaseYAML + `agent:
  container:
    agent_recipes: []
    base_image: debian:bookworm-slim
    extra_packages:
      gentoo: [foo]
    runtime: docker
    setup_recipe: debian
`,
		pointer: "/agent/container/extra_packages",
	},
	{
		name: "duplicate agent recipe names",
		yaml: agentBaseYAML + `agent:
  container:
    agent_recipes:
      - name: codex
      - name: codex
    base_image: debian:bookworm-slim
    runtime: docker
    setup_recipe: debian
`,
		pointer: "/agent/container/agent_recipes/1/name",
	},
	{
		name: "missing required container fields",
		yaml: agentBaseYAML + `agent:
  container:
    runtime: docker
`,
		pointer: "/agent/container",
	},
}

// TestLoadAgentSection covers the #0003 schema extension: a fully spelled-out
// agent section loads, and bad enum values, package names, env var names, and
// semantic duplicates are rejected at exact JSON Pointers.
func TestLoadAgentSection(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, project.FileName)
		write(t, path, agentBaseYAML+validAgentBlock)
		cfg, err := project.Load(path)
		if err != nil {
			t.Fatal(err)
		}
		ac := cfg.Agent.Container
		if ac.Runtime != "docker" || ac.SetupRecipe != "debian" || len(ac.AgentRecipes) != 1 {
			t.Fatalf("agent section = %+v", ac)
		}
		if ac.ExtraPackages["debian"][0] != "graphicsmagick" || ac.EnvPassthrough[0] != "EDITOR" {
			t.Fatalf("agent section = %+v", ac)
		}
	})
	for _, tc := range agentFailureCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, project.FileName)
			write(t, path, tc.yaml)
			_, err := project.Load(path)
			if err == nil {
				t.Fatal("expected validation failure")
			}
			var ve configloader.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("expected ValidationError, got %T: %v", err, err)
			}
			for _, p := range ve.Problems {
				if p.Pointer == tc.pointer {
					return
				}
			}
			t.Fatalf("no problem at pointer %q: %+v", tc.pointer, ve.Problems)
		})
	}
}

// TestLoadAgentDefaults asserts a #0002-era file without the agent key (and a
// partially specified agent section) normalizes in memory to the documented
// defaults without touching the file.
func TestLoadAgentDefaults(t *testing.T) {
	t.Run("absent agent block", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, project.FileName)
		original := `dirs:
  cache: .cache/overplane
  specs: .overplane/specs
model:
  default: ""
project:
  description: ""
  name: demo
schema_version: 1
`
		write(t, path, original)
		cfg, err := project.Load(path)
		if err != nil {
			t.Fatal(err)
		}
		def, err := project.Default("")
		if err != nil {
			t.Fatal(err)
		}
		got, _ := json.Marshal(cfg.Agent)
		want, _ := json.Marshal(def.Agent)
		if !bytes.Equal(got, want) {
			t.Fatalf("normalized agent = %s, want %s", got, want)
		}
		if data, _ := os.ReadFile(path); string(data) != original {
			t.Fatalf("file rewritten: %q", data)
		}
	})
	t.Run("optional leaves filled", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, project.FileName)
		write(t, path, `dirs: {cache: .cache/overplane, specs: .overplane/specs}
project: {name: demo}
schema_version: 1
agent:
  container:
    agent_recipes:
      - name: claude-code
    base_image: debian:bookworm-slim
    runtime: docker
    setup_recipe: debian
`)
		cfg, err := project.Load(path)
		if err != nil {
			t.Fatal(err)
		}
		ac := cfg.Agent.Container
		if ac.AgentRecipes[0].Version != "latest" {
			t.Fatalf("version default = %q", ac.AgentRecipes[0].Version)
		}
		if ac.EnvPassthrough == nil || len(ac.EnvPassthrough) != 0 {
			t.Fatalf("env_passthrough default = %#v", ac.EnvPassthrough)
		}
		if pkgs, ok := ac.ExtraPackages["debian"]; !ok || len(pkgs) != 0 {
			t.Fatalf("extra_packages default = %#v", ac.ExtraPackages)
		}
	})
}

func TestLocate(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, ok := project.Locate(nested); ok {
		t.Fatal("Locate found a project file in an empty tree")
	}
	write(t, filepath.Join(root, project.FileName), "schema_version: 1\n")
	p, ok := project.Locate(nested)
	if !ok || p != filepath.Join(root, project.FileName) {
		t.Fatalf("Locate = %q, %v", p, ok)
	}
}

// TestSchemaDrift asserts the embedded schemas are byte-identical to the
// canonical monorepo copies under config/schema. In a standalone (public
// repo) checkout there is no monorepo root, so the test skips.
func TestSchemaDrift(t *testing.T) {
	p, err := paths.Resolve("")
	if err != nil {
		t.Skipf("no monorepo root (standalone checkout): %v", err)
	}
	for _, name := range []string{"overplane.schema.json", "spec-metadata.schema.json"} {
		canonical, err := os.ReadFile(filepath.Join(p.ConfigSchemaDir, name))
		if err != nil {
			t.Fatalf("read canonical %s: %v", name, err)
		}
		embedded, err := assetfs.ReadFile("files/schema/" + name)
		if err != nil {
			t.Fatalf("read embedded %s: %v", name, err)
		}
		if !bytes.Equal(canonical, embedded) {
			t.Fatalf("embedded %s drifted from config/schema copy; re-copy and run go generate ./...", name)
		}
	}
}

func TestDocumentedInitYAML(t *testing.T) {
	cfg, err := project.Default("demo")
	if err != nil {
		t.Fatal(err)
	}
	opts, err := project.YAMLDocumentOptions()
	if err != nil {
		t.Fatal(err)
	}
	data, err := yamlcanon.MarshalPlainDocumented(cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"# Version of the overplane.yaml file format.",
		"schema_version: 1",
		"\n\n# Identity of the project",
		"name: demo",
		"\n\n# Filesystem layout",
		"specs: .overplane/specs",
		"\n\n# Configuration of the project's agent container",
		"runtime: docker",
		"-\n\n        # Registry name of the agent recipe",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("documented yaml missing %q:\n%s", want, text)
		}
	}
	dir := t.TempDir()
	path := filepath.Join(dir, project.FileName)
	write(t, path, text)
	if _, err := project.Load(path); err != nil {
		t.Fatalf("documented yaml must validate: %v", err)
	}
	_ = cfg
}

func write(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}
