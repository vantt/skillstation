package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/vantt/skillstation/internal/digest"
	"github.com/vantt/skillstation/internal/skill"
	"github.com/vantt/skillstation/internal/skillmeta"
	"gopkg.in/yaml.v3"
)

// RemoteEditInput is a change to a skill of a hub on another machine, made
// from a project without a checkout in it.
type RemoteEditInput struct {
	SkillID string
	// Update carries the description and the label, topic and technology
	// lists. Its Routing is left nil: Routing below sets it.
	Update skill.UpdateInput
	// Routing, when set, is given the routing the skill has now and returns
	// the routing to write, so a change that names some fields keeps the rest.
	Routing func(current skill.RoutingInput) *skill.RoutingInput
}

// RemoteEditPreview is what an edit would send to the hub, before anything is.
type RemoteEditPreview struct {
	RemoteCommitPreview
	// Diff is the change to each file that differs from the hub's copy.
	Diff string `json:"diff,omitempty"`
	// RoutingBefore and RoutingAfter are the routing the hub holds and the
	// routing this edit would leave. A routing flag replaces its whole field,
	// so a person or an agent reads the first to know what to repeat.
	RoutingBefore skill.RoutingInput `json:"routing_before"`
	RoutingAfter  skill.RoutingInput `json:"routing_after"`
}

// RemoteEditService edits a skill of a hub on another machine without leaving
// a checkout in the project: it fetches the skill, applies the change in
// memory, and sends the result to the hub's review queue like a commit does.
//
// Its fields are those of RemoteCommitService, which sends what it prepares.
type RemoteEditService struct {
	Client *http.Client
	// Label and Note name this machine and say why, as for a commit.
	Label, Note string
}

func remoteEditError(err *Error) RemoteEditPreview {
	return RemoteEditPreview{RemoteCommitPreview: remotePreviewError(err)}
}

// Preview fetches the skill and shows what the change would send. It reads
// the hub, with the hub token, and writes nothing there or here.
func (service RemoteEditService) Preview(ctx context.Context, hub RemoteHub, input RemoteEditInput) (RemoteEditPreview, error) {
	id := strings.TrimSpace(input.SkillID)
	if !skillmeta.ValidSkillID(id) {
		return remoteEditError(NewInvalidRequestError(fmt.Sprintf("invalid skill id %q", id), "Use the lowercase id shown by `skillstation skill list` on the hub machine.")), nil
	}
	hubURL, problem := NormalizeConnectHubURL(hub.URL)
	if problem != nil {
		return remoteEditError(problem), nil
	}
	token := ReadHubToken(hubURL, hub.TokenFile)
	if token.Problem != nil {
		return remoteEditError(NewInvalidRequestError(token.Problem.Detail, token.Problem.Fix)), nil
	}
	fetched, bundleDigest, rawDetail, refusal, err := (RemoteCheckoutService{Client: service.Client}).fetchWithDetail(ctx, hubURL, token.Value, id)
	if err != nil {
		return RemoteEditPreview{}, err
	}
	if refusal != nil {
		return remoteEditError(refusal), nil
	}
	before := treeBytes(fetched)
	working := before
	var seeded []byte
	update := input.Update
	if input.Routing != nil || update.Labels != nil || update.Topics != nil || update.Technologies != nil {
		// The skill.yaml the submission carries replaces what the hub keeps in it,
		// so a folder without one gets the whole of what the hub holds first.
		if _, has := before["skill.yaml"]; !has {
			var problem *Error
			if seeded, problem = seedSkillYAML(rawDetail); problem != nil {
				return remoteEditError(problem), nil
			}
			working = make(map[string][]byte, len(before)+1)
			for rel, data := range before {
				working[rel] = data
			}
			working["skill.yaml"] = seeded
		}
	}
	if input.Routing != nil {
		update.Routing = input.Routing(skill.RoutingOfFiles(working))
	}
	editedFiles, err := skill.EditFiles(working, id, update)
	if errors.Is(err, skill.ErrEditNeedsHub) {
		return remoteEditError(NewInvalidRequestError(err.Error(), "Check the skill out with `skillstation skill checkout "+id+"`, edit the files, and send them with `skillstation skill commit "+id+"`.")), nil
	}
	if err != nil {
		return remoteEditError(NewInvalidRequestError(err.Error(), "Check the values you passed, or check the skill out and edit its files.")), nil
	}
	if seeded != nil && bytes.Equal(editedFiles["skill.yaml"], seeded) {
		// The change left every value the hub holds as it was, so the folder needs no skill.yaml.
		delete(editedFiles, "skill.yaml")
	}
	edited := make(map[string]treeFile, len(editedFiles))
	for rel, data := range editedFiles {
		edited[rel] = treeFile{Data: data, Executable: fetched[rel].Executable}
	}
	baseFiles := make(map[string]string, len(fetched))
	for rel, file := range fetched {
		baseFiles[rel] = digest.Bytes(file.Data)
	}
	files, total := describeRemoteFiles(edited, baseFiles)
	preview := RemoteEditPreview{RemoteCommitPreview: RemoteCommitPreview{
		SkillID: id, Hub: hubURL, CheckedOut: true, BaseVersion: skillBaseVersion(fetched), Files: files, Excluded: []ExcludedFile{},
		TotalBytes: total, MaySendToken: hub.MaySendToken, NothingToCommit: len(files) == 0, Note: CleanSubmissionNote(service.Note),
		tree: edited, baseBundle: bundleDigest,
	}}
	preview.RoutingBefore = skill.RoutingOfFiles(working)
	preview.RoutingAfter = preview.RoutingBefore
	if _, written := editedFiles["skill.yaml"]; written {
		preview.RoutingAfter = skill.RoutingOfFiles(editedFiles)
	}
	var diff strings.Builder
	for _, file := range files {
		diff.WriteString(fileDiff(file.Path, file.Change, before[file.Path], editedFiles[file.Path]))
	}
	preview.Diff = diff.String()
	if preview.NothingToCommit {
		preview.Result = NewResult(StatusOK, fmt.Sprintf("Nothing to change: %s already has these values.", id))
		return preview, nil
	}
	preview.Result = NewResult(StatusActionRequired, fmt.Sprintf("%s: this changes %s; it sends the result to the hub at %s for review.", id, pluralize(len(files), "file"), hubURL))
	return preview, nil
}

// seededSkillYAML is the skill.yaml of a folder made from what the hub holds,
// with its keys in the order a person writes them.
type seededSkillYAML struct {
	SchemaVersion int                `yaml:"schema_version"`
	Routing       skill.RoutingInput `yaml:"routing"`
	Labels        []string           `yaml:"labels,omitempty"`
	Topics        []string           `yaml:"topics,omitempty"`
	Technologies  []string           `yaml:"technologies,omitempty"`
}

// seedSkillYAML writes the skill.yaml a project folder would hold for a skill
// the hub describes: its routing and its labels, topics and technologies. The
// bundle carries that description beside the files.
func seedSkillYAML(rawDetail json.RawMessage) ([]byte, *Error) {
	var detail struct {
		Routing      skill.RoutingInput `json:"routing"`
		Labels       []string           `json:"labels"`
		Topics       []string           `json:"topics"`
		Technologies []string           `json:"technologies"`
	}
	if len(rawDetail) == 0 || json.Unmarshal(rawDetail, &detail) != nil {
		return nil, NewInvalidRequestError("the hub did not send the routing this skill has now, so a change to it could erase what the hub keeps",
			"Update the hub to this skillstation version, or check the skill out and send it with `skillstation skill checkout` and `skillstation skill commit`.")
	}
	encoded, err := yaml.Marshal(seededSkillYAML{
		SchemaVersion: 1, Routing: detail.Routing, Labels: detail.Labels, Topics: detail.Topics, Technologies: detail.Technologies,
	})
	if err != nil {
		return nil, NewInvalidRequestError("the routing of the skill cannot be written: "+err.Error(), "Check the skill out and edit its files.")
	}
	return encoded, nil
}

// Send sends a previewed edit to the hub's review queue, as a commit does.
func (service RemoteEditService) Send(ctx context.Context, hub RemoteHub, preview RemoteEditPreview) (RemoteCommitResult, error) {
	return RemoteCommitService(service).Send(ctx, hub, preview.RemoteCommitPreview)
}
