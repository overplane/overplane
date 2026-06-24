package cli

import (
	"context"
	"flag"
	"fmt"

	"github.com/overplane/overplane/internal/platform/color"
)

type setupCommand struct{ r *Runner }

func (c setupCommand) Name() string  { return "setup" }
func (c setupCommand) Usage() string { return Binary + " setup [--force]" }

func setupHelp() color.HelpSpec {
	return color.HelpSpec{
		Command: "setup",
		Usage:   Binary + " setup [--force]",
		Description: "Validate overplane.yaml, run project-aware system checks, and build " +
			"(or reuse) the agent container image configured in the file.",
		Flags: []color.HelpFlag{
			{Name: "--force", Description: "rebuild without the layer cache, re-resolving 'latest' agent versions"},
		},
		Examples: []string{Binary + " setup", Binary + " setup --force"},
	}
}

func (c setupCommand) Run(ctx context.Context, args []string) error {
	if len(args) > 0 && isHelpToken(args[0]) {
		fmt.Fprint(c.r.Out, usage(setupHelp()))
		return nil
	}
	fl := flag.NewFlagSet("setup", flag.ContinueOnError)
	fl.SetOutput(c.r.Err)
	force := fl.Bool("force", false, "rebuild without the layer cache")
	if err := fl.Parse(args); err != nil {
		return UsageError("setup: %v", err)
	}
	if fl.NArg() > 0 {
		return UsageError("setup takes no positional arguments")
	}

	cfg, _, err := requireProject(c.r)
	if err != nil {
		return err
	}
	reg, err := loadRegistry()
	if err != nil {
		return err
	}
	if err := runProjectChecks(ctx, cfg, reg); err != nil {
		return err
	}
	return runAgentSetup(ctx, c.r, cfg, *force)
}
