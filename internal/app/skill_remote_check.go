package app

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/vantt/skillstation/internal/canonical"
	"github.com/vantt/skillstation/internal/skillruntime"
)

// checkSkillEntry reads SKILL.md the way the hub will when it queues the
// folder: frontmatter that does not parse is refused here rather than found out
// by the person who reviews it. A missing description is only a warning, since
// the hub still takes the skill. A folder without SKILL.md was already refused
// when it was read.
func checkSkillEntry(tree map[string]treeFile) (*Error, []Warning) {
	entry := tree["SKILL.md"]
	_, _, hasFrontmatter, err := parseAndValidateSkillFrontmatter(entry.Data)
	if err != nil {
		return NewInvalidRequestError(err.Error(), "Fix the frontmatter at the top of SKILL.md (between the two --- lines) and run commit again."), nil
	}
	if !hasFrontmatter {
		return nil, []Warning{{Code: "skill_md_no_frontmatter", Summary: "SKILL.md has no frontmatter, so the skill has no description for agents to match a request against."}}
	}
	if _, description, _ := parseSkillMDFrontmatter(entry.Data); description == "" {
		return nil, []Warning{{Code: "skill_md_no_description", Summary: "SKILL.md has no description in its frontmatter, so agents have nothing to match a request against."}}
	}
	return nil, nil
}

// checkSkillSettings applies the hub's routing rules to the folder's
// skill.yaml, so a routing block the hub would refuse on arrival is refused
// here rather than found out by the person who reviews it.
func checkSkillSettings(tree map[string]treeFile) *Error {
	entry, found := tree["skill.yaml"]
	if !found {
		return nil
	}
	issues := canonical.ValidateSkillSettings("skill.yaml", entry.Data)
	if len(issues) == 0 {
		return nil
	}
	issue := issues[0]
	fix := issue.Fix
	if fix == "" {
		fix = "Fix skill.yaml as reported and run commit again."
	}
	return NewInvalidRequestError(fmt.Sprintf("skill.yaml line %d: %s", issue.Line, issue.Message), fix)
}

// hubSkillAnswer is what the hub answers for a skill id.
type hubSkillAnswer int

const (
	// hubSkillUnknown: the hub did not answer in a way that says anything.
	hubSkillUnknown hubSkillAnswer = iota
	// hubSkillAbsent: the hub serves no skill with this id.
	hubSkillAbsent
	// hubSkillSame: the hub's copy is the one the folder was checked out from.
	hubSkillSame
	// hubSkillChanged: the hub has the skill, in a version other than the one
	// the folder was checked out from (or there is no checkout to compare).
	hubSkillChanged
)

// CheckHub compares the previewed folder with what the hub serves under the
// same id and adds what the person should know before sending: the id is
// already taken, or the hub moved on since the checkout. It asks only for the
// bundle's headers, and only when this machine's own settings name the hub: a
// hub named by a cloned project's files alone is never sent the token before
// the person answered. Whatever it cannot find out, it leaves out.
func (service RemoteCommitService) CheckHub(ctx context.Context, hub RemoteHub, preview RemoteCommitPreview) RemoteCommitPreview {
	if preview.Error != nil || preview.NothingToCommit || !hub.MaySendToken {
		return preview
	}
	hubURL, problem := NormalizeConnectHubURL(hub.URL)
	if problem != nil {
		return preview
	}
	token := ReadHubToken(hubURL, hub.TokenFile)
	if token.Problem != nil {
		return preview
	}
	checkedOutHere := preview.CheckedOut && !hasWarning(preview.Warnings, "other_hub")
	held := ""
	if checkedOutHere {
		held = preview.baseBundle
	}
	switch service.probeSkill(ctx, hubURL, token.Value, preview.SkillID, held) {
	case hubSkillAbsent:
		if checkedOutHere {
			preview.Warnings = append(preview.Warnings, Warning{Code: "hub_gone", Summary: fmt.Sprintf("The hub no longer serves %s; sending this proposes it again as a new skill.", preview.SkillID)})
		} else {
			preview.Summary = fmt.Sprintf("%s: no skill with this id is served by the hub at %s; this submits it for review as a new skill.", preview.SkillID, preview.Hub)
		}
	case hubSkillChanged:
		if checkedOutHere {
			preview.Warnings = append(preview.Warnings, Warning{Code: "hub_moved", Summary: fmt.Sprintf("The hub's copy of %s changed since you checked it out. Run `skillstation skill pull %s` first: it brings the hub's changes in and keeps yours.", preview.SkillID, preview.SkillID)})
		} else {
			preview.Warnings = append(preview.Warnings, Warning{Code: "hub_has_skill", Summary: fmt.Sprintf("The hub already has a skill with the id %s: sending this folder proposes a replacement for it. To add a new skill instead, rename the folder, because the folder name is the id.", preview.SkillID)})
		}
	}
	return preview
}

func hasWarning(warnings []Warning, code string) bool {
	for _, warning := range warnings {
		if warning.Code == code {
			return true
		}
	}
	return false
}

// probeSkill asks the bundle endpoint about one skill and reads only its
// status. A skill that waits for review answers 403 and still counts as there.
func (service RemoteCommitService) probeSkill(ctx context.Context, hubURL, token, id, held string) hubSkillAnswer {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, hubURL+remoteBundlePath+"?skill="+url.QueryEscape(id), nil)
	if err != nil {
		return hubSkillUnknown
	}
	request.Header.Set("Accept", skillruntime.BundleMediaType)
	request.Header.Set("Authorization", "Bearer "+token)
	if skillruntime.ValidBundleDigest(held) {
		request.Header.Set("If-None-Match", `"`+held+`"`)
	}
	response, err := service.client().Do(request)
	if err != nil {
		return hubSkillUnknown
	}
	// The archive is not read: the status is the answer.
	_ = response.Body.Close()
	switch response.StatusCode {
	case http.StatusNotFound:
		return hubSkillAbsent
	case http.StatusNotModified:
		return hubSkillSame
	case http.StatusOK, http.StatusForbidden:
		return hubSkillChanged
	}
	return hubSkillUnknown
}
