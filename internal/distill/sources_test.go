package distill

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestWithSourcesSetsTheBlockAndKeepsEveryOtherLine(t *testing.T) {
	t.Parallel()
	fixture, err := os.ReadFile(filepath.Join("..", "migration", "testdata", "phase0-distill.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	sources := []Source{
		{ID: "openclaw", Repository: "https://github.com/openclaw/openclaw", Ref: "main", Path: ".agents/skills"},
		{ID: "superpowers", Repository: "https://github.com/obra/superpowers"},
	}

	added, err := WithSources(fixture, sources)
	if err != nil {
		t.Fatal(err)
	}
	cursors := strings.Index(string(added), "\ncursors:\n")
	if cursors < 0 || !strings.HasSuffix(string(added[:cursors+1]), "sources:\n  - id: openclaw\n    repository: https://github.com/openclaw/openclaw\n    ref: main\n    path: .agents/skills\n  - id: superpowers\n    repository: https://github.com/obra/superpowers\n") {
		t.Fatalf("the block is not written right before cursors:\n%s", added[:min(len(added), 2000)])
	}
	if removed := strings.Replace(string(added), string(added[strings.Index(string(added), "sources:\n"):cursors+1]), "", 1); removed != string(fixture) {
		t.Fatal("adding the block changed other lines of the file")
	}
	doc, err := UnmarshalDocument(added)
	if err != nil {
		t.Fatalf("the file with the block does not validate: %v", err)
	}
	if !reflect.DeepEqual(doc.Sources, sources) {
		t.Fatalf("sources read back = %#v", doc.Sources)
	}

	replaced, err := WithSources(added, sources[1:])
	if err != nil {
		t.Fatal(err)
	}
	if got, err := ReadSources(replaced); err != nil || !reflect.DeepEqual(got, sources[1:]) {
		t.Fatalf("replaced sources = %#v, %v", got, err)
	}
	cleared, err := WithSources(replaced, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(cleared) != string(fixture) {
		t.Fatal("removing the block did not give back the original file")
	}
}

func TestWithSourcesKeepsCommentsAroundTheBlock(t *testing.T) {
	t.Parallel()
	data := "goal: {}\nsources:\n- id: old\n  repository: r\n# about cursors\ncursors: {}\n"
	got, err := WithSources([]byte(data), []Source{{ID: "new", Repository: "r2"}})
	if err != nil {
		t.Fatal(err)
	}
	if want := "goal: {}\nsources:\n  - id: new\n    repository: r2\n# about cursors\ncursors: {}\n"; string(got) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	inserted, err := WithSources([]byte("goal: {}\n# about cursors\ncursors: {}\n"), []Source{{ID: "new", Repository: "r2"}})
	if err != nil {
		t.Fatal(err)
	}
	if want := "goal: {}\nsources:\n  - id: new\n    repository: r2\n# about cursors\ncursors: {}\n"; string(inserted) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", inserted, want)
	}
}

func TestAFileHoldingOnlySourcesIsAnUndistilledDocument(t *testing.T) {
	t.Parallel()
	data, err := WithSources(nil, []Source{{ID: "docs", Repository: "https://github.com/example/docs", Ref: "v1.0"}})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := UnmarshalDocument(data)
	if err != nil {
		t.Fatalf("a file holding only sources is refused: %v", err)
	}
	if !doc.Undistilled() || doc.Sources[0].Ref != "v1.0" {
		t.Fatalf("doc = %#v", doc)
	}
	if _, err := UnmarshalDocument([]byte("sources:\n  - id: docs\n    repository: r\nlessons:\n  - key: x\n")); err == nil {
		t.Fatal("a document with lessons but no goal was accepted")
	}
}

func TestReadSourcesRefusesAMalformedBlock(t *testing.T) {
	t.Parallel()
	for name, data := range map[string]string{
		"not a list":          "sources: docs\n",
		"bad id":              "sources:\n  - id: Docs\n    repository: r\n",
		"no repository":       "sources:\n  - id: docs\n",
		"duplicate id":        "sources:\n  - id: docs\n    repository: r\n  - id: docs\n    repository: s\n",
		"unknown field":       "sources:\n  - id: docs\n    repository: r\n    trust: high\n",
		"ref is not text":     "sources:\n  - id: docs\n    repository: r\n    ref: 1.0\n",
		"top level not a map": "- sources\n",
	} {
		if _, err := ReadSources([]byte(data)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if got, err := ReadSources([]byte("goal: {}\n")); err != nil || got != nil {
		t.Fatalf("a file without the block: %#v, %v", got, err)
	}
}
