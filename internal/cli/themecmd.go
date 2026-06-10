package cli

import (
	"context"
	"fmt"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/overplane/overplane/internal/platform/color"
	oplog "github.com/overplane/overplane/internal/platform/log"
)

type themeCommand struct{ r *Runner }

func (c themeCommand) Name() string  { return "theme" }
func (c themeCommand) Usage() string { return Binary + " theme preview" }

func (c themeCommand) Run(ctx context.Context, args []string) error {
	return runSubcommandGroup(ctx, args, c.r.Err, "theme", true, func() {
		fmt.Fprint(c.r.Out, usage(color.HelpSpec{
			Command:     "theme",
			Usage:       Binary + " theme preview",
			Description: "Preview the resolved terminal theme.",
			Examples:    []string{Binary + " theme preview"},
		}))
	}, map[string]subcommandHandler{"preview": c.preview})
}

func (c themeCommand) preview(_ context.Context, args []string) error {
	if len(args) > 0 && isHelpToken(args[0]) {
		fmt.Fprint(c.r.Out, usage(color.HelpSpec{Command: "theme preview", Usage: Binary + " theme preview"}))
		return nil
	}
	if len(args) > 0 {
		return UsageError("theme preview takes no arguments")
	}
	res, err := color.ApplyResolvedTheme("")
	if err != nil {
		return ValidationError(err)
	}
	fmt.Fprintf(c.r.Out, "Theme: %s (%s)\n\n", res.Theme.Name, res.Source)
	roles := []string{
		"titles", "steps", "warnings/examples", "success", "info/flags", "accents", "debug", "dim",
		"placeholders", "hash 9", "hash 10", "hash 11", "hash 12", "hash 13", "near-white", "dark",
	}
	for i, idx := range color.Get() {
		block := color.BG(i) + "  " + "\x1b[0m"
		if !color.Enabled() {
			block = "[]"
		}
		fmt.Fprintf(c.r.Out, "%2d  xterm-%3d  %s  %s\n", i, idx, block, roles[i])
	}
	fmt.Fprintln(c.r.Out)
	previewLog, err := oplog.New(oplog.FormatPretty, "debug", c.r.Out, false)
	if err != nil {
		return InternalError(err)
	}
	previewLog.Error("sample log line", "action", "preview")
	previewLog.Warn("sample log line", "action", "preview")
	previewLog.Info("sample log line", "action", "preview")
	previewLog.Debug("sample log line", "action", "preview")
	fmt.Fprintln(c.r.Out)
	t := color.Table(c.r.Out)
	t.AppendHeader(table.Row{"Name", "Status", "Detail"})
	t.AppendRow(table.Row{"theme", color.Sprint(3, "ok"), res.Source})
	t.Render()
	fmt.Fprintln(c.r.Out)
	fmt.Fprint(c.r.Out, color.RenderHelp(color.HelpSpec{
		Command:     "sample",
		Usage:       Binary + " sample --flag VALUE",
		Description: "Sample help block.",
		Flags:       []color.HelpFlag{{Name: "--flag", Placeholder: "VALUE", Description: "example flag"}},
		Examples:    []string{Binary + " sample --flag value"},
	}))
	return nil
}
