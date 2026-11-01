package cli

import (
	"encoding/json"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
)

// TestJSONOutputMatchesResultEnvelopeSchema validates what `--json` prints
// against schemas/result-envelope.schema.json, for a result that carries
// details and for an error.
func TestJSONOutputMatchesResultEnvelopeSchema(t *testing.T) {
	isolateInit(t)
	schema := loadResultEnvelopeSchema(t)
	hub := filepath.Join(t.TempDir(), "hub")

	code, stdout, stderr := runInitCLI(t, hub, "--json")
	if code != 0 {
		t.Fatalf("init --json: exit %d\n%s%s", code, stdout, stderr)
	}
	init := validateEnvelope(t, schema, stdout)
	if details, _ := init["details"].(map[string]any); details["init"] == nil {
		t.Fatalf("init --json carries no details.init:\n%s", stdout)
	}

	code, stdout, stderr = runCLI(t, "skill", "show", "missing-skill", "--workspace", hub, "--json")
	if code != 2 {
		t.Fatalf("skill show of a missing skill: exit %d\n%s%s", code, stdout, stderr)
	}
	refused := validateEnvelope(t, schema, stdout)
	if failure, _ := refused["error"].(map[string]any); failure["code"] != "not_found" {
		t.Fatalf("skill show of a missing skill returned:\n%s", stdout)
	}
}

func loadResultEnvelopeSchema(t *testing.T) *jsonschema.Resolved {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "schemas")
	read := func(name string) (*jsonschema.Schema, error) {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		var schema jsonschema.Schema
		if err := json.Unmarshal(data, &schema); err != nil {
			return nil, err
		}
		return &schema, nil
	}
	root, err := read("result-envelope.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	// The envelope refers to the error envelope beside it by a relative $ref.
	resolved, err := root.Resolve(&jsonschema.ResolveOptions{Loader: func(uri *url.URL) (*jsonschema.Schema, error) {
		return read(path.Base(uri.Path))
	}})
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func validateEnvelope(t *testing.T, schema *jsonschema.Resolved, output string) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal([]byte(output), &value); err != nil {
		t.Fatalf("output is not one JSON object: %v\n%s", err, output)
	}
	if err := schema.Validate(value); err != nil {
		t.Fatalf("output does not match the result envelope schema: %v\n%s", err, output)
	}
	return value
}
