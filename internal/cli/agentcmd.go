package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	osuser "os/user"
	"strings"

	"github.com/overplane/overplane/internal/container"
	"github.com/overplane/overplane/internal/platform/color"
	"github.com/overplane/overplane/internal/platform/configloader"
	"github.com/overplane/overplane/internal/project"
	"github.com/overplane/overplane/internal/recipes"
)

// agentCommand is the `overplane agent` group: setup, shell, list-images.
// Every subcommand is config-required in semantics: it locates and fully
// validates overplane.yaml first (exit 6 with an init hint when absent).
type agentCommand struct{ r *Runner }

func (c agentCommand) Name() string  { return "agent" }
func (c agentCommand) Usage() string { return Binary + " agent <subcommand> [flags]" }

func agentGroupHelp() color.HelpSpec {
	return color.HelpSpec{
		Command: "agent",
		Usage:   Binary + " agent <subcommand> [flags]",
		Description: "Manage the project's agent container: a local, project-labeled image carrying " +
			"a full developer toolchain and the AI coding agents configured in overplane.yaml. " +
			"Subcommands: setup (build or reuse the image), shell (open an ephemeral interactive " +
			"shell inside it), list-images (list this project's built images).",
		Examples: []string{
			Binary + " agent setup",
			Binary + " agent shell",
			Binary + " agent list-images --json",
		},
	}
}

func (c agentCommand) Run(ctx context.Context, args []string) error {
	return runSubcommandGroup(ctx, args, c.r.Err, "agent", true, func() {
		fmt.Fprint(c.r.Out, usage(agentGroupHelp()))
	}, map[string]subcommandHandler{
		"setup":       c.setup,
		"shell":       c.shell,
		"list-images": c.listImages,
	})
}

// requireProject locates and fully validates the nearest overplane.yaml
// (schema, semantic, and registry passes). No project file maps to exit 6
// with the init hint; validation failures print pointer-addressed problems
// and map to exit 3.
func requireProject(r *Runner) (*project.Config, string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, "", IOError(err)
	}
	path, ok := project.Locate(wd)
	if !ok {
		return nil, "", EnvError(fmt.Errorf(
			"no %s found in %s or any parent; run '%s init' first", project.FileName, wd, Binary))
	}
	cfg, err := recipes.LoadProjectConfig(path)
	if err != nil {
		var ve configloader.ValidationError
		if errors.As(err, &ve) {
			for _, p := range ve.Problems {
				fmt.Fprintf(r.Err, "%s: %s\n", p.Pointer, p.Message)
			}
			return nil, "", ValidationError(fmt.Errorf("%s failed validation; fix the problems above", path))
		}
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			return nil, "", IOError(err)
		}
		return nil, "", ValidationError(err)
	}
	return cfg, path, nil
}

// loadRegistry returns the embedded recipe registry; a failure means the
// shipped binary is broken (exit 5).
func loadRegistry() (*recipes.Registry, error) {
	reg, err := recipes.Load()
	if err != nil {
		return nil, InternalError(fmt.Errorf("embedded recipe registry: %w", err))
	}
	return reg, nil
}

// agentEngineClientHook, when set by tests, replaces subprocess engine
// discovery with an in-process fake so CI runners never hit a real daemon.
var agentEngineClientHook func(ctx context.Context, runtimeName string) (container.Client, error)

// agentEngineClient builds the configured engine client and verifies the
// engine is reachable and new enough; failures map to exit 6 with the
// engine's hint.
func agentEngineClient(ctx context.Context, runtimeName string) (container.Client, error) {
	if agentEngineClientHook != nil {
		return agentEngineClientHook(ctx, runtimeName)
	}
	engine, err := container.ParseEngine(runtimeName)
	if err != nil {
		return nil, InternalError(err) // schema + registry validation make this unreachable
	}
	client, err := container.New(ctx, engine, engine)
	if err != nil {
		return nil, InternalError(err)
	}
	if err := client.Available(ctx); err != nil {
		return nil, EnvError(withHint(err))
	}
	return client, nil
}

// withHint folds a Hint()-carrying error's remediation into its message so
// it survives the plain error rendering at the top level.
func withHint(err error) error {
	var h interface{ Hint() string }
	if errors.As(err, &h) && h.Hint() != "" {
		return fmt.Errorf("%w (hint: %s)", err, h.Hint())
	}
	return err
}

// hostIdentity captures the host user baked into the image and used to run
// shell containers.
type hostIdentity struct {
	UID      string
	GID      string
	Username string
	Home     string
}

func currentHostIdentity() hostIdentity {
	id := hostIdentity{UID: "1000", GID: "1000", Username: "overplane", Home: "/home/overplane"}
	u, err := osuser.Current()
	if err != nil {
		return id
	}
	if u.Uid != "" {
		id.UID = u.Uid
	}
	if u.Gid != "" {
		id.GID = u.Gid
	}
	if u.Username != "" {
		name := u.Username
		if idx := strings.LastIndex(name, `\`); idx >= 0 {
			name = name[idx+1:]
		}
		id.Username = name
	}
	if u.HomeDir != "" {
		id.Home = u.HomeDir
	}
	return id
}

// shortImageID renders an image id in the familiar 12-hex form.
func shortImageID(id string) string {
	id = strings.TrimPrefix(id, "sha256:")
	if len(id) > 12 {
		id = id[:12]
	}
	return id
}

// formatImageSize renders a byte count in engine-style human units.
func formatImageSize(size int64) string {
	const unit = 1000.0
	switch f := float64(size); {
	case f >= unit*unit*unit:
		return fmt.Sprintf("%.2fGB", f/(unit*unit*unit))
	case f >= unit*unit:
		return fmt.Sprintf("%.1fMB", f/(unit*unit))
	case f >= unit:
		return fmt.Sprintf("%.0fkB", f/unit)
	default:
		return fmt.Sprintf("%dB", size)
	}
}

// agentSummary renders the configured agents as name@version, in file order.
func agentSummary(cfg project.AgentContainer) string {
	if len(cfg.AgentRecipes) == 0 {
		return "(none)"
	}
	parts := make([]string, 0, len(cfg.AgentRecipes))
	for _, sel := range cfg.AgentRecipes {
		parts = append(parts, sel.Name+"@"+sel.Version)
	}
	return strings.Join(parts, ", ")
}
