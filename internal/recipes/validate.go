package recipes

import (
	"fmt"

	"github.com/overplane/overplane/internal/platform/configloader"
	"github.com/overplane/overplane/internal/platform/serde/jsonschema"
	"github.com/overplane/overplane/internal/project"
)

// ValidateProjectConfig cross-checks a loaded overplane.yaml against the
// registry (the registry-aware second pass of spec #0003 §2.3): the runtime
// must be a supported engine, the setup recipe must exist, every configured
// agent recipe must exist and support the selected setup recipe, and every
// extra_packages key must name a package family some registered recipe
// defines.
func ValidateProjectConfig(r *Registry, cfg *project.Config) []jsonschema.Problem {
	if cfg == nil || cfg.Agent == nil {
		return nil
	}
	var problems problemList
	ac := cfg.Agent.Container
	r.checkProjectRuntime(&problems, ac)
	r.checkProjectAgents(&problems, ac)
	r.checkProjectPackageFamilies(&problems, ac)
	return problems
}

func (r *Registry) checkProjectRuntime(problems *problemList, ac project.AgentContainer) {
	if rt, ok := r.Runtime(ac.Runtime); !ok || rt.Status != "supported" ||
		!rt.HasCapability("build") || !rt.HasCapability("run") {
		problems.add("/agent/container/runtime",
			fmt.Sprintf("unsupported container runtime %q (see the Agent Environment reference)", ac.Runtime),
			ac.Runtime)
	}
	if _, ok := r.SetupRecipe(ac.SetupRecipe); !ok {
		problems.add("/agent/container/setup_recipe",
			fmt.Sprintf("unknown setup recipe %q (see the Agent Environment reference)", ac.SetupRecipe),
			ac.SetupRecipe)
	}
}

func (r *Registry) checkProjectAgents(problems *problemList, ac project.AgentContainer) {
	for i, sel := range ac.AgentRecipes {
		ptr := fmt.Sprintf("/agent/container/agent_recipes/%d/name", i)
		recipe, ok := r.AgentRecipe(sel.Name)
		if !ok {
			problems.add(ptr,
				fmt.Sprintf("unknown agent recipe %q (see the Agent Environment reference)", sel.Name), sel.Name)
			continue
		}
		if !recipe.SupportsSetupRecipe(ac.SetupRecipe) {
			problems.add(ptr,
				fmt.Sprintf("agent recipe %q does not support setup recipe %q", sel.Name, ac.SetupRecipe), sel.Name)
		}
	}
}

func (r *Registry) checkProjectPackageFamilies(problems *problemList, ac project.AgentContainer) {
	families := map[string]bool{}
	for _, f := range r.PackageFamilies() {
		families[f] = true
	}
	for family := range ac.ExtraPackages {
		if !families[family] {
			problems.add("/agent/container/extra_packages/"+family,
				fmt.Sprintf("no setup recipe defines package family %q", family), family)
		}
	}
}

// LoadProjectConfig loads, schema-validates, semantically validates, and
// registry-validates the project file at path. This is the loader every
// agent-aware consumer should use; failures arrive as
// configloader.ValidationError with pointer-addressed problems.
func LoadProjectConfig(path string) (*project.Config, error) {
	cfg, err := project.Load(path)
	if err != nil {
		return nil, err
	}
	reg, err := Load()
	if err != nil {
		return nil, fmt.Errorf("embedded recipe registry: %w", err)
	}
	if problems := ValidateProjectConfig(reg, cfg); len(problems) > 0 {
		return nil, configloader.ValidationError{Problems: problems}
	}
	return cfg, nil
}
