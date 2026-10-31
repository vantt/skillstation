package app

import (
	"errors"

	"github.com/vantt/skillstation/internal/canonical"
	"github.com/vantt/skillstation/internal/mutation"
)

// SkillCommandError renders a skill change the hub refused for a person at a
// command line, with a FIX that names the command for the skill id. A missing
// skill is not_found and points to the skill list. Any other refusal keeps its
// own message as the WHY; a change refused by canonical validation names the
// edit of the routing field the validation reported.
//
// The field comes from the code of the validation issue, in one place;
// delivery adapters render the result and keep no matching rules of their own.
func SkillCommandError(err error, id string) *Error {
	if ClassifyError(err).Code == ErrorNotFound {
		why := "skill not found"
		if id != "" {
			why += ": " + id
		}
		return NewNotFoundError(why, "Run `skillstation skill list` to inspect available skills.")
	}
	return NewInvalidRequestError(err.Error(), skillValidationFix(err, id))
}

func skillValidationFix(err error, id string) string {
	if id == "" {
		id = "<id>"
	}
	var invalid *mutation.ValidationError
	if !errors.As(err, &invalid) {
		return "Correct the skill fields or workspace state and retry."
	}
	switch invalid.Issue.Code {
	case canonical.IssueRoutingNotFor, canonical.IssueRoutingReviewRationale:
		return "Preview `skillstation skill edit " + id + " --not-for \"<when not to use>\"`, then review and confirm before retrying. If nothing applies, preview `skillstation skill edit " + id + " --rationale \"<reason>\"`."
	case canonical.IssueRoutingMinScope:
		return "Preview `skillstation skill edit " + id + " --min-scope single_step`, then review and confirm before retrying."
	case canonical.IssueRoutingTriggerMissing:
		return "Preview `skillstation skill edit " + id + " --trigger \"<when to use>\"`, then review and confirm before retrying."
	}
	return "Correct the skill fields or workspace state and retry."
}
