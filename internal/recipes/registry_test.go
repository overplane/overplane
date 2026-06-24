package recipes_test

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/overplane/overplane/internal/container"
	assetfs "github.com/overplane/overplane/internal/platform/embed/assets"
	"github.com/overplane/overplane/internal/platform/paths"
	"github.com/overplane/overplane/internal/project"
	"github.com/overplane/overplane/internal/recipes"
)

// TestRuntimePairsMatchContainerMatrix keeps the registry's runtime_pairs in
// lockstep with container.ValidatePair: every registry pair must be allowed
// by the matrix and every allowed matrix pair must be declared.
func TestRuntimePairsMatchContainerMatrix(t *testing.T) {
	reg, err := recipes.Load()
	if err != nil {
		t.Fatal(err)
	}
	declared := map[[2]container.Engine]bool{}
	for _, pair := range reg.RuntimePairs {
		b, err := container.ParseEngine(pair.Build)
		if err != nil {
			t.Fatalf("registry build engine %q: %v", pair.Build, err)
		}
		r, err := container.ParseEngine(pair.Run)
		if err != nil {
			t.Fatalf("registry run engine %q: %v", pair.Run, err)
		}
		declared[[2]container.Engine{b, r}] = true
		if err := container.ValidatePair(b, r); err != nil {
			t.Errorf("registry pair %s/%s rejected by container matrix: %v", pair.Build, pair.Run, err)
		}
	}
	engines := []container.Engine{
		container.EngineDocker, container.EnginePodman, container.EngineNerdctl, container.EngineK3s,
	}
	for _, b := range engines {
		for _, r := range engines {
			if container.ValidatePair(b, r) == nil && !declared[[2]container.Engine{b, r}] {
				t.Errorf("matrix allows %s/%s but the registry does not declare it", b, r)
			}
		}
	}
}

// TestLoadEmbeddedRegistry exercises the real embedded registry through the
// schema pass and the structural self-check, so a bad registry can never
// ship.
func TestLoadEmbeddedRegistry(t *testing.T) {
	reg, err := recipes.Load()
	if err != nil {
		t.Fatalf("embedded registry invalid: %v", err)
	}
	if reg.SchemaVersion != 1 {
		t.Fatalf("schema_version = %d", reg.SchemaVersion)
	}
	for _, name := range []string{"docker", "podman", "nerdctl", "k3s"} {
		if _, ok := reg.Runtime(name); !ok {
			t.Fatalf("missing runtime %q", name)
		}
	}
	for _, name := range []string{"debian", "alpine"} {
		if _, ok := reg.SetupRecipe(name); !ok {
			t.Fatalf("missing setup recipe %q", name)
		}
	}
	for _, name := range []string{"codex", "claude-code", "gemini-cli"} {
		if _, ok := reg.AgentRecipe(name); !ok {
			t.Fatalf("missing agent recipe %q", name)
		}
	}
	if got := reg.PackageFamilies(); !reflect.DeepEqual(got, []string{"alpine", "debian"}) {
		t.Fatalf("package families = %v", got)
	}
}

// TestValidateRegistryBytes asserts corrupted registries fail with
// pointer-accurate problems for both the schema pass and the structural
// self-check.
func TestValidateRegistryBytes(t *testing.T) {
	valid := string(recipes.RegistryBytes())
	cases := []struct {
		name    string
		mutate  func(string) string
		pointer string
	}{
		{
			name:    "unknown top-level key",
			mutate:  func(s string) string { return s + "\nbogus: true\n" },
			pointer: "/",
		},
		{
			name:    "bad status enum",
			mutate:  func(s string) string { return strings.Replace(s, "status: supported", "status: beta", 1) },
			pointer: "/container_runtimes/0/status",
		},
		{
			name:    "missing fragment asset",
			mutate:  func(s string) string { return strings.Replace(s, "- debian-base-setup", "- no-such-fragment", 1) },
			pointer: "/setup_recipes/0/fragments/0",
		},
		{
			name: "pair references runtime without capability",
			mutate: func(s string) string {
				return strings.Replace(s, "- {build: docker, run: docker}", "- {build: k3s, run: docker}", 1)
			},
			pointer: "/runtime_pairs/0/build",
		},
		{
			name: "unknown supported setup recipe",
			mutate: func(s string) string {
				return strings.Replace(s, "supported_setup_recipes: [debian, alpine]", "supported_setup_recipes: [gentoo]", 1)
			},
			pointer: "/agent_recipes/0/supported_setup_recipes/0",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			problems, err := recipes.ValidateRegistryBytes([]byte(tc.mutate(valid)))
			if err != nil {
				t.Fatal(err)
			}
			for _, p := range problems {
				if p.Pointer == tc.pointer {
					return
				}
			}
			t.Fatalf("no problem at pointer %q: %+v", tc.pointer, problems)
		})
	}
	if problems, err := recipes.ValidateRegistryBytes([]byte(valid)); err != nil || len(problems) > 0 {
		t.Fatalf("shipped registry should validate: %v %+v", err, problems)
	}
}

func TestEnvPassthroughUnion(t *testing.T) {
	reg, err := recipes.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg := project.AgentContainer{
		AgentRecipes: []project.AgentRecipeSelection{
			{Name: "gemini-cli", Version: "latest"},
			{Name: "codex", Version: "latest"},
		},
		EnvPassthrough: []string{"OPENAI_API_KEY", "EDITOR", "NO_COLOR"},
	}
	got := reg.EnvPassthrough(cfg)
	want := []string{"EDITOR", "GEMINI_API_KEY", "NO_COLOR", "OPENAI_API_KEY"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("EnvPassthrough = %v, want %v", got, want)
	}
	if got := reg.EnvPassthrough(project.AgentContainer{}); len(got) != 0 {
		t.Fatalf("empty config should pass nothing: %v", got)
	}
}

// projectConfigCases are registry-aware validation failures: each mutation of
// the default config must produce a problem at the given JSON Pointer.
var projectConfigCases = []struct {
	name    string
	mutate  func(*project.Config)
	pointer string
}{
	{
		name:    "planned runtime rejected",
		mutate:  func(c *project.Config) { c.Agent.Container.Runtime = "k3s" },
		pointer: "/agent/container/runtime",
	},
	{
		name:    "unknown runtime rejected",
		mutate:  func(c *project.Config) { c.Agent.Container.Runtime = "lxc" },
		pointer: "/agent/container/runtime",
	},
	{
		name:    "unknown setup recipe",
		mutate:  func(c *project.Config) { c.Agent.Container.SetupRecipe = "gentoo" },
		pointer: "/agent/container/setup_recipe",
	},
	{
		name: "unknown agent recipe",
		mutate: func(c *project.Config) {
			c.Agent.Container.AgentRecipes[1].Name = "clippy"
		},
		pointer: "/agent/container/agent_recipes/1/name",
	},
	{
		name: "unknown package family",
		mutate: func(c *project.Config) {
			c.Agent.Container.ExtraPackages["gentoo"] = []string{"foo"}
		},
		pointer: "/agent/container/extra_packages/gentoo",
	},
}

// TestValidateProjectConfig covers the registry-aware second validation pass:
// unknown agents, unsupported recipe pairings, bad runtimes, and bad package
// families.
func TestValidateProjectConfig(t *testing.T) {
	reg, err := recipes.Load()
	if err != nil {
		t.Fatal(err)
	}
	base := func() *project.Config {
		cfg, err := project.Default("demo")
		if err != nil {
			t.Fatal(err)
		}
		return cfg
	}
	t.Run("default config is clean", func(t *testing.T) {
		if problems := recipes.ValidateProjectConfig(reg, base()); len(problems) > 0 {
			t.Fatalf("default config: %+v", problems)
		}
	})
	t.Run("nil agent section is clean", func(t *testing.T) {
		cfg := base()
		cfg.Agent = nil
		if problems := recipes.ValidateProjectConfig(reg, cfg); len(problems) > 0 {
			t.Fatalf("nil agent: %+v", problems)
		}
	})
	for _, tc := range projectConfigCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base()
			tc.mutate(cfg)
			problems := recipes.ValidateProjectConfig(reg, cfg)
			for _, p := range problems {
				if p.Pointer == tc.pointer {
					return
				}
			}
			t.Fatalf("no problem at pointer %q: %+v", tc.pointer, problems)
		})
	}
}

func TestLoadProjectConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, project.FileName)
	write(t, path, `dirs: {cache: .cache/overplane, specs: .overplane/specs}
project: {name: demo}
schema_version: 1
`)
	cfg, err := recipes.LoadProjectConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Agent == nil || cfg.Agent.Container.Runtime != "docker" {
		t.Fatalf("normalized agent section missing: %+v", cfg.Agent)
	}
}

// TestRegistryDrift asserts the embedded registry and schema are
// byte-identical to the enclosing-repo mirrors; standalone checkouts skip.
func TestRegistryDrift(t *testing.T) {
	p, err := paths.Resolve("")
	if err != nil {
		t.Skipf("no monorepo root (standalone checkout): %v", err)
	}
	for _, pair := range [][2]string{
		{recipes.RegistryAssetKey, filepath.Join(p.ConfigDataDir, "recipes.yaml")},
		{recipes.SchemaAssetKey, filepath.Join(p.ConfigSchemaDir, "recipes.schema.json")},
	} {
		embedded, err := assetfs.ReadFile(pair[0])
		if err != nil {
			t.Fatalf("read embedded %s: %v", pair[0], err)
		}
		canonical, err := os.ReadFile(pair[1])
		if err != nil {
			t.Fatalf("read canonical %s: %v", pair[1], err)
		}
		if !bytes.Equal(embedded, canonical) {
			t.Fatalf("embedded %s drifted from %s; re-copy and run go generate ./...", pair[0], pair[1])
		}
	}
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
