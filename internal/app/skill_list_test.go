package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/vantt/skillhub-mcp/internal/telemetry"
)

func TestDeriveOrigin(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name     string
		upstream bool
		status   string
		learning bool
		want     string
	}{
		{"nothing", false, "", false, OriginAuthored},
		{"learning source", false, "", true, OriginLearned},
		{"upstream in sync", true, "in_sync", false, OriginAdopted},
		{"upstream wins over learning", true, "", true, OriginAdopted},
		{"modified upstream", true, "modified", true, OriginForked},
		{"diverged upstream", true, "diverged", false, OriginForked},
	} {
		if got := deriveOrigin(test.upstream, test.status, test.learning); got != test.want {
			t.Errorf("%s: origin = %s, want %s", test.name, got, test.want)
		}
	}
}

func writeFacetSkill(t *testing.T, root, id, extra string) {
	t.Helper()
	dir := filepath.Join(root, "skills", "software", id)
	if err := os.MkdirAll(filepath.Join(dir, ".meta"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+id+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	meta := "schema_version: 1\nid: " + id + "\nname: " + strings.ToUpper(id[:1]) + id[1:] + "\nstatus: active\ndescription: Does " + id + " work.\n" + extra +
		"routing:\n  triggers: [" + id + "]\n  not_for: [none]\n  min_scope: single_step\n"
	if err := os.WriteFile(filepath.Join(dir, ".meta", "skill.yaml"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}
}

func listedSkillIDs(skills []SkillListEntry) []string {
	ids := make([]string, len(skills))
	for index, entry := range skills {
		ids[index] = entry.ID
	}
	return ids
}

func TestListSkillCatalogFacetsFilterSearchAndSort(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root := newSkillWorkspace(t)
	writeFacetSkill(t, root, "mine", "labels: [work, favorite]\ntopics: [Git]\ntechnologies: [go]\n")
	writeFacetSkill(t, root, "studied", "sources:\n  - id: src-docs\n    kind: github\n    repo: https://github.com/example/docs\n    roles: [learning]\n")
	writeFacetSkill(t, root, "borrowed", "sources:\n  - id: src-up\n    kind: github\n    repo: https://github.com/example/up\n    ref: main\n    path: x\n    roles: [upstream]\n")
	if _, err := (CatalogService{}).BuildCatalogGeneration(ctx, root); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "skills", "--no-gpg-sign"}} {
		command := exec.Command("git", args...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	recorder, err := (TelemetryService{}).Open(root)
	if err != nil {
		t.Fatal(err)
	}
	load := func(id, skillID string, occurred time.Time) {
		recorder.Record(telemetry.Event{
			Version: telemetry.EventVersion, ID: id, Type: telemetry.EventSkillLoaded, OccurredAt: occurred,
			CatalogSnapshot: "s", PolicyRevision: "p", Client: telemetry.Client{Name: "test", Version: "1"},
			Payload: map[string]any{"skill_id": skillID, "basis": telemetry.LoadBasisServerObserved, "resource_kind": "entrypoint", "surface": "skill_get"},
		})
	}
	now := time.Now().UTC()
	load("l1", "borrowed", now)
	load("l2", "borrowed", now)
	load("l3", "studied", now.AddDate(0, 0, -3))
	load("l4", "mine", now.AddDate(0, 0, -45))
	if err := recorder.Close(ctx); err != nil {
		t.Fatal(err)
	}

	service := SkillService{}
	list := func(query SkillListQuery) SkillListResult {
		t.Helper()
		result, err := service.ListSkillCatalog(ctx, root, query)
		if err != nil {
			t.Fatalf("ListSkillCatalog(%+v): %v", query, err)
		}
		return result
	}
	all := list(SkillListQuery{})
	byID := map[string]SkillListEntry{}
	for _, entry := range all.Skills {
		byID[entry.ID] = entry
	}
	mine := byID["mine"]
	if mine.Origin != OriginAuthored || byID["studied"].Origin != OriginLearned || byID["borrowed"].Origin != OriginAdopted {
		t.Fatalf("origins = %s/%s/%s", mine.Origin, byID["studied"].Origin, byID["borrowed"].Origin)
	}
	if !reflect.DeepEqual(mine.Labels, []string{"work", "favorite"}) || !reflect.DeepEqual(mine.Topics, []string{"Git"}) || mine.Description == "" || mine.AddedAt == "" || mine.ChangedAt == "" {
		t.Fatalf("mine facets = %#v", mine)
	}
	if !all.UsageAvailable || byID["borrowed"].Uses30d != 2 || byID["studied"].Uses30d != 1 || mine.Uses30d != 0 || byID["borrowed"].LastUsed != now.Format("2006-01-02") {
		t.Fatalf("usage = %#v / %#v / %#v available=%v", byID["borrowed"], byID["studied"], mine, all.UsageAvailable)
	}

	for _, test := range []struct {
		name  string
		query SkillListQuery
		want  []string
	}{
		{"default sort is by name", SkillListQuery{}, []string{"borrowed", "mine", "studied"}},
		{"origin", SkillListQuery{Origin: OriginLearned}, []string{"studied"}},
		{"topic matches technologies too", SkillListQuery{Topic: "GO"}, []string{"mine"}},
		{"label", SkillListQuery{Label: "Favorite"}, []string{"mine"}},
		{"state with no match", SkillListQuery{State: "draft"}, []string{}},
		{"search over labels", SkillListQuery{Search: "favor"}, []string{"mine"}},
		{"search over description", SkillListQuery{Search: "does studied"}, []string{"studied"}},
		{"sort by uses", SkillListQuery{Sort: "uses"}, []string{"borrowed", "studied", "mine"}},
		{"sort by last used", SkillListQuery{Sort: "last-used"}, []string{"borrowed", "studied", "mine"}},
		{"sort by added falls back to id", SkillListQuery{Sort: "added"}, []string{"borrowed", "mine", "studied"}},
	} {
		if got := listedSkillIDs(list(test.query).Skills); !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s: %v, want %v", test.name, got, test.want)
		}
	}
	for _, query := range []SkillListQuery{{Origin: "stolen"}, {Sort: "random"}, {State: "bogus"}} {
		if _, err := service.ListSkillCatalog(ctx, root, query); err == nil {
			t.Errorf("ListSkillCatalog(%+v) accepted an invalid value", query)
		}
	}
}

func TestListSkillCatalogReportsUsageUnavailableWithoutLoads(t *testing.T) {
	t.Parallel()
	root := newSkillWorkspace(t)
	createTestSkill(t, root, "quiet", "Quiet", true)
	result, err := (SkillService{}).ListSkillCatalog(context.Background(), root, SkillListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if result.UsageAvailable || len(result.Skills) != 1 || result.Skills[0].Uses30d != 0 {
		t.Fatalf("result = %#v", result)
	}
}
