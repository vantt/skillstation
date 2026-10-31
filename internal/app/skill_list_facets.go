package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/vantt/skillstation/internal/skillruntime"
)

// Skill origins, derived from provenance, sources and upstream status.
const (
	OriginAuthored = "authored"
	OriginLearned  = "learned"
	OriginAdopted  = "adopted"
	OriginForked   = "forked"
)

// SkillOrigins are the values accepted by the origin filter.
var SkillOrigins = []string{OriginAuthored, OriginLearned, OriginAdopted, OriginForked}

// SkillSorts are the values accepted by the sort option.
var SkillSorts = []string{"name", "added", "changed", "uses", "last-used"}

const (
	usageWindowDays    = 30
	usageRetentionDays = 180
	loadEntrypoint     = "load:entrypoint"
	dayLayout          = "2006-01-02"
)

// skillFacetDocument is the part of a skill manifest the listing reads.
type skillFacetDocument struct {
	contentTrustDocument
	Description  string   `json:"description"`
	Topics       []string `json:"topics"`
	Technologies []string `json:"technologies"`
	Labels       []string `json:"labels"`
	Routing      struct {
		Topics       []string `json:"topics"`
		Technologies []string `json:"technologies"`
	} `json:"routing"`
}

type skillFacets struct {
	description  string
	topics       []string
	technologies []string
	labels       []string
	hasUpstream  bool
	hasLearning  bool
	dir          string
}

// skillFacetRows reads manifest-derived facets of every skill, keyed by id.
func skillFacetRows(ctx context.Context, database *sql.DB) (map[string]*skillFacets, error) {
	rows, err := database.QueryContext(ctx, `SELECT skills.id, skills.path, canonical_entities.content_json FROM skills JOIN canonical_entities ON canonical_entities.id = skills.id`)
	if err != nil {
		return nil, fmt.Errorf("read skill metadata: %w", err)
	}
	defer rows.Close()
	facets := map[string]*skillFacets{}
	for rows.Next() {
		var id, metaPath, content string
		if err := rows.Scan(&id, &metaPath, &content); err != nil {
			return nil, fmt.Errorf("read skill metadata row: %w", err)
		}
		var document skillFacetDocument
		if err := json.Unmarshal([]byte(content), &document); err != nil {
			return nil, fmt.Errorf("decode skill %s metadata: %w", id, err)
		}
		facet := &skillFacets{description: document.Description, labels: document.Labels, dir: skillFolder(metaPath)}
		facet.topics = firstList(document.Topics, document.Routing.Topics)
		facet.technologies = firstList(document.Technologies, document.Routing.Technologies)
		facet.hasUpstream = skillruntime.IsThirdParty(document.provenance())
		for _, source := range document.Sources {
			for _, role := range source.Roles {
				facet.hasLearning = facet.hasLearning || role == "learning"
			}
		}
		facets[id] = facet
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read skill metadata: %w", err)
	}
	return facets, nil
}

func firstList(primary, fallback []string) []string {
	if len(primary) > 0 {
		return primary
	}
	return fallback
}

// skillFolder returns the skill directory for a metadata path, which is either
// <dir>/skill.meta.yaml or <dir>/.meta/skill.yaml.
func skillFolder(metaPath string) string {
	dir := path.Dir(metaPath)
	if path.Base(dir) == ".meta" {
		dir = path.Dir(dir)
	}
	return dir
}

// markSourceLinks records, per skill, the source links that say where it came
// from: an "origin" link is an upstream, a learning or inspiration link a
// learning source.
func markSourceLinks(ctx context.Context, database *sql.DB, facets map[string]*skillFacets) error {
	rows, err := database.QueryContext(ctx, `SELECT json_extract(content_json, '$.skill_id'), COALESCE(json_extract(content_json, '$.role'), '') FROM canonical_entities WHERE kind = 'skill_source_link'`)
	if err != nil {
		return fmt.Errorf("read skill source links: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var skillID, role string
		if err := rows.Scan(&skillID, &role); err != nil {
			return fmt.Errorf("read skill source link: %w", err)
		}
		facet := facets[skillID]
		if facet == nil {
			continue
		}
		switch role {
		case "origin":
			facet.hasUpstream = true
		case "learning-source", "inspiration":
			facet.hasLearning = true
		}
	}
	return rows.Err()
}

// deriveOrigin applies the precedence: an upstream wins, then a learning
// source, otherwise the skill is authored.
func deriveOrigin(hasUpstream bool, upstreamStatus string, hasLearning bool) string {
	switch {
	case hasUpstream && (upstreamStatus == "modified" || upstreamStatus == "diverged"):
		return OriginForked
	case hasUpstream:
		return OriginAdopted
	case hasLearning:
		return OriginLearned
	}
	return OriginAuthored
}

// skillGitDates returns the first and last commit date of every skill folder
// in one pass over the history. Missing Git or uncommitted skills yield none.
func skillGitDates(ctx context.Context, root string, facets map[string]*skillFacets) (added, changed map[string]string) {
	added, changed = map[string]string{}, map[string]string{}
	output, err := gitOutput(ctx, root, "log", "--format=%x00%cI", "--name-only", "--", "skills/")
	if err != nil {
		return added, changed
	}
	byDir := make(map[string]string, len(facets))
	for id, facet := range facets {
		byDir[facet.dir] = id
	}
	date := ""
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "\x00") {
			date = strings.TrimPrefix(line, "\x00")
			continue
		}
		if line == "" || date == "" {
			continue
		}
		id := skillOfPath(byDir, line)
		if id == "" {
			continue
		}
		// Commits arrive newest first: the first seen is the last change,
		// the last seen is the first commit.
		if _, seen := changed[id]; !seen {
			changed[id] = date
		}
		added[id] = date
	}
	return added, changed
}

// skillOfPath maps a repository file path to the skill folder that holds it.
func skillOfPath(byDir map[string]string, file string) string {
	for dir := path.Dir(file); dir != "." && dir != "/"; dir = path.Dir(dir) {
		if id, ok := byDir[dir]; ok {
			return id
		}
	}
	return ""
}

type skillUsageFacts struct {
	uses     map[string]int64
	lastUsed map[string]string
	// available is true when any load:entrypoint rollup exists in the retention window.
	available bool
}

// skillUsage sums entrypoint loads over the last 30 days. A telemetry store
// error leaves usage unavailable; it never fails the listing.
func skillUsage(ctx context.Context, root string, now time.Time) skillUsageFacts {
	facts := skillUsageFacts{uses: map[string]int64{}, lastUsed: map[string]string{}}
	recorder, err := (TelemetryService{}).Open(root)
	if err != nil {
		return facts
	}
	var closeErr error
	defer closeTelemetryRecorder(recorder, &closeErr)
	today := now.UTC()
	rows, err := recorder.Rollups(ctx, today.AddDate(0, 0, -usageRetentionDays).Format(dayLayout), today.Format(dayLayout))
	if err != nil {
		return facts
	}
	windowStart := today.AddDate(0, 0, -(usageWindowDays - 1)).Format(dayLayout)
	for _, row := range rows {
		if row.Metric != loadEntrypoint || row.SkillID == "" {
			continue
		}
		facts.available = true
		if row.Day < windowStart || row.Count <= 0 {
			continue
		}
		facts.uses[row.SkillID] += row.Count
		if row.Day > facts.lastUsed[row.SkillID] {
			facts.lastUsed[row.SkillID] = row.Day
		}
	}
	return facts
}

func containsFold(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(value, want) {
			return true
		}
	}
	return false
}

func matchesSearch(entry SkillListEntry, needle string) bool {
	needle = strings.ToLower(needle)
	haystack := []string{entry.ID, entry.Name, entry.Description}
	haystack = append(append(append(haystack, entry.Topics...), entry.Technologies...), entry.Labels...)
	for _, text := range haystack {
		if strings.Contains(strings.ToLower(text), needle) {
			return true
		}
	}
	return false
}

func filterSkills(skills []SkillListEntry, query SkillListQuery) []SkillListEntry {
	kept := skills[:0:0]
	for _, entry := range skills {
		switch {
		case query.Origin != "" && entry.Origin != query.Origin:
		case query.Topic != "" && !containsFold(entry.Topics, query.Topic) && !containsFold(entry.Technologies, query.Topic):
		case query.Label != "" && !containsFold(entry.Labels, query.Label):
		case query.Search != "" && !matchesSearch(entry, query.Search):
		default:
			kept = append(kept, entry)
		}
	}
	return kept
}

// sortSkills orders skills in place. Empty dates sort last.
func sortSkills(skills []SkillListEntry, order string) {
	newestFirst := func(a, b string) (decided, less bool) {
		if a == b {
			return false, false
		}
		if a == "" || b == "" {
			return true, b == ""
		}
		return true, a > b
	}
	sort.SliceStable(skills, func(i, j int) bool {
		a, b := skills[i], skills[j]
		switch order {
		case "name", "":
			if an, bn := strings.ToLower(a.Name), strings.ToLower(b.Name); an != bn {
				return an < bn
			}
		case "added":
			if decided, less := newestFirst(a.AddedAt, b.AddedAt); decided {
				return less
			}
		case "changed":
			if decided, less := newestFirst(a.ChangedAt, b.ChangedAt); decided {
				return less
			}
		case "uses":
			if a.Uses30d != b.Uses30d {
				return a.Uses30d > b.Uses30d
			}
			if decided, less := newestFirst(a.LastUsed, b.LastUsed); decided {
				return less
			}
		case "last-used":
			if decided, less := newestFirst(a.LastUsed, b.LastUsed); decided {
				return less
			}
		}
		return a.ID < b.ID
	})
}
