package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"

	catalogpkg "github.com/vantt/skillstation/internal/catalog"
	"github.com/vantt/skillstation/internal/mutation"
	"github.com/vantt/skillstation/internal/skillmeta"
	"github.com/vantt/skillstation/internal/workspace"
)

// AuditRecord is the outcome of a finished security audit of one skill, as the
// skill-audit system skill reports it: the verdict and the digest of the rules
// the audit applied. The hub cannot know the auditor's rules, so their digest
// comes from the caller; the digest of the skill's content is computed by the
// hub when it records the audit.
type AuditRecord struct {
	Verdict     string `json:"verdict"`
	RulesDigest string `json:"rules_digest"`
}

// AuditRecordResult is the result of recording an audit in a skill's
// .meta/hub.yaml.
type AuditRecordResult struct {
	Result
	SkillID       string                   `json:"skill_id"`
	Path          string                   `json:"path"`
	Verdict       string                   `json:"verdict"`
	AuditedAt     string                   `json:"audited_at"`
	ContentDigest string                   `json:"content_digest"`
	Commit        *workspace.CommitOutcome `json:"commit,omitempty"`
}

// SkillAuditService records audits through the hub's mutation path, so a
// record is locked, atomic, validated and committed like any other change.
type SkillAuditService struct {
	// Now is the clock of audited_at; nil uses time.Now.
	Now func() time.Time
}

var auditDigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// Record writes record as the audit block of skill id, with the digest of the
// skill's content as it is now. The block replaces an earlier one; every other
// key of hub.yaml keeps its value, and the skill's dates do not move, since an
// audit does not change the skill.
func (service SkillAuditService) Record(ctx context.Context, path, id string, record AuditRecord) (AuditRecordResult, error) {
	if !skillmeta.ValidSkillID(id) {
		return AuditRecordResult{}, NewInvalidRequestError(fmt.Sprintf("%q is not a skill id.", id), "Name the skill by its id, for example `skillstation skill audit-record pdf --verdict ok ...`.")
	}
	switch record.Verdict {
	case "ok", "review", "block":
	default:
		return AuditRecordResult{}, NewInvalidRequestError("The verdict must be ok, review or block.", "Pass --verdict ok, review or block.")
	}
	if err := checkRulesDigest(record.RulesDigest); err != nil {
		return AuditRecordResult{}, err
	}
	root, err := workspace.Discover(path)
	if err != nil {
		return AuditRecordResult{}, err
	}
	if _, statErr := os.Lstat(root); statErr != nil {
		return AuditRecordResult{}, fmt.Errorf("no Skillstation workspace at %s: %w", root, statErr)
	}
	read := skillmeta.DirReader(root)
	if skillmeta.HubLayout(read) < skillmeta.SplitLayout {
		return AuditRecordResult{}, NewInvalidRequestError("This hub keeps skill metadata in an earlier layout, which has no audit record.", "Run `skillstation maintain migrate` first.")
	}
	_, skillDir, _, err := locateSkillDir(root, id)
	if err != nil {
		return AuditRecordResult{}, err
	}
	hubPath := skillDir + "/" + skillmeta.HubFile
	current, err := read(hubPath)
	if err != nil {
		return AuditRecordResult{}, fmt.Errorf("read %s: %w", hubPath, err)
	}
	contentDigest, err := auditContentDigest(ctx, root, id)
	if err != nil {
		return AuditRecordResult{}, err
	}
	now := time.Now
	if service.Now != nil {
		now = service.Now
	}
	auditedAt := now().UTC().Format(time.RFC3339)
	next, err := withAuditRecord(current, RecordedAudit{Verdict: record.Verdict, AuditedAt: auditedAt, ContentDigest: contentDigest, RulesDigest: record.RulesDigest})
	if err != nil {
		return AuditRecordResult{}, fmt.Errorf("%s: %w", hubPath, err)
	}
	sum := sha256.Sum256(current)
	proposal, err := mutation.PlanMutation(root, mutation.WriteSet{Command: "skill_audit_record", Changes: []mutation.Change{
		{Path: hubPath, BeforeDigest: "sha256:" + hex.EncodeToString(sum[:]), Contents: next},
	}})
	if err != nil {
		return AuditRecordResult{}, err
	}
	receipt, err := mutation.ConfirmMutationWithOptions(root, proposal, mutation.Confirmation{ProposalID: proposal.ID, ProposalDigest: proposal.Digest, BaseCatalogSnapshot: proposal.BaseCatalogSnapshot}, mutation.Options{
		Context:       ctx,
		CommitMessage: "Record the " + record.Verdict + " audit of " + id,
		PostCanonical: func(expected string) (mutation.Publication, error) {
			built, err := catalogpkg.BuildCatalogGenerationWhileLocked(ctx, root, expected, catalogpkg.BuildOptions{})
			if err != nil {
				return mutation.Publication{}, err
			}
			return mutation.Publication{CatalogSnapshot: built.Pointer.CatalogSnapshot, Generation: built.Pointer.Generation}, nil
		},
	})
	if err != nil {
		return AuditRecordResult{}, err
	}
	result := AuditRecordResult{
		Result:  NewResult(StatusApplied, "Recorded the "+record.Verdict+" audit of "+id+"."),
		SkillID: id, Path: hubPath, Verdict: record.Verdict, AuditedAt: auditedAt, ContentDigest: contentDigest,
	}
	if receipt.Commit.Status != "" {
		commit := receipt.Commit
		result.Commit = &commit
	}
	return result, nil
}

// checkRulesDigest refuses a rules digest that is not sha256:<64 lowercase hex>.
func checkRulesDigest(digest string) error {
	if auditDigestPattern.MatchString(digest) {
		return nil
	}
	return NewInvalidRequestError("--rules-digest must be sha256:<64 lowercase hex characters>.", "Record through the skill-audit script, which computes the digest of its rules: `node .agents/skills/skill-audit/scripts/audit.mjs --record <id> --verdict <v>`.")
}

// auditContentDigest is the digest an audit record binds to: the content
// digest a person approves with `skill edit --approve-content`, so an audited
// and an approved version of a skill can be compared.
func auditContentDigest(ctx context.Context, root, id string) (string, error) {
	trust, err := (SkillService{}).ContentTrustFor(ctx, root, id)
	if err != nil {
		return "", err
	}
	return trust.ContentDigest, nil
}

// withAuditRecord returns hub.yaml contents with its audit block set to
// record, keeping every other key, its order and its comments.
func withAuditRecord(contents []byte, record RecordedAudit) ([]byte, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(contents, &document); err != nil {
		return nil, err
	}
	if document.Kind != yaml.DocumentNode || len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("hub metadata must be one YAML mapping")
	}
	mapping := document.Content[0]
	audit := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, field := range [][2]string{{"verdict", record.Verdict}, {"audited_at", record.AuditedAt}, {"content_digest", record.ContentDigest}, {"rules_digest", record.RulesDigest}} {
		value := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: field[1]}
		if field[0] == "audited_at" {
			value.Style = yaml.DoubleQuotedStyle
		}
		audit.Content = append(audit.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: field[0]}, value)
	}
	key := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "audit"}
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == "audit" {
			mapping.Content[index+1] = audit
			return skillmeta.Encode(mapping)
		}
	}
	mapping.Content = append(mapping.Content, key, audit)
	return skillmeta.Encode(mapping)
}
