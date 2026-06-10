package configloader_test

import (
	"errors"
	"testing"

	"github.com/overplane/overplane/internal/platform/configloader"
)

type sample struct {
	Name string `json:"name"`
}

const sampleSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "additionalProperties": false,
  "required": ["name"],
  "properties": {
    "name": { "type": "string" }
  }
}`

func TestLoadValidatesAndDecodes(t *testing.T) {
	cfg, err := configloader.Load[sample]("name: test\n", sampleSchema, "sample.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "test" {
		t.Fatalf("name = %q, want test", cfg.Name)
	}
}

func TestLoadRejectsSchemaProblems(t *testing.T) {
	_, err := configloader.Load[sample]("name: test\nextra: true\n", sampleSchema, "sample.schema.json")
	var validationErr configloader.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want ValidationError", err)
	}
}
