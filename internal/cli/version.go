package cli

import (
	"context"
	"fmt"

	"github.com/overplane/overplane/internal/platform/color"
	"github.com/overplane/overplane/internal/platform/version"
)

type versionCommand struct{ r *Runner }

func (c versionCommand) Name() string  { return "version" }
func (c versionCommand) Usage() string { return Binary + " version" }

func (c versionCommand) Run(_ context.Context, args []string) error {
	if len(args) > 0 && isHelpToken(args[0]) {
		fmt.Fprint(c.r.Out, usage(versionHelp()))
		return nil
	}
	if len(args) > 0 {
		return UsageError("version takes no arguments")
	}
	fmt.Fprintln(c.r.Out, version.String(Binary))
	return nil
}

func versionHelp() color.HelpSpec {
	return color.HelpSpec{
		Command:     "version",
		Usage:       Binary + " version",
		Description: "Print build version information.",
		Examples:    []string{Binary + " version"},
	}
}
