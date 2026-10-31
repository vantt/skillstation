package distill

import (
	"testing"

	"github.com/vantt/skillstation/internal/source"
)

func TestCoverageSupportsCompleteConsultedAndRuledOutLedger(t *testing.T) {
	changed := []ChangedResource{{Path: "consulted.md", Status: "modified"}, {Path: "ruled.md", Status: "modified"}}
	coverage := []CoverageEntry{{Resource: "consulted.md", Status: "consulted", Reason: "Read for context; it made no claim."}, {Resource: "ruled.md", Status: "ruled_out_with_reason", Reason: "Generated content is outside source policy."}}
	if err := ValidateCoverage(changed, coverage); err != nil {
		t.Fatal(err)
	}
	coverage[1].Reason = ""
	if err := ValidateCoverage(changed, coverage); err == nil {
		t.Fatal("ruled-out coverage without a reason was accepted")
	}
}

func TestComparisonRequiresExactUniqueObservationBasis(t *testing.T) {
	revision := RevisionIdentity{Kind: "git-commit", Value: "abc", ContentDigest: source.Digest([]byte("abc"))}
	base := Comparison{SchemaVersion: 1, ID: "CMP-retry", RunID: "RUN-one", Subject: "retry", ObservationIDs: []string{"OBS-source--retry"}, Verdict: "convergent", BasedOn: map[string]RevisionIdentity{"OBS-source--retry": revision}}
	if err := ValidateComparison(base); err != nil {
		t.Fatal(err)
	}
	duplicate := base
	duplicate.ObservationIDs = []string{"OBS-source--retry", "OBS-source--retry"}
	if err := ValidateComparison(duplicate); err == nil {
		t.Fatal("duplicate observation identity was accepted")
	}
	extra := base
	extra.BasedOn = map[string]RevisionIdentity{"OBS-source--retry": revision, "OBS-source--other": revision}
	if err := ValidateComparison(extra); err == nil {
		t.Fatal("non-corresponding based_on entry was accepted")
	}
}
