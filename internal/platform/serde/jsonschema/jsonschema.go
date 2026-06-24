package jsonschema

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	js "github.com/santhosh-tekuri/jsonschema/v6"
)

type Problem struct {
	Pointer string
	Message string
	Value   any
}

func Validate(schemaName string, schemaDoc, instance any) ([]Problem, error) {
	compiler := js.NewCompiler()
	schemaBytes, err := json.Marshal(schemaDoc)
	if err != nil {
		return nil, err
	}
	var decodedSchema any
	if err := json.Unmarshal(schemaBytes, &decodedSchema); err != nil {
		return nil, err
	}
	if err := compiler.AddResource(schemaName, decodedSchema); err != nil {
		return nil, err
	}
	schema, err := compiler.Compile(schemaName)
	if err != nil {
		return nil, err
	}
	if err := schema.Validate(instance); err != nil {
		return collectProblems(err, instance), nil
	}
	return nil, nil
}

func collectProblems(err error, instance any) []Problem {
	var ve *js.ValidationError
	if errors.As(err, &ve) {
		if len(ve.Causes) > 0 {
			out := make([]Problem, 0, len(ve.Causes))
			for _, cause := range ve.Causes {
				out = append(out, collectProblems(cause, instance)...)
			}
			return out
		}
		ptr := pointer(ve.InstanceLocation)
		msg := ve.Error()
		if out := ve.BasicOutput(); out != nil && out.Error != nil {
			msg = out.Error.String()
		}
		return []Problem{{Pointer: ptr, Message: msg, Value: valueAt(instance, ptr)}}
	}
	return []Problem{{Pointer: "/", Message: err.Error(), Value: instance}}
}

func valueAt(v any, ptr string) any {
	if ptr == "" || ptr == "/" {
		return v
	}
	return nil
}

// Defaults assembles a schema's default instance by walking down
// "properties": an object subschema contributes the assembly of its own
// properties (omitted when empty), and any other subschema contributes its
// field-level "default" value when present. Root-level "default" blocks are
// intentionally ignored; field-level defaults are the single source of truth.
func Defaults(schemaJSON []byte) (map[string]any, error) {
	var doc map[string]any
	if err := json.Unmarshal(schemaJSON, &doc); err != nil {
		return nil, err
	}
	out := assembleDefaults(doc)
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}

func assembleDefaults(node map[string]any) map[string]any {
	props, ok := node["properties"].(map[string]any)
	if !ok {
		return nil
	}
	out := map[string]any{}
	for name, raw := range props {
		child, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if _, nested := child["properties"]; nested {
			if v := assembleDefaults(child); len(v) > 0 {
				out[name] = v
			}
			continue
		}
		if def, ok := child["default"]; ok {
			out[name] = def
		}
	}
	return out
}

func pointer(parts []string) string {
	if len(parts) == 0 {
		return "/"
	}
	escaped := make([]string, len(parts))
	for i, p := range parts {
		if _, err := strconv.Atoi(p); err == nil {
			escaped[i] = p
			continue
		}
		p = strings.ReplaceAll(p, "~", "~0")
		p = strings.ReplaceAll(p, "/", "~1")
		escaped[i] = p
	}
	return "/" + strings.Join(escaped, "/")
}
