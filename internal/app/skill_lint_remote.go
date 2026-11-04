package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"

	"github.com/vantt/skillstation/internal/skillmeta"
	"github.com/vantt/skillstation/internal/workspace"
)

const remoteLintPath = "/mcp/lint"

// MaxRemoteLintBytes bounds the body of a routing lint request: SKILL.md and
// skill.yaml of one skill.
const MaxRemoteLintBytes = 1 << 20

// ErrRemoteLintInvalid marks a lint request the hub cannot read as a skill.
var ErrRemoteLintInvalid = errors.New("the skill cannot be read")

// RemoteLintRequest is what a project sends a hub to check the routing of a
// skill it has not sent: the two files routing is read from.
type RemoteLintRequest struct {
	SkillID   string `json:"skill_id"`
	SkillMD   string `json:"skill_md"`
	SkillYAML string `json:"skill_yaml,omitempty"`
}

// RemoteLintReply is the hub's routing lint of the skill it was given: the
// routing it read, and what the hub's other active skills and its examples say
// about it.
type RemoteLintReply struct {
	SkillID     string             `json:"skill_id"`
	Description string             `json:"description"`
	Routing     SkillLintRouting   `json:"routing"`
	Findings    []SkillLintFinding `json:"findings"`
}

// LintProposedSkill lints a skill that is not in the hub yet against the hub's
// active skills, the same lint a commit preview runs on the hub machine. It
// only reads: nothing is queued, written or rebuilt.
func LintProposedSkill(ctx context.Context, workspacePath string, request RemoteLintRequest) (RemoteLintReply, error) {
	if !skillmeta.ValidSkillID(request.SkillID) {
		return RemoteLintReply{}, fmt.Errorf("%w: %q is not a skill id", ErrRemoteLintInvalid, request.SkillID)
	}
	if request.SkillMD == "" {
		return RemoteLintReply{}, fmt.Errorf("%w: SKILL.md is empty", ErrRemoteLintInvalid)
	}
	if _, _, _, err := parseAndValidateSkillFrontmatter([]byte(request.SkillMD)); err != nil {
		return RemoteLintReply{}, fmt.Errorf("%w: %v", ErrRemoteLintInvalid, err)
	}
	root, err := workspace.Discover(workspacePath)
	if err != nil {
		return RemoteLintReply{}, err
	}
	// A folder that was packaged by hand may have no skill.yaml: its routing is
	// then empty, and the lint says what is missing.
	meta, skillMD := []byte(request.SkillYAML), []byte(request.SkillMD)
	if len(bytes.TrimSpace(meta)) == 0 {
		meta = []byte("{}\n")
	}
	target, err := decodeProposedSkill(request.SkillID, meta, skillMD)
	if err != nil {
		return RemoteLintReply{}, fmt.Errorf("%w: %v", ErrRemoteLintInvalid, err)
	}
	findings, _, err := lintProposedSkill(ctx, root, request.SkillID, meta, skillMD)
	if err != nil {
		return RemoteLintReply{}, fmt.Errorf("%w: %v", ErrRemoteLintInvalid, err)
	}
	return RemoteLintReply{
		SkillID: request.SkillID, Description: target.Description,
		Routing: SkillLintRouting{
			Triggers: nonNil(target.Triggers), Examples: nonNil(target.Examples), CounterExamples: nonNil(target.CounterExamples),
			NotFor: nonNil(target.NotFor), MinScope: target.MinScope,
		},
		Findings: lintFindingsOf(request.SkillID, findings),
	}, nil
}

// RemoteLintService asks the hub on another machine to lint the routing of a
// folder in the project, so the author sees trigger collisions and examples
// that resolve to another skill before sending.
type RemoteLintService struct {
	Client *http.Client
}

// RemoteLintResult is the answer for one folder.
type RemoteLintResult struct {
	Result
	SkillID     string             `json:"skill_id"`
	Hub         string             `json:"hub"`
	Folder      string             `json:"folder"`
	Description string             `json:"description"`
	Routing     SkillLintRouting   `json:"routing"`
	Findings    []SkillLintFinding `json:"findings"`
}

// Lint reads the folder named by target (an id checked out in the project, or
// a path) and sends its SKILL.md and skill.yaml to the hub. Nothing else
// leaves the machine, and the hub keeps nothing. The token goes only to a hub
// this machine's own settings name; for any other the result says how to trust
// it, and no request is made.
func (service RemoteLintService) Lint(ctx context.Context, hub RemoteHub, target, projectDir string) (RemoteLintResult, error) {
	failed := func(err *Error) (RemoteLintResult, error) {
		return RemoteLintResult{Result: ErrorResult(err), Hub: hub.URL, Findings: []SkillLintFinding{}}, nil
	}
	if target == "" {
		return failed(NewInvalidRequestError("lint needs a skill folder or the id of a skill checked out in this project", "Run `skillstation skill lint <folder>`."))
	}
	resolved, refusal := resolveCommitTarget(ctx, target, projectDir)
	if refusal != nil {
		return failed(refusal)
	}
	tree, _, refusal, err := readProjectFolder(resolved.dir)
	if err != nil {
		return RemoteLintResult{}, err
	}
	if refusal != nil {
		return failed(refusal)
	}
	if refusal, _ := checkSkillEntry(tree); refusal != nil {
		return failed(refusal)
	}
	if !hub.MaySendToken {
		return failed(NewInvalidRequestError(
			fmt.Sprintf("only this project's files name the hub at %s, so its routing check does not get your hub token", hub.URL),
			"Trust that hub with `skillstation connect --hub "+hub.URL+" --user`, then lint again."))
	}
	hubURL, problem := NormalizeConnectHubURL(hub.URL)
	if problem != nil {
		return failed(problem)
	}
	token := ReadHubToken(hubURL, hub.TokenFile)
	if token.Problem != nil {
		return failed(NewInvalidRequestError(token.Problem.Detail, token.Problem.Fix))
	}
	body, err := json.Marshal(RemoteLintRequest{SkillID: resolved.id, SkillMD: string(tree["SKILL.md"].Data), SkillYAML: string(tree["skill.yaml"].Data)})
	if err != nil {
		return RemoteLintResult{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, hubURL+remoteLintPath, bytes.NewReader(body))
	if err != nil {
		return RemoteLintResult{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token.Value)
	response, err := remoteHTTPClient(service.Client).Do(request)
	if err != nil {
		return failed(NewInvalidRequestError(fmt.Sprintf("nothing answered at %s: %s", hubURL, probeFailureReason(err)), "Check the connection with `skillstation doctor`."))
	}
	defer response.Body.Close()
	answer, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode != http.StatusOK {
		return failed(remoteRefusal(hubURL, response.StatusCode, answer, "lint"))
	}
	var reply RemoteLintReply
	if json.Unmarshal(answer, &reply) != nil || reply.SkillID != resolved.id {
		return failed(NewInvalidRequestError(fmt.Sprintf("the hub at %s answered, but not as a skillstation that checks routing", hubURL), "Check that the hub runs the same skillstation version as this machine."))
	}
	if reply.Findings == nil {
		reply.Findings = []SkillLintFinding{}
	}
	summary := fmt.Sprintf("%s: no routing findings against the hub at %s.", reply.SkillID, hubURL)
	if len(reply.Findings) > 0 {
		summary = fmt.Sprintf("%s: %s against the hub at %s.", reply.SkillID, pluralize(len(reply.Findings), "routing finding"), hubURL)
	}
	return RemoteLintResult{
		Result: NewResult(StatusOK, summary), SkillID: reply.SkillID, Hub: hubURL, Folder: filepath.Clean(resolved.dir),
		Description: reply.Description, Routing: reply.Routing, Findings: reply.Findings,
	}, nil
}
