package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/vantt/skillstation/internal/fsutil"
)

// The states a submission goes through: it waits in the queue, then leaves it
// as applied (a person approved it and the hub applied it), discarded, or
// replaced by a newer one from the same machine.
const (
	SubmissionWaiting   = "waiting"
	SubmissionApplied   = "applied"
	SubmissionDiscarded = "discarded"
	SubmissionReplaced  = "replaced"
)

const (
	submissionOutcomesDir = ".outcomes"
	// maxKeptOutcomes and outcomeRetention bound how long a sender can still ask
	// what became of a submission that has left the queue.
	maxKeptOutcomes  = 200
	outcomeRetention = 90 * 24 * time.Hour
)

// SubmissionOutcome is the record a submission leaves when it leaves the
// queue: what became of it, so the machine that sent it can ask.
type SubmissionOutcome struct {
	ID          string `json:"submission_id"`
	SkillID     string `json:"skill_id"`
	ClientLabel string `json:"client_label"`
	State       string `json:"state"`
	DecidedAt   string `json:"decided_at"`
	// ReplacedBy names the newer submission when State is "replaced".
	ReplacedBy string `json:"replaced_by,omitempty"`
}

// SubmissionStatus is what a sender is told about one of its submissions.
type SubmissionStatus struct {
	ID          string `json:"submission_id"`
	SkillID     string `json:"skill_id"`
	Status      string `json:"status"`
	SubmittedAt string `json:"submitted_at,omitempty"`
	DecidedAt   string `json:"decided_at,omitempty"`
	ReplacedBy  string `json:"replaced_by,omitempty"`
	// SkillState is the lifecycle state the skill has on the hub now, told only
	// for an applied submission: a new skill is a draft until the hub's owner
	// activates it, while a change to an active skill is live at once.
	SkillState string `json:"skill_state,omitempty"`
}

func outcomesRoot(root string) string {
	return filepath.Join(submissionsRoot(root), submissionOutcomesDir)
}

// recordOutcomeLocked saves what became of submission and drops the records
// that are too old or too many. submissionMu must be held. A record that
// cannot be written is not an error for the caller: the submission has left
// the queue either way, and a sender then hears that it is unknown.
func recordOutcomeLocked(root string, submission Submission, state, replacedBy string, now time.Time) {
	dir := outcomesRoot(root)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	encoded, err := json.MarshalIndent(SubmissionOutcome{
		ID: submission.ID, SkillID: submission.SkillID, ClientLabel: submission.ClientLabel,
		State: state, DecidedAt: now.UTC().Format("2006-01-02T15:04:05Z"), ReplacedBy: replacedBy,
	}, "", "  ")
	if err != nil {
		return
	}
	if err := fsutil.WriteFileAtomic(filepath.Join(dir, submission.ID+".json"), append(encoded, '\n'), 0o600); err != nil {
		return
	}
	pruneOutcomes(dir, now)
}

func pruneOutcomes(dir string, now time.Time) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var kept []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") || !submissionIDPattern.MatchString(strings.TrimSuffix(name, ".json")) {
			continue
		}
		if info, err := entry.Info(); err == nil && now.Sub(info.ModTime()) > outcomeRetention {
			_ = os.Remove(filepath.Join(dir, name))
			continue
		}
		kept = append(kept, name)
	}
	sort.Strings(kept)
	for len(kept) > maxKeptOutcomes {
		_ = os.Remove(filepath.Join(dir, kept[0]))
		kept = kept[1:]
	}
}

// settleSubmissionAt takes submission id out of the queue and records its
// outcome.
func settleSubmissionAt(root, id, state string, now time.Time) error {
	submission, err := ReadSubmission(root, id)
	if err != nil {
		return err
	}
	submissionMu.Lock()
	defer submissionMu.Unlock()
	if err := os.RemoveAll(filepath.Join(submissionsRoot(root), id)); err != nil {
		return err
	}
	recordOutcomeLocked(root, submission, state, "", now)
	return nil
}

// ReadSubmissionStatus says what became of submission id: waiting in the
// queue, or how it left it. An id that is in neither, because it never existed
// or its record is older than the hub keeps, is ErrSubmissionNotFound.
func ReadSubmissionStatus(root, id string) (SubmissionStatus, error) {
	if !submissionIDPattern.MatchString(id) {
		return SubmissionStatus{}, fmt.Errorf("%w: %q is not a submission id", ErrSubmissionNotFound, id)
	}
	if waiting, err := ReadSubmission(root, id); err == nil {
		return SubmissionStatus{ID: id, SkillID: waiting.SkillID, Status: SubmissionWaiting, SubmittedAt: waiting.SubmittedAt}, nil
	} else if !errors.Is(err, ErrSubmissionNotFound) {
		return SubmissionStatus{}, err
	}
	encoded, err := os.ReadFile(filepath.Join(outcomesRoot(root), id+".json"))
	if errors.Is(err, fs.ErrNotExist) {
		return SubmissionStatus{}, fmt.Errorf("%w: %s", ErrSubmissionNotFound, id)
	}
	if err != nil {
		return SubmissionStatus{}, err
	}
	var outcome SubmissionOutcome
	if err := json.Unmarshal(encoded, &outcome); err != nil || outcome.ID != id {
		return SubmissionStatus{}, fmt.Errorf("the outcome record %s cannot be read", id)
	}
	status := SubmissionStatus{ID: id, SkillID: outcome.SkillID, Status: outcome.State, DecidedAt: outcome.DecidedAt, ReplacedBy: outcome.ReplacedBy}
	if outcome.State == SubmissionApplied {
		if detail, err := (SkillService{}).ReadSkillCore(context.Background(), root, outcome.SkillID); err == nil {
			status.SkillState = detail.Status
		}
	}
	return status, nil
}
