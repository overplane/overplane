package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/overplane/overplane/internal/container"
	"github.com/overplane/overplane/internal/platform/color"
	oplog "github.com/overplane/overplane/internal/platform/log"
	"github.com/overplane/overplane/internal/platform/version"
	"github.com/overplane/overplane/internal/project"
	"github.com/overplane/overplane/internal/recipes"
)

func agentSetupHelp() color.HelpSpec {
	return color.HelpSpec{
		Command: "agent setup",
		Usage:   Binary + " agent setup [--force]",
		Description: "Build the project's agent container image from the agent section of overplane.yaml, " +
			"or reuse the cached image when nothing changed. The image is tagged " +
			"overplane-<project>:latest plus a content-hash tag and labeled with the project name; " +
			"re-running with an unchanged configuration is a fast no-build cache hit.",
		Flags: []color.HelpFlag{
			{Name: "--force", Description: "rebuild without the layer cache, re-resolving 'latest' agent versions"},
		},
		Examples: []string{Binary + " agent setup", Binary + " agent setup --force"},
	}
}

func (c agentCommand) setup(ctx context.Context, args []string) error {
	if len(args) > 0 && isHelpToken(args[0]) {
		fmt.Fprint(c.r.Out, usage(agentSetupHelp()))
		return nil
	}
	force, err := parseAgentSetupForce(c.r, args)
	if err != nil {
		return err
	}
	cfg, _, err := requireProject(c.r)
	if err != nil {
		return err
	}
	return runAgentSetup(ctx, c.r, cfg, force)
}

func parseAgentSetupForce(r *Runner, args []string) (bool, error) {
	fl := flag.NewFlagSet("agent setup", flag.ContinueOnError)
	fl.SetOutput(r.Err)
	force := fl.Bool("force", false, "rebuild without the layer cache")
	if err := fl.Parse(args); err != nil {
		return false, UsageError("agent setup: %v", err)
	}
	if fl.NArg() > 0 {
		return false, UsageError("agent setup takes no positional arguments")
	}
	return *force, nil
}

// runAgentSetup builds or reuses the project's agent container image.
func runAgentSetup(ctx context.Context, r *Runner, cfg *project.Config, force bool) error {
	if cfg == nil || cfg.Agent == nil {
		return InternalError(fmt.Errorf("missing agent configuration"))
	}
	reg, err := loadRegistry()
	if err != nil {
		return err
	}
	ac := agentCommand{r: r}
	client, err := agentEngineClient(ctx, cfg.Agent.Container.Runtime)
	if err != nil {
		return err
	}
	plan, err := ac.plan(ctx, reg, cfg.Agent.Container, cfg.Project.Name)
	if err != nil {
		return err
	}
	built, err := ac.ensureImage(ctx, client, plan, force)
	if err != nil {
		return err
	}
	ac.printSetupSummary(plan, built)
	return nil
}

func (c agentCommand) plan(
	ctx context.Context, reg *recipes.Registry, ac project.AgentContainer, name string,
) (*recipes.BuildPlan, error) {
	id := currentHostIdentity()
	plan, err := reg.BuildPlan(ac, recipes.BuildPlanOptions{
		ProjectName: name,
		UID:         id.UID,
		GID:         id.GID,
		Username:    id.Username,
		Home:        id.Home,
		Version:     version.Version,
	})
	if err != nil {
		return nil, InternalError(err)
	}
	oplog.FromContext(ctx).Info("planned agent image build",
		"step", "agent-setup-plan",
		"recipe", plan.Recipe.Name,
		"base", plan.BaseImage,
		"agents", agentSummary(ac),
		"hash", plan.Hash,
	)
	return plan, nil
}

// ensureImage performs the cache lookup / build phases and returns the
// resulting image.
func (c agentCommand) ensureImage(
	ctx context.Context, client container.Client, plan *recipes.BuildPlan, force bool,
) (container.Image, error) {
	log := oplog.FromContext(ctx)
	hits, err := client.ListLocalImages(ctx, container.ImageFilter{
		Labels: map[string]string{recipes.LabelBuildHash: plan.Hash},
	})
	if err != nil {
		return container.Image{}, withHint(err)
	}
	log.Debug("cache probe", "step", "agent-setup-cache", "label", recipes.LabelBuildHash,
		"hash", plan.Hash, "hits", len(hits))
	if len(hits) > 0 && !force {
		img, err := c.cacheHit(ctx, client, plan, hits)
		if err != nil {
			return container.Image{}, err
		}
		log.Info("agent image up to date; no build needed",
			"step", "agent-setup-cache-hit", "image", string(img.Ref), "hash", plan.Hash)
		return img, nil
	}
	if len(hits) > 0 && force {
		log.Warn("--force set; rebuilding despite cached image",
			"step", "agent-setup-build", "image", string(hits[0].Ref))
	}
	return c.build(ctx, client, plan, force)
}

// cacheHit retags :latest at the cached image when needed.
func (c agentCommand) cacheHit(
	ctx context.Context, client container.Client, plan *recipes.BuildPlan, hits []container.Image,
) (container.Image, error) {
	latest := container.Ref(plan.Tags[0])
	for _, img := range hits {
		if img.Ref == latest {
			return img, nil
		}
	}
	if err := client.TagLocalImage(ctx, hits[0].Ref, latest); err != nil {
		return container.Image{}, withHint(err)
	}
	img := hits[0]
	img.Ref = latest
	return img, nil
}

func (c agentCommand) build(
	ctx context.Context, client container.Client, plan *recipes.BuildPlan, force bool,
) (container.Image, error) {
	log := oplog.FromContext(ctx)
	contextDir, err := os.MkdirTemp("", "overplane-build-*")
	if err != nil {
		return container.Image{}, IOError(err)
	}
	defer func() { _ = os.RemoveAll(contextDir) }()
	dockerfile, err := plan.Materialize(contextDir)
	if err != nil {
		return container.Image{}, IOError(err)
	}
	log.Info("building agent image",
		"step", "agent-setup-build", "engine", client.Engine().String(),
		"tags", fmt.Sprint(plan.Tags), "no_cache", force)
	res, err := client.BuildLocalImage(ctx, container.BuildSpec{
		ContextDir:      contextDir,
		DockerfilePath:  dockerfile,
		Tags:            plan.Tags,
		Labels:          plan.Labels,
		BuildArgs:       plan.BuildArgs,
		NoCache:         force,
		SourceDateEpoch: plan.SourceDateEpoch,
		Progress:        c.r.Out,
	})
	if err != nil {
		return container.Image{}, withHint(err)
	}
	log.Info("agent image built",
		"step", "agent-setup-built",
		"image", shortImageID(res.Image.ID),
		"size", formatImageSize(res.Image.Size),
		"duration", res.Duration.Round(time.Millisecond).String(),
	)
	return res.Image, nil
}

func (c agentCommand) printSetupSummary(plan *recipes.BuildPlan, img container.Image) {
	t := color.Table(c.r.Out)
	t.AppendHeader(table.Row{"Field", "Value"})
	add := func(field, value string) { t.AppendRow(table.Row{color.Sprint(4, field), value}) }
	add("Tags", plan.Tags[0]+", "+plan.Tags[1])
	add("Image ID", shortImageID(img.ID))
	add("Size", formatImageSize(img.Size))
	if !img.Created.IsZero() {
		add("Created", img.Created.UTC().Format(time.RFC3339))
	}
	add("Recipe", plan.Recipe.Name)
	add("Base", plan.BaseImage)
	add("Agents", agentSelectionSummary(plan))
	add("Build hash", plan.Hash)
	t.Render()
	fmt.Fprintf(c.r.Out, "\nNext: %s\n", color.Sprint(4, Binary+" shell"))
}

func agentSelectionSummary(plan *recipes.BuildPlan) string {
	return agentSummary(project.AgentContainer{AgentRecipes: plan.Agents})
}
