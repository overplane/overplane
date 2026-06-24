package cli

import (
	"context"
	"fmt"

	"github.com/overplane/overplane/internal/platform/color"
)

type shellCommand struct{ r *Runner }

func (c shellCommand) Name() string  { return "shell" }
func (c shellCommand) Usage() string { return Binary + " shell" }

func shellHelp() color.HelpSpec {
	return color.HelpSpec{
		Command: "shell",
		Usage:   Binary + " shell",
		Description: "Open an ephemeral interactive shell inside the project's agent container image " +
			"(built by 'setup'). Same as '" + Binary + " agent shell': the container mirrors a real " +
			"agent run's environment — toolchain, installed agents, and passed-through API keys — " +
			"but mounts nothing from the host; everything inside is discarded on exit.",
		Examples: []string{Binary + " shell", Binary + " setup && " + Binary + " shell"},
	}
}

func (c shellCommand) Run(ctx context.Context, args []string) error {
	if len(args) > 0 && isHelpToken(args[0]) {
		fmt.Fprint(c.r.Out, usage(shellHelp()))
		return nil
	}
	return runAgentShell(ctx, c.r, Binary+" shell", args)
}
