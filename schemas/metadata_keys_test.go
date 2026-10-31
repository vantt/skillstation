package schemas

import (
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/vantt/skillstation/internal/skillmeta"
)

// The metadata keys live in Go (where each key is written) and in the public
// schemas; a key added to one must reach the other.
func TestMetadataSchemasListTheKeysSkillmetaWrites(t *testing.T) {
	for file, keys := range map[string][]string{
		"skill-settings.schema.json": skillmeta.SettingsKeys(),
		"skill-hub.schema.json":      skillmeta.HubKeys(),
		"skill-metadata.schema.json": skillmeta.MetadataKeys(),
	} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var schema struct {
			Properties map[string]json.RawMessage `json:"properties"`
		}
		if err := json.Unmarshal(data, &schema); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		var properties []string
		for key := range schema.Properties {
			properties = append(properties, key)
		}
		slices.Sort(properties)
		want := slices.Sorted(slices.Values(keys))
		if !slices.Equal(properties, want) {
			t.Errorf("%s properties = %v; skillmeta keys = %v", file, properties, want)
		}
	}
}
