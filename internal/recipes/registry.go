// Package recipes owns the embedded recipe registry: the typed source of
// truth for supported container runtimes, allowed build/run engine pairs,
// setup recipes (embedded provisioning fragments), and agent install recipes.
// It validates project configs against the registry and plans container image
// builds (Dockerfile generation, build args, tags, labels, build hash) for
// the CLI to execute through internal/container.
package recipes

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/overplane/overplane/internal/platform/configloader"
	assetfs "github.com/overplane/overplane/internal/platform/embed/assets"
	"github.com/overplane/overplane/internal/platform/serde/jsonschema"
	"github.com/overplane/overplane/internal/project"
)

// RegistryAssetKey is the embedded location of the registry data file.
const RegistryAssetKey = "files/recipes/recipes.yaml"

// SchemaAssetKey is the embedded location of the registry schema.
const SchemaAssetKey = "files/schema/recipes.schema.json"

// RegistryFileName is the registry's basename, recognized by
// `overplane config validate`.
const RegistryFileName = "recipes.yaml"

const (
	setupFragmentPrefix = "files/setup/"
	profileAssetKey     = "files/misc/bash_profile_extra.sh"
	entrypointFragment  = "overplane-entrypoint"
)

// Runtime describes a container engine the registry knows about.
type Runtime struct {
	Name         string   `json:"name" yaml:"name"`
	Description  string   `json:"description" yaml:"description"`
	Capabilities []string `json:"capabilities" yaml:"capabilities"`
	Status       string   `json:"status" yaml:"status"`
}

// HasCapability reports whether the runtime declares the named capability.
func (r Runtime) HasCapability(capability string) bool {
	for _, c := range r.Capabilities {
		if c == capability {
			return true
		}
	}
	return false
}

// RuntimePair is an allowed build/run engine combination.
type RuntimePair struct {
	Build string `json:"build" yaml:"build"`
	Run   string `json:"run" yaml:"run"`
}

// SetupRecipe is a built-in base-image provisioning recipe.
type SetupRecipe struct {
	Name             string   `json:"name" yaml:"name"`
	Description      string   `json:"description" yaml:"description"`
	PackageFamily    string   `json:"package_family" yaml:"package_family"`
	DefaultBaseImage string   `json:"default_base_image" yaml:"default_base_image"`
	Fragments        []string `json:"fragments" yaml:"fragments"`
	UserFragment     string   `json:"user_fragment" yaml:"user_fragment"`
}

// AgentRecipe is an installable AI coding agent.
type AgentRecipe struct {
	Name                  string   `json:"name" yaml:"name"`
	DisplayName           string   `json:"display_name" yaml:"display_name"`
	Description           string   `json:"description" yaml:"description"`
	Command               string   `json:"command" yaml:"command"`
	InstallFragment       string   `json:"install_fragment" yaml:"install_fragment"`
	SupportedSetupRecipes []string `json:"supported_setup_recipes" yaml:"supported_setup_recipes"`
	EnvPassthrough        []string `json:"env_passthrough" yaml:"env_passthrough"`
	DocsURL               string   `json:"docs_url" yaml:"docs_url"`
}

// SupportsSetupRecipe reports whether the agent installs on the named recipe.
func (a AgentRecipe) SupportsSetupRecipe(name string) bool {
	for _, r := range a.SupportedSetupRecipes {
		if r == name {
			return true
		}
	}
	return false
}

// Registry is the decoded, validated recipe registry.
type Registry struct {
	SchemaVersion     int           `json:"schema_version" yaml:"schema_version"`
	ContainerRuntimes []Runtime     `json:"container_runtimes" yaml:"container_runtimes"`
	RuntimePairs      []RuntimePair `json:"runtime_pairs" yaml:"runtime_pairs"`
	SetupRecipes      []SetupRecipe `json:"setup_recipes" yaml:"setup_recipes"`
	AgentRecipes      []AgentRecipe `json:"agent_recipes" yaml:"agent_recipes"`
}

// SchemaBytes returns the embedded registry JSON Schema.
func SchemaBytes() []byte {
	data, err := assetfs.ReadFile(SchemaAssetKey)
	if err != nil {
		panic(fmt.Sprintf("recipes: embedded schema missing: %v", err))
	}
	return data
}

// RegistryBytes returns the embedded registry data file.
func RegistryBytes() []byte {
	data, err := assetfs.ReadFile(RegistryAssetKey)
	if err != nil {
		panic(fmt.Sprintf("recipes: embedded registry missing: %v", err))
	}
	return data
}

var loadOnce = sync.OnceValues(func() (*Registry, error) {
	return loadBytes(RegistryBytes())
})

// Load returns the embedded registry: schema-validated, strictly decoded, and
// structurally self-checked. The result is cached after the first call. A
// failure here means the shipped binary is broken (the embedded registry is
// invalid) and maps to InternalError (exit 5) at the CLI boundary.
func Load() (*Registry, error) {
	return loadOnce()
}

func loadBytes(data []byte) (*Registry, error) {
	reg, err := configloader.LoadBytes[Registry](data, SchemaBytes(), RegistryFileName)
	if err != nil {
		return nil, err
	}
	if problems := reg.selfCheck(); len(problems) > 0 {
		return nil, configloader.ValidationError{Problems: problems}
	}
	return reg, nil
}

// ValidateRegistryBytes validates an arbitrary registry document (schema pass
// plus the structural self-check) and returns its problems, for
// `overplane config validate`.
func ValidateRegistryBytes(data []byte) ([]jsonschema.Problem, error) {
	problems, err := configloader.ValidateBytes(data, SchemaBytes(), RegistryFileName)
	if err != nil || len(problems) > 0 {
		return problems, err
	}
	reg, err := configloader.LoadBytes[Registry](data, SchemaBytes(), RegistryFileName)
	if err != nil {
		return nil, err
	}
	return reg.selfCheck(), nil
}

// SetupRecipe looks up a setup recipe by name.
func (r *Registry) SetupRecipe(name string) (SetupRecipe, bool) {
	for _, s := range r.SetupRecipes {
		if s.Name == name {
			return s, true
		}
	}
	return SetupRecipe{}, false
}

// AgentRecipe looks up an agent recipe by name.
func (r *Registry) AgentRecipe(name string) (AgentRecipe, bool) {
	for _, a := range r.AgentRecipes {
		if a.Name == name {
			return a, true
		}
	}
	return AgentRecipe{}, false
}

// Runtime looks up a container runtime by name.
func (r *Registry) Runtime(name string) (Runtime, bool) {
	for _, rt := range r.ContainerRuntimes {
		if rt.Name == name {
			return rt, true
		}
	}
	return Runtime{}, false
}

// PackageFamilies returns the set of package families any registered setup
// recipe defines, sorted.
func (r *Registry) PackageFamilies() []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(r.SetupRecipes))
	for _, s := range r.SetupRecipes {
		if !seen[s.PackageFamily] {
			seen[s.PackageFamily] = true
			out = append(out, s.PackageFamily)
		}
	}
	sort.Strings(out)
	return out
}

// EnvPassthrough returns the union of the API-key variables declared by every
// configured agent recipe and the project-level env_passthrough list, sorted
// and deduplicated.
func (r *Registry) EnvPassthrough(cfg project.AgentContainer) []string {
	seen := map[string]bool{}
	var out []string
	add := func(key string) {
		if key != "" && !seen[key] {
			seen[key] = true
			out = append(out, key)
		}
	}
	for _, sel := range cfg.AgentRecipes {
		if recipe, ok := r.AgentRecipe(sel.Name); ok {
			for _, key := range recipe.EnvPassthrough {
				add(key)
			}
		}
	}
	for _, key := range cfg.EnvPassthrough {
		add(key)
	}
	sort.Strings(out)
	return out
}

// problemList collects pointer-addressed structural problems.
type problemList []jsonschema.Problem

func (p *problemList) add(pointer, message string, value any) {
	*p = append(*p, jsonschema.Problem{Pointer: pointer, Message: message, Value: value})
}

// selfCheck enforces the structural invariants the schema cannot express:
// every referenced fragment resolves to an embedded files/setup/<name>.sh
// asset, runtime pairs reference declared runtimes with the matching
// capability, supported_setup_recipes entries exist, and names are unique.
func (r *Registry) selfCheck() []jsonschema.Problem {
	var problems problemList
	runtimes := r.checkRuntimes(&problems)
	r.checkRuntimePairs(&problems, runtimes)
	setupNames := r.checkSetupRecipes(&problems)
	r.checkAgentRecipes(&problems, setupNames)
	if !fragmentExists(entrypointFragment) {
		problems.add("/", "embedded entrypoint fragment missing", entrypointFragment)
	}
	if _, err := assetfs.ReadFile(profileAssetKey); err != nil {
		problems.add("/", "embedded shell profile missing", profileAssetKey)
	}
	return problems
}

func (r *Registry) checkRuntimes(problems *problemList) map[string]Runtime {
	runtimes := map[string]Runtime{}
	for i, rt := range r.ContainerRuntimes {
		if _, dup := runtimes[rt.Name]; dup {
			problems.add(fmt.Sprintf("/container_runtimes/%d/name", i), "duplicate runtime name", rt.Name)
		}
		runtimes[rt.Name] = rt
	}
	return runtimes
}

func (r *Registry) checkRuntimePairs(problems *problemList, runtimes map[string]Runtime) {
	for i, p := range r.RuntimePairs {
		ptr := fmt.Sprintf("/runtime_pairs/%d", i)
		if rt, ok := runtimes[p.Build]; !ok || !rt.HasCapability("build") {
			problems.add(ptr+"/build", "must name a declared runtime with the build capability", p.Build)
		}
		if rt, ok := runtimes[p.Run]; !ok || !rt.HasCapability("run") {
			problems.add(ptr+"/run", "must name a declared runtime with the run capability", p.Run)
		}
	}
}

func (r *Registry) checkSetupRecipes(problems *problemList) map[string]bool {
	setupNames := map[string]bool{}
	for i, s := range r.SetupRecipes {
		ptr := fmt.Sprintf("/setup_recipes/%d", i)
		if setupNames[s.Name] {
			problems.add(ptr+"/name", "duplicate setup recipe name", s.Name)
		}
		setupNames[s.Name] = true
		for j, frag := range s.Fragments {
			if !fragmentExists(frag) {
				problems.add(fmt.Sprintf("%s/fragments/%d", ptr, j), "fragment has no embedded files/setup asset", frag)
			}
		}
		if !fragmentExists(s.UserFragment) {
			problems.add(ptr+"/user_fragment", "fragment has no embedded files/setup asset", s.UserFragment)
		}
	}
	return setupNames
}

func (r *Registry) checkAgentRecipes(problems *problemList, setupNames map[string]bool) {
	agentNames := map[string]bool{}
	for i, a := range r.AgentRecipes {
		ptr := fmt.Sprintf("/agent_recipes/%d", i)
		if agentNames[a.Name] {
			problems.add(ptr+"/name", "duplicate agent recipe name", a.Name)
		}
		agentNames[a.Name] = true
		if !fragmentExists(a.InstallFragment) {
			problems.add(ptr+"/install_fragment", "fragment has no embedded files/setup asset", a.InstallFragment)
		}
		for j, s := range a.SupportedSetupRecipes {
			if !setupNames[s] {
				problems.add(fmt.Sprintf("%s/supported_setup_recipes/%d", ptr, j), "unknown setup recipe", s)
			}
		}
	}
}

func fragmentExists(name string) bool {
	if strings.TrimSpace(name) == "" {
		return false
	}
	_, err := assetfs.ReadFile(setupFragmentPrefix + name + ".sh")
	return err == nil
}

func fragmentBytes(name string) ([]byte, error) {
	return assetfs.ReadFile(setupFragmentPrefix + name + ".sh")
}

func profileAsset() ([]byte, error) {
	return assetfs.ReadFile(profileAssetKey)
}
