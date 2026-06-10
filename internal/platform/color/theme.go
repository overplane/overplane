package color

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/overplane/overplane/internal/platform/configloader"
	"github.com/overplane/overplane/internal/platform/paths"
	"gopkg.in/yaml.v3"
)

const EnvPrefix = "OVERPLANE"

type LogSlots struct {
	Error *int `json:"error,omitempty" yaml:"error,omitempty"`
	Warn  *int `json:"warn,omitempty" yaml:"warn,omitempty"`
	Info  *int `json:"info,omitempty" yaml:"info,omitempty"`
	Debug *int `json:"debug,omitempty" yaml:"debug,omitempty"`
}

type Theme struct {
	Name    string   `json:"name" yaml:"name"`
	Palette Palette  `json:"palette" yaml:"palette"`
	Log     LogSlots `json:"log,omitempty" yaml:"log,omitempty"`
}

type ThemeResolution struct {
	Theme  Theme
	Source string
}

func ResolveTheme(startDir string) (ThemeResolution, error) {
	if p := os.Getenv(EnvPrefix + "_THEME"); p != "" {
		res, err := loadThemeFile(p)
		if err != nil {
			return ThemeResolution{}, err
		}
		return ThemeResolution{Theme: res, Source: p}, nil
	}
	if startDir == "" {
		startDir, _ = os.Getwd()
	}
	if p, ok := findTheme(startDir); ok {
		res, err := loadThemeFile(p)
		if err == nil {
			return ThemeResolution{Theme: res, Source: p}, nil
		}
	}
	return ThemeResolution{Theme: DefaultTerminalTheme(), Source: "built-in default"}, nil
}

func ApplyResolvedTheme(startDir string) (ThemeResolution, error) {
	res, err := ResolveTheme(startDir)
	if err != nil {
		return res, err
	}
	Set(res.Theme.Palette)
	return res, nil
}

func findTheme(startDir string) (string, bool) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", false
	}
	for {
		p := filepath.Join(dir, "config", "data", "theme.yaml")
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func loadThemeFile(path string) (Theme, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Theme{}, err
	}
	if p, err := paths.Resolve(path); err == nil {
		schema, err := os.ReadFile(p.ThemeSchema)
		if err == nil {
			problems, err := configloader.ValidateBytes(data, schema, p.ThemeSchema)
			if err != nil {
				return Theme{}, err
			}
			if len(problems) > 0 {
				return Theme{}, fmt.Errorf("%s: theme validation failed: %s", path, problems[0])
			}
		}
	}
	var doc struct {
		Terminal Theme `yaml:"terminal"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return Theme{}, err
	}
	if err := doc.Terminal.Validate(); err != nil {
		return Theme{}, fmt.Errorf("%s: terminal theme validation failed: %w", path, err)
	}
	return doc.Terminal, nil
}

func (t Theme) Validate() error {
	if t.Name == "" {
		return fmt.Errorf("terminal.name is required")
	}
	if t.Palette == (Palette{}) {
		return fmt.Errorf("terminal.palette must define 16 xterm colors")
	}
	return nil
}

func DefaultTerminalTheme() Theme {
	return Theme{
		Name:    "overplane-autumn",
		Palette: Palette{214, 130, 166, 64, 67, 136, 95, 244, 180, 94, 101, 172, 173, 58, 230, 238},
	}
}
