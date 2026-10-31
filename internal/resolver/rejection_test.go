package resolver

import (
	"context"
	"errors"
	"testing"
)

// rejectionSkills are three review skills that all fit a review of consumer
// retries, each less than the one before.
func rejectionSkills() []Skill {
	return []Skill{
		{ID: "consumer-review", CollectionID: "core", Name: "Consumer Review", Status: "active", Digest: "v1", Reviewed: true,
			Description: "Review kafka consumer retries and acknowledgements", Operations: []string{"review"},
			Triggers: []string{"review kafka consumer retries"}},
		{ID: "code-review", CollectionID: "core", Name: "Code Review", Status: "active", Digest: "v1", Reviewed: true,
			Description: "Review code changes in a service for defects", Operations: []string{"review"},
			Triggers: []string{"review service code"}},
		{ID: "retry-audit", CollectionID: "core", Name: "Retry Audit", Status: "active", Digest: "v1",
			Description: "Audit retry settings", Operations: []string{"review"}},
	}
}

// sessionMemory remembers the answers of one caller session, as the MCP
// session tracker does.
type sessionMemory map[string]PriorRecord

func (memory sessionMemory) note(response Response) {
	record := PriorRecord{ContextRevision: response.ContextRevision, Chain: response.Chain}
	if response.Primary != nil {
		record.PrimaryID = response.Primary.ID
	}
	memory[response.ResolutionID] = record
}

func (memory sessionMemory) recall(id string) (PriorRecord, bool) {
	record, ok := memory[id]
	return record, ok
}

func rejecting(request Request, answer Response) Request {
	request.Prior = &Prior{ResolutionID: answer.ResolutionID, ContextRevision: answer.ContextRevision, Kind: "rejected", ReasonCode: "scope_mismatch"}
	return request
}

func primaryOf(response Response) string {
	if response.Primary == nil {
		return ""
	}
	return response.Primary.ID
}

// TestRejectedAnswerLeavesTheNextResolutionOfTheSameTask pins that a rejection
// hides the rejected skill for the same task until new evidence names it, and
// that one task gets two re-resolutions.
func TestRejectedAnswerLeavesTheNextResolutionOfTheSameTask(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	engine, err := New(memoryCatalog{rejectionSkills(), "sha256:" + repeat("7", 64)}, DefaultPolicy(), NewCache(32))
	if err != nil {
		t.Fatal(err)
	}
	base := Request{SchemaVersion: "1", RequestID: "first", Task: Task{Description: "review kafka consumer retries in the billing service code", Scope: "multi_step"}, Operation: "review"}
	resolve := func(t *testing.T, request Request, memory sessionMemory) Response {
		t.Helper()
		var response Response
		var err error
		if memory == nil {
			response, err = engine.Resolve(ctx, request)
		} else {
			response, err = engine.ResolveRemembering(ctx, request, memory.recall)
		}
		if err != nil {
			t.Fatal(err)
		}
		if memory != nil {
			memory.note(response)
		}
		return response
	}
	first := resolve(t, base, nil)
	if primaryOf(first) != "consumer-review" {
		t.Fatalf("first answer = %s %v, want consumer-review", first.Status, primaryOf(first))
	}

	t.Run("resending the same task hides the rejected skill", func(t *testing.T) {
		again := resolve(t, rejecting(base, first), nil)
		if again.Status != StatusResolved || primaryOf(again) == "consumer-review" || again.ContextRevision != 2 {
			t.Fatalf("after a rejection = %s %q revision %d, want another skill at revision 2", again.Status, primaryOf(again), again.ContextRevision)
		}
	})

	t.Run("two re-resolutions, then the budget is exhausted", func(t *testing.T) {
		memory := sessionMemory{}
		answer := resolve(t, base, memory)
		seen := map[string]bool{primaryOf(answer): true}
		for round := 1; round <= maxReResolutions; round++ {
			// A corrected scope is the same task.
			corrected := base
			corrected.Task.Scope = []string{"multi_step", "project", "single_step"}[round%3]
			answer = resolve(t, rejecting(corrected, answer), memory)
			if seen[primaryOf(answer)] && primaryOf(answer) != "" {
				t.Fatalf("round %d returned %q again", round, primaryOf(answer))
			}
			seen[primaryOf(answer)] = true
		}
		_, err := engine.ResolveRemembering(ctx, rejecting(base, answer), memory.recall)
		if !errors.Is(err, ErrRetryExhausted) {
			t.Fatalf("third rejection error = %v, want the re-resolution budget exhausted", err)
		}
	})

	t.Run("a changed description is a new task", func(t *testing.T) {
		memory := sessionMemory{}
		answer := resolve(t, base, memory)
		changed := base
		changed.Task.Description = "review kafka consumer retries and dead letter handling"
		again := resolve(t, rejecting(changed, answer), memory)
		if primaryOf(again) != "consumer-review" || len(again.Chain.Hidden) != 0 || again.Chain.Rejections != 0 {
			t.Fatalf("new task answer = %q chain %+v, want consumer-review and a fresh chain", primaryOf(again), again.Chain)
		}
	})

	t.Run("a task that names the rejected skill keeps it", func(t *testing.T) {
		named := base
		named.Task.Description = "use consumer-review to review kafka consumer retries"
		answer := resolve(t, named, nil)
		again := resolve(t, rejecting(named, answer), nil)
		if primaryOf(answer) != "consumer-review" || primaryOf(again) != "consumer-review" {
			t.Fatalf("named skill answers = %q then %q, want consumer-review twice", primaryOf(answer), primaryOf(again))
		}
	})

	t.Run("an answer that cannot be recalled or replayed is unknown", func(t *testing.T) {
		corrected := rejecting(base, first)
		corrected.Task.Scope = "project"
		if _, err := engine.Resolve(ctx, corrected); !errors.Is(err, ErrUnknownPrior) {
			t.Fatalf("corrected request without memory error = %v, want an unknown prior", err)
		}
	})

	t.Run("a revision the answer did not have is stale", func(t *testing.T) {
		stale := rejecting(base, first)
		stale.Prior.ContextRevision = 2
		if _, err := engine.Resolve(ctx, stale); !errors.Is(err, ErrStalePrior) {
			t.Fatalf("stale revision error = %v, want a stale prior", err)
		}
	})
}

// TestClarificationOnALaterRevisionExhaustsTheBudget pins that a task gets one
// clarification round: questions are asked on the first answer only.
func TestClarificationOnALaterRevisionExhaustsTheBudget(t *testing.T) {
	t.Parallel()
	skill := Skill{ID: "reader", CollectionID: "core", Name: "Reader", Description: "inspect source files", Status: "active", Digest: "v1", Operations: []string{"review"}, Triggers: []string{"inspect source files"}, MinScope: "multi_step", Reviewed: true}
	engine, _ := New(memoryCatalog{[]Skill{skill}, "sha256:" + repeat("8", 64)}, DefaultPolicy(), NewCache(8))
	request := Request{SchemaVersion: "1", RequestID: "late", Task: Task{Description: "inspect source files"}, Operation: "review",
		Prior: &Prior{ResolutionID: "res_any", ContextRevision: 2, Kind: "clarification", QuestionID: "scope-minimum-multi_step", Answer: "multi_step"}}
	if _, err := engine.Resolve(context.Background(), request); !errors.Is(err, ErrClarificationExhausted) {
		t.Fatalf("second clarification round error = %v, want the clarification budget exhausted", err)
	}
}
