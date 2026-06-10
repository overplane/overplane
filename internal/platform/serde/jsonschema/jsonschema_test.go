package jsonschema_test

import (
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
