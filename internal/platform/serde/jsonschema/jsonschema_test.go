package jsonschema_test

import (
	"encoding/json"
	"testing"

	jsvalidate "github.com/overplane/overplane/internal/platform/serde/jsonschema"
)

func TestValidate(t *testing.T) {
	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []any{"name"},
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
	}
	problems, err := jsvalidate.Validate("test.json", schema, map[string]any{"name": "ok"})
	if err != nil || len(problems) != 0 {
		t.Fatalf("valid: problems=%v err=%v", problems, err)
	}
	problems, err = jsvalidate.Validate("test.json", schema, map[string]any{"name": 1, "extra": true})
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) == 0 {
		t.Fatal("expected validation problems")
	}
}

func TestDefaults(t *testing.T) {
	for _, tc := range []struct {
		name   string
		schema string
		want   string
	}{
		{
			name: "field defaults assembled by property walk",
			schema: `{
				"type": "object",
				"default": {"ignored": true},
				"properties": {
					"name": {"type": "string", "default": "demo"},
					"tags": {"type": "array", "default": ["a"]},
					"nested": {
						"type": "object",
						"default": {"also": "ignored"},
						"properties": {
							"level": {"type": "integer", "default": 3},
							"nodefault": {"type": "string"}
						}
					},
					"empty": {"type": "object", "properties": {"x": {"type": "string"}}}
				}
			}`,
			want: `{"name":"demo","nested":{"level":3},"tags":["a"]}`,
		},
		{
			name:   "no properties yields empty object",
			schema: `{"type": "object", "default": {"ignored": 1}}`,
			want:   `{}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := jsvalidate.Defaults([]byte(tc.schema))
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != tc.want {
				t.Fatalf("Defaults = %s, want %s", data, tc.want)
			}
		})
	}
}
