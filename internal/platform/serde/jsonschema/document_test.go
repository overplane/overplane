package jsonschema_test

import (
	"strings"
	"testing"

	jsvalidate "github.com/overplane/overplane/internal/platform/serde/jsonschema"
	"github.com/overplane/overplane/internal/project"
)

func TestDocumentMetaOverplaneSchema(t *testing.T) {
	meta, err := jsvalidate.DocumentMeta(project.SchemaBytes())
	if err != nil {
		t.Fatal(err)
	}
	if meta.Descriptions["schema_version"] == "" {
		t.Fatal("missing schema_version description")
	}
	if meta.Descriptions["project.name"] == "" {
		t.Fatal("missing project.name description")
	}
	if meta.Descriptions["agent.container.agent_recipes.name"] == "" {
		t.Fatal("missing agent recipe name description")
	}
	wantRoot := []string{"schema_version", "project", "dirs", "model", "agent"}
	got := meta.KeyOrder[""]
	if len(got) != len(wantRoot) {
		t.Fatalf("root key order = %v, want %v", got, wantRoot)
	}
	for i, k := range wantRoot {
		if got[i] != k {
			t.Fatalf("root key order = %v, want %v", got, wantRoot)
		}
	}
	if !strings.Contains(meta.Descriptions["dirs.cache"], "cache") {
		t.Fatalf("unexpected dirs.cache description: %q", meta.Descriptions["dirs.cache"])
	}
}
