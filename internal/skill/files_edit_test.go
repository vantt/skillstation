package skill

import (
	"errors"
	"strings"
	"testing"
)

const editFixtureSkillMD = "---\nname: team-release\ndescription: Cut a team release.\n---\n\n# Team release\n\nSteps.\n"

const editFixtureMetadata = `schema_version: 1
# routing the author keeps
routing:
  triggers:
    - cut a team release
  examples:
    - cut the release
  not_for:
    - deploying
  operations: []
  min_scope: ""
labels:
  - ops
`

func editFixture() map[string][]byte {
	return map[string][]byte{"SKILL.md": []byte(editFixtureSkillMD), "skill.yaml": []byte(editFixtureMetadata), "scripts/bump.sh": []byte("echo 1\n")}
}

func TestEditFilesReplacesRoutingAndKeepsEverythingElse(t *testing.T) {
	t.Parallel()
	files := editFixture()
	routing := RoutingOfFiles(files)
	routing.Triggers = []string{"cut a team release", "tag and publish a release"}
	routing.Examples = []string{"cut the release", "tag the release and publish it", "publish this week's release"}
	edited, err := EditFiles(files, "team-release", UpdateInput{Routing: &routing})
	if err != nil {
		t.Fatal(err)
	}
	metadata := string(edited["skill.yaml"])
	for _, want := range []string{"# routing the author keeps", "tag and publish a release", "publish this week's release", "deploying", "labels:", "- ops"} {
		if !strings.Contains(metadata, want) {
			t.Errorf("skill.yaml lost or missed %q:\n%s", want, metadata)
		}
	}
	if string(edited["SKILL.md"]) != editFixtureSkillMD || string(edited["scripts/bump.sh"]) != "echo 1\n" {
		t.Errorf("an edit of routing touched another file: %q", edited["SKILL.md"])
	}
	if string(files["skill.yaml"]) != editFixtureMetadata {
		t.Error("the input files were changed")
	}
}

func TestEditFilesWithNothingToChangeChangesNothing(t *testing.T) {
	t.Parallel()
	files := editFixture()
	routing := RoutingOfFiles(files)
	edited, err := EditFiles(files, "team-release", UpdateInput{Routing: &routing})
	if err != nil {
		t.Fatal(err)
	}
	for path, before := range files {
		if string(edited[path]) != string(before) {
			t.Errorf("%s changed though the routing is the same:\n%s", path, edited[path])
		}
	}
}

func TestEditFilesPutsTheDescriptionInSkillMDOnly(t *testing.T) {
	t.Parallel()
	description := "Cut and publish a team release. Use when the user wants a release."
	edited, err := EditFiles(editFixture(), "team-release", UpdateInput{Description: &description})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(edited["SKILL.md"]), "description: "+description) || !strings.HasSuffix(string(edited["SKILL.md"]), "# Team release\n\nSteps.\n") {
		t.Errorf("SKILL.md:\n%s", edited["SKILL.md"])
	}
	if string(edited["skill.yaml"]) != editFixtureMetadata {
		t.Errorf("skill.yaml gained a description it did not have:\n%s", edited["skill.yaml"])
	}
	named := editFixture()
	named["skill.yaml"] = []byte(editFixtureMetadata + "description: Cut a team release.\n")
	edited, err = EditFiles(named, "team-release", UpdateInput{Description: &description})
	if err != nil || !strings.Contains(string(edited["skill.yaml"]), "description: "+description) {
		t.Errorf("a skill.yaml that names a description keeps it in step: %v\n%s", err, edited["skill.yaml"])
	}
}

func TestEditFilesCreatesSkillYAMLForAFolderWithoutOne(t *testing.T) {
	t.Parallel()
	files := map[string][]byte{"SKILL.md": []byte(editFixtureSkillMD)}
	edited, err := EditFiles(files, "team-release", UpdateInput{Routing: &RoutingInput{Triggers: []string{"cut a team release"}, Examples: []string{"cut the release"}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(edited["skill.yaml"]); !strings.Contains(got, "schema_version: 1") || !strings.Contains(got, "cut a team release") {
		t.Errorf("skill.yaml:\n%s", got)
	}
	if routing := RoutingOfFiles(edited); len(routing.Triggers) != 1 || len(routing.Examples) != 1 {
		t.Errorf("the routing written reads back as %+v", routing)
	}
}

func TestEditFilesRefusesWhatOnlyTheHubChanges(t *testing.T) {
	t.Parallel()
	name, rationale := "other", "why"
	for label, input := range map[string]UpdateInput{
		"name":      {Name: &name},
		"content":   {SetContent: true, Content: []byte("x")},
		"runtime":   {Runtime: map[string]any{}},
		"rationale": {Rationale: &rationale},
		"approval":  {ContentReviewedDigest: &name},
	} {
		if _, err := EditFiles(editFixture(), "team-release", input); !errors.Is(err, ErrEditNeedsHub) {
			t.Errorf("%s: %v", label, err)
		}
	}
}

func TestEditFilesTreatsAnEmptyValueAsAnAbsentOne(t *testing.T) {
	t.Parallel()
	files := editFixture()
	files["skill.yaml"] = []byte("schema_version: 1\nrouting:\n  triggers:\n    - cut a team release\n  not_for:\n    - deploying\n")
	routing := RoutingOfFiles(files)
	edited, err := EditFiles(files, "team-release", UpdateInput{Routing: &routing})
	if err != nil {
		t.Fatal(err)
	}
	if string(edited["skill.yaml"]) != string(files["skill.yaml"]) {
		t.Errorf("writing the routing it already has rewrote skill.yaml:\n%s", edited["skill.yaml"])
	}
}
