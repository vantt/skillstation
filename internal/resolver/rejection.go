package resolver

import (
	"context"
	"strings"
)

// maxReResolutions is how many times one task may be resolved again after its
// answer was rejected. A changed task description starts a new task.
const maxReResolutions = 2

// taskDigest identifies the task a request describes. Two requests describe
// the same task when their normalized descriptions match, ignoring case; a
// corrected scope, constraint or fact does not make a new task.
func taskDigest(request Request) string {
	return fingerprint(strings.ToLower(request.Task.Description))
}

// rejectionChain checks a rejected prior and returns what the new answer
// carries: the rejected skills to hide and the re-resolutions used. The prior
// is recalled from memory, or, when memory does not know it, recognized by
// replaying the request without prior. A rejection of an answer for another
// task starts a new chain.
func (resolver *Resolver) rejectionChain(ctx context.Context, request Request, memory PriorMemory) (RejectionChain, error) {
	prior := request.Prior
	record, known := PriorRecord{}, false
	if memory != nil {
		record, known = memory(prior.ResolutionID)
	}
	if !known {
		original := request
		original.Prior = nil
		issued, err := resolver.resolve(ctx, original, nil)
		if err != nil {
			return RejectionChain{}, err
		}
		if issued.ResolutionID != prior.ResolutionID {
			return RejectionChain{}, requestError(ErrUnknownPrior, "prior.resolution_id names no answer this hub remembers for this caller, and resending the request without prior gives another answer")
		}
		record = PriorRecord{ContextRevision: issued.ContextRevision, Chain: issued.Chain}
		if issued.Primary != nil {
			record.PrimaryID = issued.Primary.ID
		}
	}
	if record.ContextRevision != prior.ContextRevision {
		return RejectionChain{}, requestError(ErrStalePrior, "prior.context_revision does not match the answer prior.resolution_id names")
	}
	if record.Chain.TaskDigest != taskDigest(request) {
		return RejectionChain{}, nil
	}
	rejections := record.Chain.Rejections + 1
	if rejections > maxReResolutions {
		return RejectionChain{}, requestError(ErrRetryExhausted, "the answers for this task were rejected %d times", rejections)
	}
	hidden := uniqueSorted(append(append([]string(nil), record.Chain.Hidden...), record.PrimaryID))
	return RejectionChain{Hidden: hidden, Rejections: rejections}, nil
}

// hideRejected removes the skills a rejection chain hides from the active
// skills, except a skill the task description names by id or one-word alias:
// naming it is new evidence for it. A hidden skill leaves the comparison
// entirely, so the next best skill can win.
func hideRejected(byID map[string]Skill, request Request, hidden []string) {
	if len(hidden) == 0 {
		return
	}
	query := tokenize(request.Task.Description)
	for _, id := range hidden {
		if skill, ok := byID[id]; ok && !explicitIdentity(skill, query) {
			delete(byID, id)
		}
	}
}

// rejectionKey is the part of a cache key a rejection chain adds. It is nil
// without a rejection, so the key, and the resolution ID, of every other
// request stay as they were.
func rejectionKey(chain RejectionChain) *RejectionChain {
	if len(chain.Hidden) == 0 && chain.Rejections == 0 {
		return nil
	}
	return &RejectionChain{Hidden: chain.Hidden, Rejections: chain.Rejections}
}
