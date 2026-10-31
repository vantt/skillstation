package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/vantt/skillstation/internal/skill"
	"github.com/vantt/skillstation/internal/skillmeta"
	"github.com/vantt/skillstation/internal/workspace"
)

// Values of SkillAuditStatus.SinceLastAudit.
const (
	AuditNeverAudited   = "never-audited"
	AuditContentChanged = "content-changed"
	AuditRulesChanged   = "rules-changed"
	AuditUnchanged      = "unchanged"
)

// RecordedAudit is the audit block of a skill's .meta/hub.yaml.
type RecordedAudit struct {
	Verdict       string `json:"verdict" yaml:"verdict"`
	AuditedAt     string `json:"audited_at" yaml:"audited_at"`
	ContentDigest string `json:"content_digest" yaml:"content_digest"`
	RulesDigest   string `json:"rules_digest" yaml:"rules_digest"`
}

// complete reports whether every field of the block is set; a partial block
// does not say what was audited, so it counts as no record.
func (record RecordedAudit) complete() bool {
	return record.Verdict != "" && record.AuditedAt != "" && record.ContentDigest != "" && record.RulesDigest != ""
}

// SkillAuditStatus says whether the recorded audit of one skill still covers
// its content and, when the caller names its rules, those rules.
type SkillAuditStatus struct {
	SkillID string `json:"skill_id"`
	// Dir is the skill folder, an absolute path on this machine.
	Dir string `json:"dir"`
	// ContentDigest is the digest of the content as it is now, computed the
	// way an audit record and a content approval compute it.
	ContentDigest  string         `json:"content_digest"`
	Recorded       *RecordedAudit `json:"recorded"`
	SinceLastAudit string         `json:"since_last_audit"`
}

// AuditStatusResult lists the audit status of the requested skills.
type AuditStatusResult struct {
	Result
	Workspace string             `json:"workspace"`
	Skills    []SkillAuditStatus `json:"skills"`
}

// Status reports, for each skill in ids (every skill of the hub when ids is
// empty), the folder, the current content digest, the recorded audit and
// whether that audit still covers the content. When rulesDigest is set, a
// record of other rules is reported as rules-changed; without it the rules are
// not compared.
func (service SkillAuditService) Status(ctx context.Context, path string, ids []string, rulesDigest string) (AuditStatusResult, error) {
	if rulesDigest != "" {
		if err := checkRulesDigest(rulesDigest); err != nil {
			return AuditStatusResult{}, err
		}
	}
	for _, id := range ids {
		if !skillmeta.ValidSkillID(id) {
			return AuditStatusResult{}, NewInvalidRequestError(fmt.Sprintf("%q is not a skill id.", id), "Name each skill by its id, or pass --all.")
		}
	}
	root, err := workspace.Discover(path)
	if err != nil {
		return AuditStatusResult{}, err
	}
	if len(ids) == 0 {
		if ids, err = hubSkillIDs(root); err != nil {
			return AuditStatusResult{}, err
		}
	}
	result := AuditStatusResult{Workspace: root, Skills: make([]SkillAuditStatus, 0, len(ids))}
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return AuditStatusResult{}, err
		}
		status, err := skillAuditStatus(ctx, root, id, rulesDigest)
		if err != nil {
			return AuditStatusResult{}, err
		}
		result.Skills = append(result.Skills, status)
	}
	result.Result = NewResult(StatusOK, fmt.Sprintf("Audit status of %s.", termCount(len(result.Skills), "skill", "skills")))
	return result, nil
}

func skillAuditStatus(ctx context.Context, root, id, rulesDigest string) (SkillAuditStatus, error) {
	_, skillRelDir, metadata, err := locateSkillDir(root, id)
	if err != nil {
		if errors.Is(err, skill.ErrNotFound) {
			return SkillAuditStatus{}, NewNotFoundError(fmt.Sprintf("No skill %q in this hub.", id), "Run `skillstation skill list` to see the skill ids.")
		}
		return SkillAuditStatus{}, err
	}
	digest, err := auditContentDigest(ctx, root, id)
	if err != nil {
		return SkillAuditStatus{}, err
	}
	var document struct {
		Audit *RecordedAudit `yaml:"audit"`
	}
	if err := yaml.Unmarshal(metadata, &document); err != nil {
		return SkillAuditStatus{}, fmt.Errorf("read the metadata of %s: %w", id, err)
	}
	status := SkillAuditStatus{SkillID: id, Dir: filepath.Join(root, filepath.FromSlash(skillRelDir)), ContentDigest: digest}
	switch record := document.Audit; {
	case record == nil || !record.complete():
		status.SinceLastAudit = AuditNeverAudited
	case record.ContentDigest != digest:
		status.Recorded, status.SinceLastAudit = record, AuditContentChanged
	case rulesDigest != "" && record.RulesDigest != rulesDigest:
		status.Recorded, status.SinceLastAudit = record, AuditRulesChanged
	default:
		status.Recorded, status.SinceLastAudit = record, AuditUnchanged
	}
	return status, nil
}

// hubSkillIDs lists the ids of the skill folders under skills/ that have
// metadata, sorted.
func hubSkillIDs(root string) ([]string, error) {
	found, err := listWorkspaceSkillIDs(root)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(found))
	for id := range found {
		if _, _, _, locateErr := locateSkillDir(root, id); locateErr == nil {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids, nil
}
