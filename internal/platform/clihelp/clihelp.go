package clihelp

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/overplane/overplane/internal/platform/color"
)

type Command struct {
	Name        string
	Description string
}

type Flag struct {
	Name        string
	Description string
}

type RootSpec struct {
	Binary      string
	Description string
	Version     string
	Runtime     string
	Metadata    string
	Banner      []byte
	Fallback    string
	Commands    []Command
	Flags       []Flag
}

func RenderRoot(w io.Writer, spec RootSpec) {
	title := spec.Binary
	if spec.Description != "" {
		title += " - " + spec.Description
	}
	if spec.Version != "" {
		title += " v" + spec.Version
	}
	if spec.Runtime != "" {
		title += " (" + spec.Runtime + ")"
	}
	fmt.Fprintln(w, color.Sprint(0, title))
	if spec.Metadata != "" {
		fmt.Fprintf(w, "%s\n\n", color.Sprint(7, spec.Metadata))
	}
	printBanner(w, spec)
	fmt.Fprintf(w, "%s\n  %s <command> [flags]\n\n", color.Sprint(0, "Usage"), spec.Binary)
	fmt.Fprintln(w, color.Sprint(0, "Commands"))
	commands := append([]Command(nil), spec.Commands...)
	sort.Slice(commands, func(i, j int) bool { return commands[i].Name < commands[j].Name })
	for _, cmd := range commands {
		fmt.Fprintf(w, "  %s%s%s\n", color.Sprint(4, cmd.Name), padding(cmd.Name), cmd.Description)
	}
	if len(spec.Flags) > 0 {
		fmt.Fprintf(w, "\n%s\n", color.Sprint(0, "Global flags"))
		for _, flag := range spec.Flags {
			fmt.Fprintf(w, "  %s%s%s\n", color.Sprint(4, flag.Name), flagPadding(flag.Name), flag.Description)
		}
	}
}

func printBanner(w io.Writer, spec RootSpec) {
	if len(spec.Banner) > 0 {
		_, _ = w.Write(spec.Banner)
		fmt.Fprintln(w)
		return
	}
	if spec.Fallback != "" {
		fmt.Fprintln(w, spec.Fallback)
		fmt.Fprintln(w)
	}
}

func padding(name string) string {
	const width = 14
	spaces := width - len(name)
	if spaces < 2 {
		spaces = 2
	}
	return strings.Repeat(" ", spaces)
}

func flagPadding(name string) string {
	const width = 28
	spaces := width - len(name)
	if spaces < 2 {
		spaces = 2
	}
	return strings.Repeat(" ", spaces)
}
