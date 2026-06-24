package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/overplane/overplane/internal/platform/color"
	oplog "github.com/overplane/overplane/internal/platform/log"
	"github.com/overplane/overplane/internal/platform/paths"
)

type themeCommand struct{ r *Runner }

func (c themeCommand) Name() string  { return "theme" }
func (c themeCommand) Usage() string { return Binary + " theme [preview|set|list]" }

func themeGroupHelp() color.HelpSpec {
	return color.HelpSpec{
		Command: "theme",
		Usage:   Binary + " theme [preview|set|list]",
		Description: "Preview the CLI terminal palette, or switch the site's visual brand identity " +
			"(set/list delegate to sitebuild inside an Overplane monorepo checkout).",
		Examples: []string{
			Binary + " theme",
			Binary + " theme preview",
			Binary + " theme list",
			Binary + " theme set hand-drawn",
		},
	}
}

func (c themeCommand) Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return c.preview(ctx, nil)
	}
	return runSubcommandGroup(ctx, args, c.r.Err, "theme", true, func() {
		fmt.Fprint(c.r.Out, usage(themeGroupHelp()))
	}, map[string]subcommandHandler{
		"preview": c.preview,
		"list":    c.list,
		"set":     c.set,
	})
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
	t.AppendRow(table.Row{"theme", color.Sprint(3, "ok"), res.Theme.Name})
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

func (c themeCommand) list(ctx context.Context, args []string) error {
	if len(args) > 0 && isHelpToken(args[0]) {
		return c.delegateSitebuild(ctx, "list")
	}
	if len(args) > 0 {
		return UsageError("theme list takes no arguments")
	}
	return c.delegateSitebuild(ctx, "list")
}

func (c themeCommand) set(ctx context.Context, args []string) error {
	if len(args) > 0 && isHelpToken(args[0]) {
		fmt.Fprint(c.r.Out, usage(color.HelpSpec{
			Command:     "theme set",
			Usage:       Binary + " theme set <name>",
			Description: "Apply a brand identity from the design gallery to config/data/theme.yaml.",
			Examples:    []string{Binary + " theme set hand-drawn", Binary + " theme set 22"},
		}))
		return nil
	}
	if len(args) == 0 {
		return UsageError("theme set requires an identity name")
	}
	return c.delegateSitebuild(ctx, "set", args...)
}

func (c themeCommand) delegateSitebuild(ctx context.Context, sub string, args ...string) error {
	p, err := paths.Resolve("")
	if err != nil {
		return EnvError(fmt.Errorf("theme %s requires an Overplane monorepo checkout: %w", sub, err))
	}
	siteCLI := filepath.Join(p.SiteDir, "cli.sh")
	if st, err := os.Stat(siteCLI); err != nil || st.IsDir() {
		return EnvError(fmt.Errorf("theme %s requires site tooling at %s", sub, siteCLI))
	}
	cmdArgs := append([]string{"theme", sub}, args...)
	cmd := exec.CommandContext(ctx, siteCLI, cmdArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = c.r.Out
	cmd.Stderr = c.r.Err
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code := exitErr.ExitCode()
			if code < 0 {
				code = 1
			}
			return ExitError{Code: code, Err: err}
		}
		return InternalError(err)
	}
	return nil
}
