package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/overplane/overplane/internal/platform/clihelp"
	"github.com/overplane/overplane/internal/platform/color"
	assetfs "github.com/overplane/overplane/internal/platform/embed/assets"
	"github.com/overplane/overplane/internal/platform/telemetry"
	"github.com/overplane/overplane/internal/platform/version"
	"go.opentelemetry.io/otel/trace"
)

const Binary = "overplane"
const Description = "Evolve verified software."

type Runner struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
	Tel *telemetry.Providers
}

// ConfigOptionalCommand is a command that runs with or without a project
// overplane.yaml. Commands that want the project config call project.Locate
// and project.Load themselves; a future config-required interface would have
// the dispatcher load and validate it before Run.
type ConfigOptionalCommand interface {
	Name() string
	Usage() string
	Run(ctx context.Context, args []string) error
}

type commandInfo struct {
	name string
	desc string
	cmd  any
}

func Dispatch(ctx context.Context, r *Runner, args []string) error {
	if r.In == nil {
		r.In = os.Stdin
	}
	if r.Out == nil {
		r.Out = os.Stdout
	}
	if r.Err == nil {
		r.Err = os.Stderr
	}
	registry := commandRegistry(r)
	if len(args) == 0 {
		printRootHelp(r.Out, registry)
		return UsageError("missing command")
	}
	if isHelpToken(args[0]) {
		printRootHelp(r.Out, registry)
		return nil
	}
	info, ok := registry[args[0]]
	if !ok {
		fmt.Fprintf(r.Err, "unknown command %q\n\n", args[0])
		printRootHelp(r.Err, registry)
		return UsageError("unknown command %q", args[0])
	}
	ctx, span := telemetrySpan(ctx, r, "cli."+args[0])
	defer span.End()
	if c, ok := info.cmd.(ConfigOptionalCommand); ok {
		return c.Run(ctx, args[1:])
	}
	return InternalError(fmt.Errorf("command %q has invalid type", args[0]))
}

func commandRegistry(r *Runner) map[string]commandInfo {
	return map[string]commandInfo{
		"init":    {name: "init", desc: "initialize an Overplane project", cmd: initCommand{r: r}},
		"setup":   {name: "setup", desc: "validate project and build agent image", cmd: setupCommand{r: r}},
		"shell":   {name: "shell", desc: "open ephemeral shell in agent image", cmd: shellCommand{r: r}},
		"version": {name: "version", desc: "print version information", cmd: versionCommand{r: r}},
		"check":   {name: "check", desc: "run local system checks", cmd: checkCommand{r: r}},
		"config":  {name: "config", desc: "validate repo config", cmd: configCommand{r: r}},
		"agent":   {name: "agent", desc: "manage the project's agent container", cmd: agentCommand{r: r}},
		"theme":   {name: "theme", desc: "terminal theme and brand identity", cmd: themeCommand{r: r}},
		"demo":    {name: "demo", desc: "run sample interactive TUI", cmd: demoCommand{r: r}},
	}
}

func printRootHelp(w io.Writer, registry map[string]commandInfo) {
	commands := make([]clihelp.Command, 0, len(registry))
	for _, info := range registry {
		commands = append(commands, clihelp.Command{Name: info.name, Description: info.desc})
	}
	clihelp.RenderRoot(w, clihelp.RootSpec{
		Binary:      Binary,
		Description: Description,
		Version:     version.Version,
		Runtime:     version.Runtime(),
		Metadata:    "build date " + version.BuildDate() + " commit " + version.Commit + executableNote(),
		Banner:      splashBytes(),
		Fallback:    "Overplane / Apache-2.0",
		Commands:    commands,
		Flags: []clihelp.Flag{
			{Name: "--log-format pretty|json", Description: "log format"},
			{Name: "--log-level debug|info|warn|error", Description: "log level"},
			{Name: "--log-file PATH", Description: "append logs to file"},
			{Name: "--log-timestamps=false", Description: "omit log timestamps"},
			{Name: "-v, --verbose", Description: "include provenance"},
			{Name: "--version", Description: "print version"},
		},
	})
}

// executableNote reports the on-disk size of the running binary as a friendly
// sentence; lookup failures simply omit the note.
func executableNote() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	st, err := os.Stat(exe)
	if err != nil {
		return ""
	}
	return fmt.Sprintf(". I am a %.1f MB executable.", float64(st.Size())/1e6)
}

func splashBytes() []byte {
	if b, err := assetfs.ReadFile("files/misc/banner.ans"); err == nil {
		return b
	}
	return nil
}

func isHelpToken(s string) bool {
	return s == "help" || s == "-h" || s == "--help"
}

func telemetrySpan(ctx context.Context, r *Runner, name string) (context.Context, interface{ End() }) {
	if r.Tel == nil {
		return ctx, noopSpan{}
	}
	ctx, sp := r.Tel.StartSpan(ctx, name)
	return ctx, otelSpan{sp: sp}
}

type noopSpan struct{}

func (noopSpan) End() {}

type otelSpan struct {
	sp trace.Span
}

func (s otelSpan) End() {
	s.sp.End()
}

func usage(spec color.HelpSpec) string {
	return color.RenderHelp(spec)
}

func wantsJSON(args []string) (bool, []string) {
	return wantsFlag(args, "--json")
}

func wantsFlag(args []string, name string) (bool, []string) {
	var out []string
	found := false
	for _, a := range args {
		if a == name {
			found = true
			continue
		}
		out = append(out, a)
	}
	return found, out
}
