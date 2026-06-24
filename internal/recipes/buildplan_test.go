package recipes_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/overplane/overplane/internal/project"
	"github.com/overplane/overplane/internal/recipes"
)

func testOpts() recipes.BuildPlanOptions {
	return recipes.BuildPlanOptions{
		ProjectName: "my-project",
		UID:         "1000",
		GID:         "1000",
		Username:    "dev",
		Home:        "/home/dev",
		Version:     "1.2.3",
	}
}

func defaultContainer(t *testing.T) project.AgentContainer {
	t.Helper()
	cfg, err := project.Default("my-project")
	if err != nil {
		t.Fatal(err)
	}
	return cfg.Agent.Container
}

func plan(t *testing.T, cfg project.AgentContainer, opts recipes.BuildPlanOptions) *recipes.BuildPlan {
	t.Helper()
	reg, err := recipes.Load()
	if err != nil {
		t.Fatal(err)
	}
	p, err := reg.BuildPlan(cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// TestBuildPlanGoldenDockerfile pins the generated Dockerfile for the §2.1
// default config (golden: testdata/default.dockerfile.golden).
func TestBuildPlanGoldenDockerfile(t *testing.T) {
	golden, err := os.ReadFile(filepath.Join("testdata", "default.dockerfile.golden"))
	if err != nil {
		t.Fatal(err)
	}
	p := plan(t, defaultContainer(t), testOpts())
	if p.Dockerfile != string(golden) {
		t.Fatalf("dockerfile mismatch:\n got:\n%s\nwant:\n%s", p.Dockerfile, golden)
	}
}

func TestBuildPlanContract(t *testing.T) {
	cfg := defaultContainer(t)
	p := plan(t, cfg, testOpts())
	if p.ImageRepo != "overplane-my-project" {
		t.Fatalf("image repo = %q", p.ImageRepo)
	}
	wantTags := []string{"overplane-my-project:latest", "overplane-my-project:b" + p.Hash}
	if len(p.Tags) != 2 || p.Tags[0] != wantTags[0] || p.Tags[1] != wantTags[1] {
		t.Fatalf("tags = %v, want %v", p.Tags, wantTags)
	}
	if len(p.Hash) != 12 {
		t.Fatalf("hash = %q, want 12 hex chars", p.Hash)
	}
	for k, v := range map[string]string{
		"overplane.project":      "my-project",
		"overplane.build.hash":   p.Hash,
		"overplane.build.recipe": "debian",
		"overplane.build.base":   "debian:bookworm-slim",
		"overplane.version":      "1.2.3",
	} {
		if p.Labels[k] != v {
			t.Fatalf("label %s = %q, want %q", k, p.Labels[k], v)
		}
	}
	for k, v := range map[string]string{
		"BASE_REF":            "debian:bookworm-slim",
		"SOURCE_DATE_EPOCH":   "0",
		"EXTRA_OS_PACKAGES":   "",
		"OVERPLANE_UID":       "1000",
		"OVERPLANE_GID":       "1000",
		"OVERPLANE_USER":      "dev",
		"OVERPLANE_HOME":      "/home/dev",
		"OVERPLANE_PROJECT":   "my-project",
		"CODEX_VERSION":       "latest",
		"CLAUDE_CODE_VERSION": "latest",
		"GEMINI_CLI_VERSION":  "latest",
	} {
		if p.BuildArgs[k] != v {
			t.Fatalf("build arg %s = %q, want %q", k, p.BuildArgs[k], v)
		}
	}
	if p.Platform != "linux/"+runtime.GOARCH {
		t.Fatalf("platform = %q", p.Platform)
	}
}

// TestBuildPlanHashProperties asserts the hash is stable across calls and
// changes on every meaningful input change.
func TestBuildPlanHashProperties(t *testing.T) {
	opts := testOpts()
	base := plan(t, defaultContainer(t), opts).Hash
	if again := plan(t, defaultContainer(t), opts).Hash; again != base {
		t.Fatalf("hash unstable: %s vs %s", base, again)
	}
	mutations := map[string]func(*project.AgentContainer, *recipes.BuildPlanOptions){
		"base image": func(c *project.AgentContainer, _ *recipes.BuildPlanOptions) {
			c.BaseImage = "debian:trixie-slim"
		},
		"agent version": func(c *project.AgentContainer, _ *recipes.BuildPlanOptions) {
			c.AgentRecipes[0].Version = "2.1.3"
		},
		"extra package": func(c *project.AgentContainer, _ *recipes.BuildPlanOptions) {
			c.ExtraPackages["debian"] = []string{"graphicsmagick"}
		},
		"setup recipe": func(c *project.AgentContainer, _ *recipes.BuildPlanOptions) {
			c.SetupRecipe = "alpine"
			c.BaseImage = "alpine:3"
		},
		"agent removed": func(c *project.AgentContainer, _ *recipes.BuildPlanOptions) {
			c.AgentRecipes = c.AgentRecipes[:2]
		},
		"host uid": func(_ *project.AgentContainer, o *recipes.BuildPlanOptions) {
			o.UID = "1001"
		},
		"project name": func(_ *project.AgentContainer, o *recipes.BuildPlanOptions) {
			o.ProjectName = "other"
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			cfg := defaultContainer(t)
			o := testOpts()
			mutate(&cfg, &o)
			if got := plan(t, cfg, o).Hash; got == base {
				t.Fatalf("hash did not change for mutation %q", name)
			}
		})
	}
	// The CLI version is a label, not a hash input: upgrading overplane must
	// not invalidate cached images by itself.
	o := testOpts()
	o.Version = "9.9.9"
	if got := plan(t, defaultContainer(t), o).Hash; got != base {
		t.Fatalf("hash must not depend on CLI version")
	}
}

// TestBuildPlanAgentOrder asserts agent layers follow overplane.yaml file
// order and that the extras layer is isolated from the toolchain layer.
func TestBuildPlanAgentOrder(t *testing.T) {
	cfg := defaultContainer(t)
	cfg.AgentRecipes = []project.AgentRecipeSelection{
		{Name: "gemini-cli", Version: "latest"},
		{Name: "codex", Version: "latest"},
	}
	p := plan(t, cfg, testOpts())
	fragNames := make([]string, 0, len(p.Fragments))
	for _, f := range p.Fragments {
		fragNames = append(fragNames, f.Name)
	}
	want := []string{
		"debian-base-setup", "debian-install-deps", "debian-extra-packages",
		"agent-gemini-cli", "agent-codex", "linux-setup-user",
	}
	if strings.Join(fragNames, ",") != strings.Join(want, ",") {
		t.Fatalf("fragment order = %v, want %v", fragNames, want)
	}
	// Extras must be their own layer: the toolchain RUN line must not consume
	// EXTRA_OS_PACKAGES.
	for _, line := range strings.Split(p.Dockerfile, "\n") {
		if strings.Contains(line, "debian-install-deps.sh") && strings.Contains(line, "EXTRA_OS_PACKAGES") {
			t.Fatalf("toolchain layer references EXTRA_OS_PACKAGES: %s", line)
		}
	}
}

func TestBuildPlanMaterialize(t *testing.T) {
	p := plan(t, defaultContainer(t), testOpts())
	dir := t.TempDir()
	dfPath, err := p.Materialize(dir)
	if err != nil {
		t.Fatal(err)
	}
	if dfPath != filepath.Join(dir, "Dockerfile") {
		t.Fatalf("dockerfile path = %q", dfPath)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	// 7 fragments (3 setup + 3 agents + user) + profile + entrypoint + Dockerfile.
	if len(entries) != 10 {
		t.Fatalf("context entries = %d: %v", len(entries), entries)
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			t.Fatal(err)
		}
		if !info.ModTime().Equal(timeUnix0()) {
			t.Fatalf("%s mtime not pinned: %v", e.Name(), info.ModTime())
		}
	}
	df, err := os.ReadFile(dfPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(df) != p.Dockerfile {
		t.Fatal("materialized Dockerfile differs from plan")
	}
}

// TestBuildPlanAlpineBootstrap asserts the alpine recipe bootstraps bash
// before any fragment runs.
func TestBuildPlanAlpineBootstrap(t *testing.T) {
	cfg := defaultContainer(t)
	cfg.SetupRecipe = "alpine"
	cfg.BaseImage = "alpine:3"
	p := plan(t, cfg, testOpts())
	bootstrap := strings.Index(p.Dockerfile, "apk add --no-cache bash")
	firstFragment := strings.Index(p.Dockerfile, "COPY alpine-base-setup.sh")
	if bootstrap < 0 || firstFragment < 0 || bootstrap > firstFragment {
		t.Fatalf("alpine bash bootstrap missing or misplaced:\n%s", p.Dockerfile)
	}
}

func timeUnix0() (t0 time.Time) {
	return time.Unix(0, 0)
}
