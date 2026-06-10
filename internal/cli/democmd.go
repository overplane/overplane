package cli

import (
	"context"
	"fmt"

	"github.com/overplane/overplane/internal/platform/color"
	"github.com/overplane/overplane/internal/platform/tui"
	"github.com/overplane/overplane/internal/platform/tui/nav"
)

type demoCommand struct{ r *Runner }

func (c demoCommand) Name() string  { return "demo" }
func (c demoCommand) Usage() string { return Binary + " demo" }

func (c demoCommand) Run(_ context.Context, args []string) error {
	if len(args) > 0 && isHelpToken(args[0]) {
		fmt.Fprint(c.r.Out, usage(color.HelpSpec{
			Command:     "demo",
			Usage:       Binary + " demo",
			Description: "Run a small palette-themed TUI demo.",
		}))
		return nil
	}
	if len(args) > 0 {
		return UsageError("demo takes no arguments")
	}
	if err := tui.EnsureInteractive("run demo in a TTY"); err != nil {
		return EnvError(err)
	}
	item, err := nav.Run([]nav.Item{
		{
			ID: "config", Title: "Config", Description: "strict YAML validation",
			Detail: "Config files are schema-validated and decoded with KnownFields.",
		},
		{
			ID: "theme", Title: "Theme", Description: "16-slot palette",
			Detail: "Terminal color is xterm-256 only and shared with TUI styles.",
		},
		{
			ID: "check", Title: "Check", Description: "local preflight checks",
			Detail: "Checks never make external network calls or print key material.",
		},
	}, "Overplane demo")
	if err != nil {
		return err
	}
	if item == nil {
		return nil
	}
	fmt.Fprintf(c.r.Out, "selected=%s\n", item.ID)
	return nil
}
