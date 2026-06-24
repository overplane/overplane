// Package project owns the per-project overplane.yaml file: its embedded JSON
// Schema, default contents, discovery, and strict three-pass validation
// (schema, typed decode, semantic checks).
package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/overplane/overplane/internal/platform/configloader"
	assetfs "github.com/overplane/overplane/internal/platform/embed/assets"
	"github.com/overplane/overplane/internal/platform/serde/jsonschema"
	"github.com/overplane/overplane/internal/platform/serde/yamlcanon"
)

// FileName is the project file's fixed basename at the project root.
const FileName = "overplane.yaml"

// SchemaURL is the hosted schema location referenced by the editor pragma in
// generated files.
const SchemaURL = "https://overplane.dev/schema/overplane.schema.json"

const schemaAssetKey = "files/schema/overplane.schema.json"

// Config is the typed form of overplane.yaml.
type Config struct {
	SchemaVersion int           `json:"schema_version" yaml:"schema_version"`
	Project       ProjectInfo   `json:"project" yaml:"project"`
	Dirs          Dirs          `json:"dirs" yaml:"dirs"`
	Model         ModelSettings `json:"model,omitempty" yaml:"model"`
	Agent         *AgentSection `json:"agent,omitempty" yaml:"agent"`
}

// AgentSection configures the project's agent container subsystem.
type AgentSection struct {
	Container AgentContainer `json:"container" yaml:"container"`
}

// AgentContainer describes how the agent container image is built and run.
type AgentContainer struct {
	Runtime        string                 `json:"runtime" yaml:"runtime"`
	BaseImage      string                 `json:"base_image" yaml:"base_image"`
	SetupRecipe    string                 `json:"setup_recipe" yaml:"setup_recipe"`
	ExtraPackages  map[string][]string    `json:"extra_packages,omitempty" yaml:"extra_packages"`
	AgentRecipes   []AgentRecipeSelection `json:"agent_recipes" yaml:"agent_recipes"`
	EnvPassthrough []string               `json:"env_passthrough" yaml:"env_passthrough"`
}

// AgentRecipeSelection selects a registry agent recipe and a version (an npm
// dist-tag or exact version).
type AgentRecipeSelection struct {
	Name    string `json:"name" yaml:"name"`
	Version string `json:"version" yaml:"version"`
}

// ProjectInfo labels the project; it does not affect tool behavior.
type ProjectInfo struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description"`
}

// Dirs holds the Overplane-managed directory layout, relative to the project
// root (the directory containing overplane.yaml).
type Dirs struct {
	Specs string `json:"specs" yaml:"specs"`
	Cache string `json:"cache" yaml:"cache"`
}

// ModelSettings is a forward-looking stub for agent features.
type ModelSettings struct {
	Default string `json:"default,omitempty" yaml:"default"`
}

// SchemaBytes returns the embedded overplane.yaml JSON Schema.
func SchemaBytes() []byte {
	data, err := assetfs.ReadFile(schemaAssetKey)
	if err != nil {
		panic(fmt.Sprintf("project: embedded schema missing: %v", err))
	}
	return data
}

// Default builds the schema-default config with the project name overridden
// by the sanitized form of name (§2.1 of spec #0002). The defaults are
// assembled by walking the embedded schema's "properties" tree and collecting
// field-level "default" values, so the schema stays the single source of
// truth.
func Default(name string) (*Config, error) {
	defaults, err := jsonschema.Defaults(SchemaBytes())
	if err != nil {
		return nil, fmt.Errorf("parse embedded schema: %w", err)
	}
	raw, err := json.Marshal(defaults)
	if err != nil {
		return nil, fmt.Errorf("encode schema defaults: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("decode schema defaults: %w", err)
	}
	if n := SanitizeName(name); n != "" {
		cfg.Project.Name = n
	}
	return &cfg, nil
}

// YAMLDocumentOptions returns schema-driven layout options for emitting
// overplane.yaml from defaults (field descriptions as comments, schema key
// order, blank lines between top-level keys).
func YAMLDocumentOptions() (yamlcanon.DocumentOptions, error) {
	meta, err := jsonschema.DocumentMeta(SchemaBytes())
	if err != nil {
		return yamlcanon.DocumentOptions{}, fmt.Errorf("parse embedded schema metadata: %w", err)
	}
	return yamlcanon.DocumentOptions{
		Comments:           meta.Descriptions,
		KeyOrder:           meta.KeyOrder,
		TopLevelBlankLines: true,
	}, nil
}

// SanitizeName lowercases name and replaces every character outside
// [a-z0-9._-] with '-', trimming separators so the result satisfies the
// schema's name pattern. Returns "" when nothing usable remains.
func SanitizeName(name string) string {
	lower := strings.ToLower(name)
	mapped := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			return r
		default:
			return '-'
		}
	}, lower)
	return strings.Trim(mapped, "-._")
}

// Locate walks upward from startDir looking for overplane.yaml, for
// config-optional commands that want the nearest enclosing project.
func Locate(startDir string) (string, bool) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", false
	}
	for {
		p := filepath.Join(dir, FileName)
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

// Load reads, schema-validates, strictly decodes, and semantically validates
// the project file at path. Validation failures are returned as
// configloader.ValidationError so callers can print pointer-addressed
// problems uniformly.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg, err := configloader.LoadBytes[Config](data, SchemaBytes(), FileName)
	if err != nil {
		return nil, err
	}
	if problems := cfg.Validate(); len(problems) > 0 {
		return nil, configloader.ValidationError{Problems: problems}
	}
	if err := cfg.normalizeAgent(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// normalizeAgent fills the agent section in memory from the embedded schema's
// defaults so cfg.Agent.Container is always fully populated: pre-#0003 files
// without the key stay untouched on disk but behave exactly as if the default
// block were written out, and optional leaves (version, env_passthrough,
// extra_packages) get their documented defaults.
func (c *Config) normalizeAgent() error {
	def, err := Default("")
	if err != nil {
		return fmt.Errorf("assemble agent defaults: %w", err)
	}
	if c.Agent == nil {
		c.Agent = def.Agent
		return nil
	}
	ac := &c.Agent.Container
	dc := def.Agent.Container
	for i := range ac.AgentRecipes {
		if ac.AgentRecipes[i].Version == "" {
			ac.AgentRecipes[i].Version = defaultAgentVersion(dc, ac.AgentRecipes[i].Name)
		}
	}
	if ac.EnvPassthrough == nil {
		ac.EnvPassthrough = dc.EnvPassthrough
	}
	if ac.ExtraPackages == nil {
		ac.ExtraPackages = dc.ExtraPackages
	}
	return nil
}

func defaultAgentVersion(dc AgentContainer, name string) string {
	for _, r := range dc.AgentRecipes {
		if r.Name == name && r.Version != "" {
			return r.Version
		}
	}
	return "latest"
}

// Validate runs the semantic (third-pass) checks that the schema cannot
// express: the managed directories must be relative, must stay inside the
// project root, and must be distinct.
func (c *Config) Validate() []jsonschema.Problem {
	var problems []jsonschema.Problem
	specs := validateRelDir("/dirs/specs", c.Dirs.Specs, &problems)
	cache := validateRelDir("/dirs/cache", c.Dirs.Cache, &problems)
	if specs != "" && specs == cache {
		problems = append(problems, jsonschema.Problem{
			Pointer: "/dirs/cache",
			Message: "must differ from dirs.specs",
			Value:   c.Dirs.Cache,
		})
	}
	c.validateAgent(&problems)
	return problems
}

// validateAgent runs the semantic checks the schema cannot express on the
// agent section: duplicate agent recipe names and duplicate entries within
// each extra_packages list and env_passthrough.
func (c *Config) validateAgent(problems *[]jsonschema.Problem) {
	if c.Agent == nil {
		return
	}
	ac := c.Agent.Container
	seen := map[string]bool{}
	for i, r := range ac.AgentRecipes {
		if seen[r.Name] {
			*problems = append(*problems, jsonschema.Problem{
				Pointer: fmt.Sprintf("/agent/container/agent_recipes/%d/name", i),
				Message: "duplicate agent recipe name",
				Value:   r.Name,
			})
		}
		seen[r.Name] = true
	}
	for family, pkgs := range ac.ExtraPackages {
		if dup, ok := firstDuplicate(pkgs); ok {
			*problems = append(*problems, jsonschema.Problem{
				Pointer: "/agent/container/extra_packages/" + family,
				Message: "duplicate package name",
				Value:   dup,
			})
		}
	}
	if dup, ok := firstDuplicate(ac.EnvPassthrough); ok {
		*problems = append(*problems, jsonschema.Problem{
			Pointer: "/agent/container/env_passthrough",
			Message: "duplicate environment variable name",
			Value:   dup,
		})
	}
}

func firstDuplicate(items []string) (string, bool) {
	seen := map[string]bool{}
	for _, it := range items {
		if seen[it] {
			return it, true
		}
		seen[it] = true
	}
	return "", false
}

func validateRelDir(pointer, value string, problems *[]jsonschema.Problem) string {
	add := func(message string) {
		*problems = append(*problems, jsonschema.Problem{Pointer: pointer, Message: message, Value: value})
	}
	if strings.TrimSpace(value) == "" {
		add("must be a non-empty relative path")
		return ""
	}
	if filepath.IsAbs(value) {
		add("must be relative to the project root, not absolute")
		return ""
	}
	cleaned := filepath.Clean(value)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		add("must stay inside the project root (no .. segments, not the root itself)")
		return ""
	}
	return cleaned
}
