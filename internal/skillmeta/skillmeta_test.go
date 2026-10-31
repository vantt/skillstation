package skillmeta

import (
	"io/fs"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// mapReader reads paths from an in-memory file map.
func mapReader(files map[string][]byte) ReadFunc {
	return func(rel string) ([]byte, error) {
		contents, ok := files[rel]
		if !ok {
			return nil, fs.ErrNotExist
		}
		return contents, nil
	}
}

func decodeMap(t *testing.T, contents []byte) map[string]any {
	t.Helper()
	var values map[string]any
	if err := yaml.Unmarshal(contents, &values); err != nil {
		t.Fatalf("parse %q: %v", contents, err)
	}
	return values
}

func keysOf(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

const mergedFixture = `schema_version: 1
id: review
collection: core
collection_id: core
name: Review
description: Review code.
# The skill's lifecycle state.
status: active
# How agents find the skill.
routing:
    triggers: [review code] # the main trigger
labels: [favorite]
quality:
    reviewed: true
created_at: "2026-10-01T00:00:00Z"
surprise: kept for validation
`

func TestSplitSendsEachKeyToItsFileAndDropsDerivedKeys(t *testing.T) {
	settings, hub, err := Split([]byte(mergedFixture))
	if err != nil {
		t.Fatal(err)
	}
	settingsValues, hubValues := decodeMap(t, settings), decodeMap(t, hub)
	for _, key := range []string{"routing", "labels"} {
		if _, ok := settingsValues[key]; !ok {
			t.Errorf("skill.yaml lacks %s:\n%s", key, settings)
		}
	}
	for _, key := range []string{"status", "quality", "created_at", "surprise"} {
		if _, ok := hubValues[key]; !ok {
			t.Errorf("hub.yaml lacks %s:\n%s", key, hub)
		}
	}
	for _, key := range []string{"id", "collection", "collection_id", "name", "description"} {
		if _, ok := settingsValues[key]; ok {
			t.Errorf("skill.yaml keeps derived key %s", key)
		}
		if _, ok := hubValues[key]; ok {
			t.Errorf("hub.yaml keeps derived key %s", key)
		}
	}
	for name, values := range map[string]map[string]any{SettingsFile: settingsValues, HubFile: hubValues} {
		if values["schema_version"] != 1 {
			t.Errorf("%s schema_version = %v", name, values["schema_version"])
		}
		for _, key := range keysOf(values) {
			if key == "schema_version" || key == "surprise" {
				continue
			}
			if IsSettingsKey(key) != (name == SettingsFile) {
				t.Errorf("%s holds %s", name, key)
			}
		}
	}
	for file, comment := range map[string]string{string(settings): "# How agents find the skill.", string(hub): "# The skill's lifecycle state."} {
		if !strings.Contains(file, comment) {
			t.Errorf("comment %q was lost:\n%s", comment, file)
		}
	}
	if !strings.Contains(string(settings), "# the main trigger") {
		t.Errorf("line comment inside routing was lost:\n%s", settings)
	}
}

func TestMergeAndSplitRoundTrip(t *testing.T) {
	settings, hub, err := Split([]byte(mergedFixture))
	if err != nil {
		t.Fatal(err)
	}
	merged, err := Merge(settings, hub, "review")
	if err != nil {
		t.Fatal(err)
	}
	values := decodeMap(t, merged)
	if values["schema_version"] != 1 || values["id"] != "review" {
		t.Fatalf("merged header = %v, %v", values["schema_version"], values["id"])
	}
	if !strings.HasPrefix(string(merged), "schema_version: 1\nid: review\n") {
		t.Fatalf("merged document does not start with schema_version and id:\n%s", merged)
	}
	want := decodeMap(t, []byte(mergedFixture))
	for _, key := range []string{"collection", "collection_id", "name", "description"} {
		delete(want, key)
	}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("merged = %#v\nwant %#v", values, want)
	}
	for _, comment := range []string{"# How agents find the skill.", "# The skill's lifecycle state.", "# the main trigger"} {
		if !strings.Contains(string(merged), comment) {
			t.Errorf("comment %q was lost in the merge:\n%s", comment, merged)
		}
	}

	settingsAgain, hubAgain, err := Split(merged)
	if err != nil {
		t.Fatal(err)
	}
	if string(settingsAgain) != string(settings) || string(hubAgain) != string(hub) {
		t.Fatalf("split after merge differs:\n%s\n%s\nwant\n%s\n%s", settingsAgain, hubAgain, settings, hub)
	}
}

func TestMergeTakesTheIDFromTheFolderNotTheFiles(t *testing.T) {
	merged, err := Merge([]byte("schema_version: 1\nid: stray\n"), []byte("schema_version: 1\nid: other\nstatus: draft\n"), "review")
	if err != nil {
		t.Fatal(err)
	}
	if values := decodeMap(t, merged); values["id"] != "review" || values["status"] != "draft" {
		t.Fatalf("merged = %s", merged)
	}
	if strings.Count(string(merged), "id:") != 1 {
		t.Fatalf("merged keeps a second id:\n%s", merged)
	}
}

func TestLocateLooksForHubYAMLFirst(t *testing.T) {
	const dir = "skills/core/review"
	files := map[string][]byte{
		dir + "/" + HubFile:    []byte("schema_version: 1\nstatus: draft\n"),
		dir + "/" + LegacyFile: []byte("schema_version: 1\nid: review\n"),
		dir + "/" + V2File:     []byte("schema_version: 1\nid: review\n"),
	}
	source, found, err := Locate(mapReader(files), dir)
	if err != nil || !found {
		t.Fatalf("Locate: found=%v err=%v", found, err)
	}
	if !source.Split() || source.Anchor() != dir+"/"+HubFile || !reflect.DeepEqual(source.Paths, []string{dir + "/" + HubFile, dir + "/" + SettingsFile}) {
		t.Fatalf("source = %#v", source)
	}
}

func TestLocateOrderOfEarlierLayouts(t *testing.T) {
	const dir = "skills/core/review"
	cases := map[string]struct {
		marker string
		want   string
		layout int
	}{
		"layout 2 reads skill.meta.yaml first":   {marker: "2\n", want: dir + "/" + V2File, layout: 2},
		"layout 3 reads .meta/skill.yaml first":  {marker: "3\n", want: dir + "/" + LegacyFile, layout: 3},
		"layout 5 reads .meta/skill.yaml first":  {marker: "5\n", want: dir + "/" + LegacyFile, layout: 3},
		"no marker reads .meta/skill.yaml first": {want: dir + "/" + LegacyFile, layout: 3},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			files := map[string][]byte{
				dir + "/" + LegacyFile: []byte("schema_version: 1\nid: review\n"),
				dir + "/" + V2File:     []byte("schema_version: 1\nid: review\n"),
			}
			if tc.marker != "" {
				files[".skillstation/schema-version"] = []byte(tc.marker)
			}
			source, found, err := Locate(mapReader(files), dir)
			if err != nil || !found {
				t.Fatalf("Locate: found=%v err=%v", found, err)
			}
			if source.Anchor() != tc.want || source.Layout != tc.layout || source.Split() {
				t.Fatalf("source = %#v, want anchor %s at layout %d", source, tc.want, tc.layout)
			}
		})
	}
}

func TestLocateReportsAFolderWithoutMetadata(t *testing.T) {
	source, found, err := Locate(mapReader(map[string][]byte{"skills/core/review/SKILL.md": []byte("# Review\n")}), "skills/core/review")
	if err != nil || found || source.Anchor() != "" {
		t.Fatalf("Locate = %#v, %v, %v", source, found, err)
	}
}

func TestReadSplitSkillWithoutSkillYAML(t *testing.T) {
	const dir = "skills/core/review"
	files := map[string][]byte{
		dir + "/" + HubFile: []byte("schema_version: 1\nstatus: draft\nquality:\n    reviewed: false\n"),
	}
	merged, source, found, err := Read(mapReader(files), dir)
	if err != nil || !found {
		t.Fatalf("Read: found=%v err=%v", found, err)
	}
	if !source.Split() {
		t.Fatalf("source = %#v", source)
	}
	values := decodeMap(t, merged)
	want := map[string]any{"schema_version": 1, "id": "review", "status": "draft", "quality": map[string]any{"reviewed": false}}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("merged = %#v\nwant %#v", values, want)
	}
}

func TestReadLegacySkillReturnsItsFileAsItIs(t *testing.T) {
	const dir = "skills/core/review"
	stored := []byte("schema_version: 1\nid: review\nname: Review\nstatus: draft\n")
	merged, source, found, err := Read(mapReader(map[string][]byte{dir + "/" + LegacyFile: stored}), dir)
	if err != nil || !found || source.Split() || string(merged) != string(stored) {
		t.Fatalf("Read = %q, %#v, %v, %v", merged, source, found, err)
	}
}

func TestFilesFollowTheLayout(t *testing.T) {
	const dir = "skills/core/review"
	merged := []byte("schema_version: 1\nid: review\nstatus: draft\nlabels: [a]\n")
	for layout, want := range map[int][]string{
		SplitLayout: {dir + "/" + HubFile, dir + "/" + SettingsFile},
		5:           {dir + "/" + LegacyFile},
		3:           {dir + "/" + LegacyFile},
		2:           {dir + "/" + V2File},
	} {
		files, err := Files(dir, layout, merged)
		if err != nil {
			t.Fatal(err)
		}
		var paths []string
		for _, file := range files {
			paths = append(paths, file.Path)
		}
		if !reflect.DeepEqual(paths, want) {
			t.Errorf("layout %d files = %v, want %v", layout, paths, want)
		}
	}
}

func TestValidSkillID(t *testing.T) {
	t.Parallel()
	for id, want := range map[string]bool{
		"test-audit": true, "a": true, "0x": true, "a-": true, strings.Repeat("a", 63): true,
		"": false, "-a": false, "A": false, "a_b": false, "a.b": false, "a/b": false, "..": false,
		"a b": false, "é": false, strings.Repeat("a", 64): false,
	} {
		if got := ValidSkillID(id); got != want {
			t.Errorf("ValidSkillID(%q) = %v; want %v", id, got, want)
		}
	}
}
