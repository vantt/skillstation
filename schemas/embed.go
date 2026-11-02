// Package schemas exposes committed public JSON Schemas to runtime discovery.
package schemas

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
)

//go:embed skill-resolve-request-v1.schema.json
var resolverRequestJSON []byte

//go:embed skill-resolve-response-v1.schema.json
var resolverResponseJSON []byte

func ResolverRequest() (*jsonschema.Schema, error) {
	return decode("resolver request", resolverRequestJSON)
}

func ResolverResponse() (*jsonschema.Schema, error) {
	return decode("resolver response", resolverResponseJSON)
}

func decode(name string, data []byte) (*jsonschema.Schema, error) {
	var schema jsonschema.Schema
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("decode committed %s schema: %w", name, err)
	}
	return &schema, nil
}
