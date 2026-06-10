package configloader

import (
	"encoding/json"
	"fmt"

	"github.com/overplane/overplane/internal/platform/serde/jsonschema"
	"gopkg.in/yaml.v3"
)

// ValidationError groups schema validation problems into an error value.
type ValidationError struct {
	Problems []jsonschema.Problem
}

func (e ValidationError) Error() string {
	if len(e.Problems) == 0 {
		return "validation failed"
	}
	return e.Problems[0].Pointer + ": " + e.Problems[0].Message
}

// Load parses a YAML document, validates it against a JSON Schema document, and
// decodes the resulting JSON-compatible tree into the caller-provided type.
func Load[T any](yamlText, schemaText, schemaName string) (*T, error) {
	return LoadBytes[T]([]byte(yamlText), []byte(schemaText), schemaName)
}

// LoadBytes is the byte-slice form of Load.
func LoadBytes[T any](yamlData, schemaData []byte, schemaName string) (*T, error) {
	instance, err := parseYAML(yamlData)
	if err != nil {
		return nil, err
	}
	problems, err := validateInstance(instance, schemaData, schemaName)
	if err != nil {
		return nil, err
	}
	if len(problems) > 0 {
		return nil, ValidationError{Problems: problems}
	}
	var out T
	data, err := json.Marshal(instance)
	if err != nil {
		return nil, fmt.Errorf("encode validated config as json: %w", err)
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("decode validated config: %w", err)
	}
	return &out, nil
}

// Validate parses a YAML document and validates it against a JSON Schema
// document. It does not decode into a config-specific Go type.
func Validate(yamlText, schemaText, schemaName string) ([]jsonschema.Problem, error) {
	return ValidateBytes([]byte(yamlText), []byte(schemaText), schemaName)
}

// ValidateBytes is the byte-slice form of Validate.
func ValidateBytes(yamlData, schemaData []byte, schemaName string) ([]jsonschema.Problem, error) {
	instance, err := parseYAML(yamlData)
	if err != nil {
		return nil, err
	}
	return validateInstance(instance, schemaData, schemaName)
}

func parseYAML(data []byte) (any, error) {
	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return normalizeYAML(raw), nil
}

func validateInstance(instance any, schemaData []byte, schemaName string) ([]jsonschema.Problem, error) {
	var schemaDoc any
	if err := json.Unmarshal(schemaData, &schemaDoc); err != nil {
		return nil, fmt.Errorf("parse schema %s: %w", schemaName, err)
	}
	return jsonschema.Validate(schemaName, schemaDoc, instance)
}

func normalizeYAML(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, v := range x {
			out[k] = normalizeYAML(v)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(x))
		for k, v := range x {
			out[fmt.Sprint(k)] = normalizeYAML(v)
		}
		return out
	case []any:
		for i, v := range x {
			x[i] = normalizeYAML(v)
		}
		return x
	default:
		return v
	}
}
