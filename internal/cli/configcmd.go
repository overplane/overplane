package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/overplane/overplane/internal/platform/color"
	"github.com/overplane/overplane/internal/platform/configloader"
	"github.com/overplane/overplane/internal/platform/paths"
	"github.com/overplane/overplane/internal/project"
	"github.com/overplane/overplane/internal/recipes"
)

type configCommand struct{ r *Runner }

func (c configCommand) Name() string  { return "config" }
func (c configCommand) Usage() string { return Binary + " config validate [path]" }

func (c configCommand) Run(ctx context.Context, args []string) error {
	return runSubcommandGroup(ctx, args, c.r.Err, "config", true, func() {
		fmt.Fprint(c.r.Out, usage(color.HelpSpec{
			Command: "config",
			Usage:   Binary + " config validate [path]",
			Description: "Validate repo config files, a project overplane.yaml, or a recipes.yaml " +
				"registry (checked against the embedded recipes schema plus structural cross-checks).",
			Examples: []string{
				Binary + " config validate",
				Binary + " config validate config/data/theme.yaml",
				Binary + " config validate config/data/recipes.yaml",
				Binary + " config validate overplane.yaml",
			},
		}))
	}, map[string]subcommandHandler{"validate": c.validate})
}

func (c configCommand) validate(_ context.Context, args []string) error {
	if len(args) > 0 && isHelpToken(args[0]) {
		fmt.Fprint(c.r.Out, usage(color.HelpSpec{
			Command:     "config validate",
			Usage:       Binary + " config validate [path]",
			Description: "Run JSON Schema validation for repo config.",
		}))
		return nil
	}
	if len(args) > 1 {
		return UsageError("config validate accepts at most one path")
	}
	if len(args) == 1 {
		return c.validatePath(args[0])
	}
	p, err := paths.Resolve("")
	if err != nil {
		return IOError(err)
	}
	files := []string{p.GlobalFile, p.ThemeFile, p.InfraFile}
	// The recipes registry mirror is optional in checkouts; validate it only
	// when present.
	recipesFile := filepath.Join(p.ConfigDataDir, recipes.RegistryFileName)
	if _, err := os.Stat(recipesFile); err == nil {
		files = append(files, recipesFile)
	}
	for _, path := range files {
		if err := c.validatePath(path); err != nil {
			return err
		}
	}
	return nil
}

func (c configCommand) validatePath(path string) error {
	if err := validateConfigPath(path); err != nil {
		var ve configloader.ValidationError
		if errors.As(err, &ve) {
			for _, p := range ve.Problems {
				fmt.Fprintf(c.r.Err, "%s: %s\n", p.Pointer, p.Message)
			}
		}
		return ValidationError(err)
	}
	fmt.Fprintf(c.r.Out, "%s valid\n", path)
	return nil
}

func validateConfigPath(path string) error {
	// The project file validates against the embedded schema (plus the
	// semantic pass) so it works outside the monorepo.
	if filepath.Base(path) == project.FileName {
		_, err := project.Load(path)
		return err
	}
	// The recipe registry validates against the embedded recipes schema plus
	// the structural self-check, so drifted mirrors are caught everywhere.
	if filepath.Base(path) == recipes.RegistryFileName {
		return validateRecipesPath(path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	p, err := paths.Resolve(path)
	if err != nil {
		return err
	}
	schemaPath, err := schemaForPath(p, path)
	if err != nil {
		return err
	}
	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		return err
	}
	problems, err := configloader.ValidateBytes(data, schema, schemaPath)
	if err != nil {
		return err
	}
	if len(problems) > 0 {
		return configloader.ValidationError{Problems: problems}
	}
	return nil
}

func validateRecipesPath(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	problems, err := recipes.ValidateRegistryBytes(data)
	if err != nil {
		return err
	}
	if len(problems) > 0 {
		return configloader.ValidationError{Problems: problems}
	}
	return nil
}

func schemaForPath(p *paths.Paths, path string) (string, error) {
	switch filepath.Base(path) {
	case "global.yaml":
		return p.GlobalSchema, nil
	case "theme.yaml":
		return p.ThemeSchema, nil
	case "infra.yaml":
		return p.InfraSchema, nil
	default:
		return "", fmt.Errorf("unsupported config file %q", path)
	}
}
