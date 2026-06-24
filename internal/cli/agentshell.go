package cli

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/overplane/overplane/internal/container"
	"github.com/overplane/overplane/internal/platform/color"
	"github.com/overplane/overplane/internal/platform/env"
	oplog "github.com/overplane/overplane/internal/platform/log"
	"github.com/overplane/overplane/internal/platform/timeutil"
	"github.com/overplane/overplane/internal/platform/tui"
	"github.com/overplane/overplane/internal/project"
	"github.com/overplane/overplane/internal/recipes"
)

// agentEnsureInteractive guards the shell subcommand behind a real TTY; a
// package var so tests can simulate interactive sessions.
var agentEnsureInteractive = tui.EnsureInteractive

func agentShellHelp() color.HelpSpec {
	return color.HelpSpec{
		Command: "agent shell",
		Usage:   Binary + " agent shell",
		Description: "Open an ephemeral interactive shell inside the project's agent container image " +
			"(built by 'agent setup'). The container mirrors a real agent run's environment -- " +
			"toolchain, installed agents, and passed-through API keys -- but mounts nothing from " +
			"the host: everything inside is discarded on exit.",
		Examples: []string{Binary + " agent shell", Binary + " agent setup && " + Binary + " agent shell"},
	}
}

func (c agentCommand) shell(ctx context.Context, args []string) error {
	if len(args) > 0 && isHelpToken(args[0]) {
		fmt.Fprint(c.r.Out, usage(agentShellHelp()))
		return nil
	}
	return runAgentShell(ctx, c.r, Binary+" agent shell", args)
}

// runAgentShell opens an ephemeral interactive shell in the project's agent
// image. usageCmd is the command string shown in TTY guard errors.
func runAgentShell(ctx context.Context, r *Runner, usageCmd string, args []string) error {
	if len(args) > 0 {
		return UsageError("shell takes no arguments")
	}
	cfg, _, err := requireProject(r)
	if err != nil {
		return err
	}
	if err := agentEnsureInteractive("run '" + usageCmd + "' from a terminal"); err != nil {
		return EnvError(err)
	}
	reg, err := loadRegistry()
	if err != nil {
		return err
	}
	client, err := agentEngineClient(ctx, cfg.Agent.Container.Runtime)
	if err != nil {
		return err
	}
	latest := container.Ref(recipes.LatestTag(cfg.Project.Name))
	if err := requireShellImage(ctx, client, latest); err != nil {
		return err
	}
	ac := agentCommand{r: r}
	return ac.runShell(ctx, client, cfg, reg, latest)
}

// requireShellImage verifies the project's :latest image exists locally.
func requireShellImage(ctx context.Context, client container.Client, latest container.Ref) error {
	images, err := client.ListLocalImages(ctx, container.ImageFilter{Reference: latest.String()})
	if err != nil {
		return withHint(err)
	}
	if len(images) == 0 {
		return EnvError(fmt.Errorf(
			"image %s not found; run '%s setup' first", latest, Binary))
	}
	return nil
}

func (c agentCommand) runShell(
	ctx context.Context,
	client container.Client,
	cfg *project.Config,
	reg *recipes.Registry,
	latest container.Ref,
) error {
	log := oplog.FromContext(ctx)
	envVals := shellEnv(ctx, cfg, reg)
	id := currentHostIdentity()
	start := timeutil.NowUTC()
	name := fmt.Sprintf("overplane-shell-%s-%d", cfg.Project.Name, start.Unix())
	log.Info("starting agent shell",
		"step", "agent-shell-start", "image", latest.String(), "name", name,
		"env_keys", sortedKeys(envVals))
	ctr, err := client.RunLocalImage(ctx, latest, []string{"bash", "-l"}, container.RunOptions{
		Env:         envVals,
		NetworkMode: "host",
		User:        &container.UserSpec{UID: id.UID, GID: id.GID},
		WorkDir:     id.Home,
		Stdin:       c.r.In,
		Stdout:      c.r.Out,
		Stderr:      c.r.Err,
		TTY:         true,
		Name:        name,
		Labels: map[string]string{
			recipes.LabelProject: cfg.Project.Name,
			recipes.LabelShell:   "true",
		},
	})
	duration := timeutil.NowUTC().Sub(start)
	log.Info("agent shell exited",
		"step", "agent-shell-exit", "duration", timeutil.HumanDuration(duration),
		"exit_code", ctr.ExitCode)
	var exitErr container.ExitError
	if errors.As(err, &exitErr) {
		// The container's exit status becomes the command's exit status.
		return ExitError{Code: exitErr.Code, Err: fmt.Errorf("shell exited with status %d", exitErr.Code)}
	}
	if err != nil {
		return withHint(err)
	}
	return nil
}

// shellEnv resolves the passthrough environment for the shell container:
// registry API keys for the configured agents, project-level extras, plus
// TERM and LANG. Missing keys for configured agents WARN once each.
func shellEnv(ctx context.Context, cfg *project.Config, reg *recipes.Registry) map[string]string {
	log := oplog.FromContext(ctx)
	keys := reg.EnvPassthrough(cfg.Agent.Container)
	keys = append(keys, "TERM", "LANG")
	vals := env.Passthrough(keys)
	for _, sel := range cfg.Agent.Container.AgentRecipes {
		recipe, ok := reg.AgentRecipe(sel.Name)
		if !ok {
			continue
		}
		for _, key := range recipe.EnvPassthrough {
			if _, set := vals[key]; !set {
				log.Warn("agent API key not set on host",
					"step", "agent-shell-env", "key", key, "agent", sel.Name,
					"hint", fmt.Sprintf("export %s to use %s inside the container", key, sel.Name))
			}
		}
	}
	log.Debug("resolved shell environment",
		"step", "agent-shell-env", "keys", sortedKeys(vals))
	return vals
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
