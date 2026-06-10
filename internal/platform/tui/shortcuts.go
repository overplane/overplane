package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/overplane/overplane/internal/platform/color"
	"golang.org/x/term"
)

var (
	ErrNotInteractive = errors.New("not interactive")
	ErrCancelled      = errors.New("cancelled")
)

type HintError struct {
	Err      error
	HintText string
}

func (e HintError) Error() string { return e.Err.Error() + ": " + e.HintText }
func (e HintError) Unwrap() error { return e.Err }
func (e HintError) Hint() string  { return e.HintText }

type Option struct {
	Value, Label, Description string
}

func ConfirmYesNo(ctx context.Context, title, desc string, defaultYes bool) (bool, error) {
	if err := requireTTY("pass a non-interactive flag instead"); err != nil {
		return false, err
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	var v = defaultYes
	err := huh.NewConfirm().Title(title).Description(desc).Value(&v).Run()
	return v, normalizeErr(err)
}

func PromptString(ctx context.Context, title, desc, placeholder string, validate func(string) error) (string, error) {
	if err := requireTTY("pass the value as a flag"); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	var v string
	err := huh.NewInput().Title(title).Description(desc).Placeholder(placeholder).Validate(validate).Value(&v).Run()
	return v, normalizeErr(err)
}

func Select(ctx context.Context, title string, options []Option) (Option, error) {
	if err := requireTTY("pass the selection as a flag"); err != nil {
		return Option{}, err
	}
	if err := ctx.Err(); err != nil {
		return Option{}, err
	}
	var val string
	hopts := make([]huh.Option[string], 0, len(options))
	byValue := map[string]Option{}
	for _, o := range options {
		hopts = append(hopts, huh.NewOption(o.Label, o.Value))
		byValue[o.Value] = o
	}
	err := huh.NewSelect[string]().Title(title).Options(hopts...).Value(&val).Run()
	return byValue[val], normalizeErr(err)
}

func MultiSelect(ctx context.Context, title string, options []Option, initial []string) ([]Option, error) {
	if err := requireTTY("pass selections as flags"); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	selected := append([]string{}, initial...)
	hopts := make([]huh.Option[string], 0, len(options))
	byValue := map[string]Option{}
	for _, o := range options {
		hopts = append(hopts, huh.NewOption(o.Label, o.Value))
		byValue[o.Value] = o
	}
	err := huh.NewMultiSelect[string]().Title(title).Options(hopts...).Value(&selected).Run()
	if err := normalizeErr(err); err != nil {
		return nil, err
	}
	out := make([]Option, 0, len(selected))
	for _, v := range selected {
		out = append(out, byValue[v])
	}
	return out, nil
}

func ChoosePath(ctx context.Context, title, startDir string, onlyDirs bool) (string, error) {
	if onlyDirs {
		title += " (directory)"
	}
	return PromptString(ctx, title, "Enter a path", startDir, nil)
}

func OpenEditor(ctx context.Context, path string) error {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		return HintError{Err: ErrNotInteractive, HintText: "set VISUAL or EDITOR"}
	}
	cmd := exec.CommandContext(ctx, editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Theme(bool) huh.Theme {
	p := color.Get()
	t := huh.ThemeBase()
	t.Focused.Base = t.Focused.Base.Foreground(lipgloss.Color(fmt.Sprint(p[4])))
	t.Focused.Title = t.Focused.Title.Foreground(lipgloss.Color(fmt.Sprint(p[0])))
	t.Focused.Description = t.Focused.Description.Foreground(lipgloss.Color(fmt.Sprint(p[7])))
	return *t
}

func requireTTY(hint string) error {
	inputTTY := term.IsTerminal(int(os.Stdin.Fd()))
	outputTTY := term.IsTerminal(int(os.Stdout.Fd())) || term.IsTerminal(int(os.Stderr.Fd()))
	if inputTTY && outputTTY {
		return nil
	}
	return HintError{Err: ErrNotInteractive, HintText: hint}
}

func EnsureInteractive(hint string) error {
	return requireTTY(hint)
}

func normalizeErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return ErrCancelled
	}
	return err
}
