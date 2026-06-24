package recipes

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/overplane/overplane/internal/platform/hashutil"
	"github.com/overplane/overplane/internal/project"
)

// Image-contract constants (spec #0003 §4.4).
const (
	// ImageRepoPrefix prefixes every project image repository:
	// overplane-<project.name>.
	ImageRepoPrefix = "overplane-"
	// LabelProject labels images and containers with the owning project name.
	LabelProject = "overplane.project"
	// LabelBuildHash carries the deterministic build hash; it is the cache key
	// `overplane agent setup` probes before building.
	LabelBuildHash = "overplane.build.hash"
	// LabelBuildRecipe records the setup recipe the image was built with.
	LabelBuildRecipe = "overplane.build.recipe"
	// LabelBuildBase records the base image reference.
	LabelBuildBase = "overplane.build.base"
	// LabelVersion records the overplane CLI version that built the image.
	LabelVersion = "overplane.version"
	// LabelShell marks containers started by `overplane agent shell`.
	LabelShell = "overplane.shell"

	profileFileName    = "bash_profile_extra.sh"
	entrypointFileName = entrypointFragment + ".sh"
	entrypointPath     = "/usr/local/bin/" + entrypointFileName
	fragmentStageDir   = "/tmp/overplane-setup"
)

// BuildPlanOptions carries the host- and invocation-specific inputs to a
// build plan.
type BuildPlanOptions struct {
	// ProjectName is the validated project.name from overplane.yaml.
	ProjectName string
	// UID, GID, Username, Home describe the host identity baked into the
	// container user.
	UID      string
	GID      string
	Username string
	Home     string
	// Version is the overplane CLI version, recorded as an image label (not
	// part of the build hash).
	Version string
	// SourceDateEpoch pins file mtimes and image timestamps for reproducible
	// builds; defaults to "0".
	SourceDateEpoch string
}

// Fragment is an embedded setup script scheduled into the build, in layer
// order, with its bytes.
type Fragment struct {
	// Name is the registry fragment name (file <Name>.sh in the context).
	Name string
	// Args are the Dockerfile-side positional arguments passed to the
	// fragment's RUN invocation (build-arg references like "${OVERPLANE_UID}").
	Args []string
	// PreArgs are ARG declarations emitted immediately before this fragment's
	// layer so value changes invalidate as few layers as possible.
	PreArgs []string
	// Bytes is the embedded script content.
	Bytes []byte
}

// BuildPlan is everything needed to build the project's agent container
// image: the ordered fragments with bytes, the generated Dockerfile, build
// args, labels, tags, and the deterministic build hash (the cache key).
type BuildPlan struct {
	Recipe          SetupRecipe
	BaseImage       string
	Agents          []project.AgentRecipeSelection
	Fragments       []Fragment
	Profile         []byte
	Entrypoint      []byte
	Dockerfile      string
	BuildArgs       map[string]string
	Labels          map[string]string
	ImageRepo       string
	Tags            []string
	Hash            string
	Platform        string
	SourceDateEpoch string
}

// ImageRepo returns the image repository for a project name.
func ImageRepo(projectName string) string { return ImageRepoPrefix + projectName }

// LatestTag returns the stable tag for a project name.
func LatestTag(projectName string) string { return ImageRepo(projectName) + ":latest" }

// BuildPlan assembles the deterministic build plan for cfg (spec #0003 §3.3):
// setup fragments in registry order, one agent install fragment per
// configured recipe in file order, then the user fragment; the generated
// Dockerfile; sorted build args; tags; labels; and the build hash over base
// image, recipe name, fragment names and bytes, Dockerfile text, build args,
// and platform.
func (r *Registry) BuildPlan(cfg project.AgentContainer, opts BuildPlanOptions) (*BuildPlan, error) {
	recipe, ok := r.SetupRecipe(cfg.SetupRecipe)
	if !ok {
		return nil, fmt.Errorf("unknown setup recipe %q", cfg.SetupRecipe)
	}
	if opts.ProjectName == "" {
		return nil, fmt.Errorf("build plan requires a project name")
	}
	if opts.SourceDateEpoch == "" {
		opts.SourceDateEpoch = "0"
	}
	if _, err := strconv.ParseInt(opts.SourceDateEpoch, 10, 64); err != nil {
		return nil, fmt.Errorf("invalid SOURCE_DATE_EPOCH %q: %w", opts.SourceDateEpoch, err)
	}

	plan := &BuildPlan{
		Recipe:          recipe,
		BaseImage:       cfg.BaseImage,
		Agents:          append([]project.AgentRecipeSelection(nil), cfg.AgentRecipes...),
		ImageRepo:       ImageRepo(opts.ProjectName),
		Platform:        "linux/" + runtime.GOARCH,
		SourceDateEpoch: opts.SourceDateEpoch,
	}
	if err := plan.collectFragments(r, cfg); err != nil {
		return nil, err
	}
	var err error
	if plan.Profile, err = readProfile(); err != nil {
		return nil, err
	}
	if plan.Entrypoint, err = fragmentBytes(entrypointFragment); err != nil {
		return nil, fmt.Errorf("embedded entrypoint: %w", err)
	}
	plan.Dockerfile = plan.renderDockerfile()
	plan.BuildArgs = plan.buildArgs(cfg, recipe, opts)
	plan.Hash = plan.computeHash()
	plan.Labels = map[string]string{
		LabelProject:     opts.ProjectName,
		LabelBuildHash:   plan.Hash,
		LabelBuildRecipe: recipe.Name,
		LabelBuildBase:   cfg.BaseImage,
		LabelVersion:     opts.Version,
	}
	plan.Tags = []string{LatestTag(opts.ProjectName), plan.ImageRepo + ":b" + plan.Hash}
	return plan, nil
}

func (p *BuildPlan) collectFragments(r *Registry, cfg project.AgentContainer) error {
	appendFragment := func(name string, args, preArgs []string) error {
		data, err := fragmentBytes(name)
		if err != nil {
			return fmt.Errorf("embedded fragment %s: %w", name, err)
		}
		p.Fragments = append(p.Fragments, Fragment{Name: name, Args: args, PreArgs: preArgs, Bytes: data})
		return nil
	}
	for _, name := range p.Recipe.Fragments {
		var args, preArgs []string
		switch {
		case strings.HasSuffix(name, "-install-deps"):
			preArgs = []string{"ARG GO_VERSION", "ARG NODE_MAJOR", "ARG RUST_MIN_VERSION"}
		case strings.HasSuffix(name, "-extra-packages"):
			args = []string{`"${EXTRA_OS_PACKAGES}"`}
			preArgs = []string{`ARG EXTRA_OS_PACKAGES=""`}
		}
		if err := appendFragment(name, args, preArgs); err != nil {
			return err
		}
	}
	for _, sel := range cfg.AgentRecipes {
		recipe, ok := r.AgentRecipe(sel.Name)
		if !ok {
			return fmt.Errorf("unknown agent recipe %q", sel.Name)
		}
		arg := AgentVersionArg(sel.Name)
		if err := appendFragment(recipe.InstallFragment,
			[]string{`"${` + arg + `}"`}, []string{"ARG " + arg}); err != nil {
			return err
		}
	}
	return appendFragment(p.Recipe.UserFragment,
		[]string{`"${OVERPLANE_UID}"`, `"${OVERPLANE_GID}"`, `"${OVERPLANE_USER}"`, `"${OVERPLANE_HOME}"`},
		[]string{"ARG OVERPLANE_UID", "ARG OVERPLANE_GID", "ARG OVERPLANE_USER", "ARG OVERPLANE_HOME"})
}

// AgentVersionArg maps an agent recipe name to its Dockerfile version build
// arg (codex -> CODEX_VERSION, claude-code -> CLAUDE_CODE_VERSION).
func AgentVersionArg(name string) string {
	return strings.ReplaceAll(strings.ToUpper(name), "-", "_") + "_VERSION"
}

// renderDockerfile generates the single-stage Dockerfile, layers ordered
// slowest-changing first; each RUN deletes its fragment in the same layer and
// ARG declarations sit immediately before the layer that consumes them so
// value changes invalidate as little cache as possible.
func (p *BuildPlan) renderDockerfile() string {
	var b strings.Builder
	b.WriteString("# syntax=docker/dockerfile:1.7\n")
	b.WriteString("# Generated by overplane agent setup; do not edit.\n")
	b.WriteString("ARG BASE_REF\n")
	b.WriteString("FROM ${BASE_REF}\n")
	b.WriteString("ARG SOURCE_DATE_EPOCH\n")
	if p.Recipe.PackageFamily == "alpine" {
		// Alpine bases ship without bash; the fragments are bash scripts.
		b.WriteString("RUN command -v bash >/dev/null 2>&1 || apk add --no-cache bash\n")
	}
	for _, frag := range p.Fragments {
		for _, arg := range frag.PreArgs {
			b.WriteString(arg + "\n")
		}
		staged := fragmentStageDir + "/" + frag.Name + ".sh"
		b.WriteString("COPY " + frag.Name + ".sh " + staged + "\n")
		b.WriteString("RUN chmod +x " + staged + " && " + staged)
		for _, a := range frag.Args {
			b.WriteString(" " + a)
		}
		b.WriteString(" && rm " + staged + "\n")
	}
	b.WriteString("COPY " + profileFileName + " /etc/overplane/" + profileFileName + "\n")
	b.WriteString("RUN chmod 0644 /etc/overplane/" + profileFileName + "\n")
	b.WriteString("ARG OVERPLANE_PROJECT\n")
	b.WriteString("ENV OVERPLANE_PROJECT=${OVERPLANE_PROJECT}\n")
	b.WriteString("COPY " + entrypointFileName + " " + entrypointPath + "\n")
	b.WriteString("RUN chmod +x " + entrypointPath + "\n")
	b.WriteString("ENTRYPOINT [\"tini\", \"--\", \"" + entrypointPath + "\"]\n")
	return b.String()
}

func (p *BuildPlan) buildArgs(
	cfg project.AgentContainer, recipe SetupRecipe, opts BuildPlanOptions,
) map[string]string {
	args := map[string]string{
		"BASE_REF":          cfg.BaseImage,
		"SOURCE_DATE_EPOCH": opts.SourceDateEpoch,
		"EXTRA_OS_PACKAGES": strings.Join(cfg.ExtraPackages[recipe.PackageFamily], " "),
		"OVERPLANE_UID":     opts.UID,
		"OVERPLANE_GID":     opts.GID,
		"OVERPLANE_USER":    opts.Username,
		"OVERPLANE_HOME":    opts.Home,
		"OVERPLANE_PROJECT": opts.ProjectName,
	}
	for _, sel := range cfg.AgentRecipes {
		args[AgentVersionArg(sel.Name)] = sel.Version
	}
	return args
}

// computeHash derives the deterministic build hash (spec #0003 §3.3): SHA-256
// short hash over base image, recipe name, fragment names and bytes (plus the
// profile and entrypoint assets), Dockerfile text, sorted build args, and
// platform.
func (p *BuildPlan) computeHash() string {
	h := hashutil.NewHasher()
	line := func(s string) { h.WriteString(s); h.WriteString("\n") }
	line(p.BaseImage)
	line(p.Recipe.Name)
	for _, frag := range p.Fragments {
		line(frag.Name)
		_, _ = h.Write(frag.Bytes)
		line("")
	}
	line(profileFileName)
	_, _ = h.Write(p.Profile)
	line("")
	line(entrypointFileName)
	_, _ = h.Write(p.Entrypoint)
	line("")
	line(p.Dockerfile)
	keys := make([]string, 0, len(p.BuildArgs))
	for k := range p.BuildArgs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		line(k + "=" + p.BuildArgs[k])
	}
	line(p.Platform)
	return h.Sum()
}

// Materialize writes the build context (fragments, profile, entrypoint,
// Dockerfile) into dir with SOURCE_DATE_EPOCH-pinned mtimes and returns the
// Dockerfile path.
func (p *BuildPlan) Materialize(dir string) (string, error) {
	epoch, err := strconv.ParseInt(p.SourceDateEpoch, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid SOURCE_DATE_EPOCH %q: %w", p.SourceDateEpoch, err)
	}
	modTime := time.Unix(epoch, 0)
	writeFile := func(name string, data []byte, mode os.FileMode) error {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, data, mode); err != nil {
			return err
		}
		return os.Chtimes(path, modTime, modTime)
	}
	for _, frag := range p.Fragments {
		if err := writeFile(frag.Name+".sh", frag.Bytes, 0o644); err != nil {
			return "", err
		}
	}
	if err := writeFile(profileFileName, p.Profile, 0o644); err != nil {
		return "", err
	}
	if err := writeFile(entrypointFileName, p.Entrypoint, 0o755); err != nil {
		return "", err
	}
	dfPath := filepath.Join(dir, "Dockerfile")
	if err := writeFile("Dockerfile", []byte(p.Dockerfile), 0o644); err != nil {
		return "", err
	}
	return dfPath, nil
}

func readProfile() ([]byte, error) {
	data, err := profileAsset()
	if err != nil {
		return nil, fmt.Errorf("embedded shell profile: %w", err)
	}
	return data, nil
}
