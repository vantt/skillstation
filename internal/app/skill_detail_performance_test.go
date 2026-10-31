package app

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/vantt/skillstation/internal/catalog"
	"github.com/vantt/skillstation/internal/workspace"
)

// skillDetailP95Budget bounds one skill's detail on a 256-skill hub. The
// detail checks the skill against canonical validation of the whole hub, so
// its cost must not grow with every page view of an unchanged hub.
var skillDetailP95Budget = 150 * time.Millisecond

func init() {
	if os.Getenv("CI") != "" {
		skillDetailP95Budget = 500 * time.Millisecond
	}
}

func TestSkillDetailPerformanceBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("performance budgets are excluded from short tests")
	}
	if os.Getenv("SKILLSTATION_PERF") != "1" {
		t.Skip("wall-clock performance budgets run only with SKILLSTATION_PERF=1 on an otherwise idle machine")
	}
	if skillDetailRaceEnabled() {
		t.Skip("wall-clock performance budgets are not stable under race instrumentation")
	}
	const skills = 256
	root := filepath.Join(t.TempDir(), "workspace")
	if _, err := workspace.Apply(root); err != nil {
		t.Fatal(err)
	}
	// Each skill has an 8 KiB entrypoint and three references of the same
	// size, the shape of a curated skill rather than a stub.
	body := strings.Repeat("Check the change against the review checklist and cite the evidence.\n", 120)
	for index := 0; index < skills; index++ {
		id := fmt.Sprintf("detail-skill-%03d", index)
		meta := fmt.Sprintf("schema_version: 1\nid: %s\nname: Detail Skill %03d\nstatus: active\ndescription: Review detail target %03d.\nquality:\n  reviewed: true\nrouting:\n  triggers: [review detail target %03d]\n  operations: [review]\n  not_for: [write marketing prose]\n  min_scope: multi_step\n", id, index, index, index)
		writeSkillMetadata(t, root, "skills/benchmark/"+id, meta)
		writeResolveBenchmarkFile(t, root, "skills/benchmark/"+id+"/SKILL.md", fmt.Sprintf("---\nname: %s\ndescription: Review detail target %03d.\n---\n\n# Detail %03d\n\n%s", id, index, index, body))
		for reference := 0; reference < 3; reference++ {
			writeResolveBenchmarkFile(t, root, fmt.Sprintf("skills/benchmark/%s/references/guide-%d.md", id, reference), body)
		}
	}
	// A hub's files are normally older than the few seconds within which a
	// changed file is hashed again on every scan.
	settled := time.Now().Add(-time.Hour)
	if err := filepath.WalkDir(filepath.Join(root, "skills"), func(path string, _ os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return os.Chtimes(path, settled, settled)
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.BuildCatalogGeneration(t.Context(), root, catalog.BuildOptions{}); err != nil {
		t.Fatal(err)
	}

	service := SkillService{}
	detail := func(index int) {
		t.Helper()
		id := fmt.Sprintf("detail-skill-%03d", index%skills)
		got, err := service.GetSkillDetail(t.Context(), root, id)
		if err != nil || got.SkillID != id || got.Diverged {
			t.Fatalf("GetSkillDetail(%s) = %#v, %v", id, got.SkillID, err)
		}
	}
	for index := 0; index < 3; index++ {
		detail(index)
	}
	durations := make([]time.Duration, 30)
	for index := range durations {
		started := time.Now()
		detail(index * 7)
		durations[index] = time.Since(started)
	}
	slices.Sort(durations)
	p95 := durations[(95*len(durations)+99)/100-1]
	t.Logf("skill detail skills=%d n=%d p50=%s p95=%s", skills, len(durations), durations[len(durations)/2], p95)
	if p95 >= skillDetailP95Budget {
		t.Fatalf("skill detail p95 %s exceeds budget %s", p95, skillDetailP95Budget)
	}
}

func skillDetailRaceEnabled() bool {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return false
	}
	for _, setting := range info.Settings {
		if setting.Key == "-race" && setting.Value == "true" {
			return true
		}
	}
	return false
}
