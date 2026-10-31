# `skill_resolve` case catalogue

Every case of the [`skill_resolve` protocol](skill-resolve-protocol.md), with
its trigger, what the server does, what comes back and what the agent does
next. Field and code meanings are in the protocol's
[message reference](skill-resolve-protocol.md#message-reference).

**Diagram** names the sequence diagram that shows the case:
[D1 happy path](skill-resolve-protocol.md#happy-path-resolve-get-use-feedback),
[D2 clarification](skill-resolve-protocol.md#needs-clarification-question-answer-prior),
[D3 no skill](skill-resolve-protocol.md#no-skill-fits),
[D4 rejected](skill-resolve-protocol.md#rejected-recommendation),
[D5 snapshot expired](skill-resolve-protocol.md#snapshot-expired-and-reload),
[D6 review and preflight](skill-resolve-protocol.md#review-required-preflight-check-and-setup),
[D7 remote hub](skill-resolve-protocol.md#remote-hub-through-the-proxy).

**Evidence** says how the case was checked: `live` means it was reproduced
against a test hub with the current binary; `test` means a repository test
covers it; `code` means it was read from the source only and not run.

## Before the call

| ID | Trigger | Server does | Status and codes | Agent next | Diagram | Evidence |
|---|---|---|---|---|---|---|
| B1 | A new task starts | Nothing yet | — | Call `skill_resolve` | D1 | code (instructions) |
| B2 | The task's operation, scope or constraints change | Nothing yet | — | Call `skill_resolve` again with the new task | D1 | code (instructions) |
| B3 | A trivial edit, a new turn, a tool call | Nothing | — | Do not call | — | code (instructions) |

## Resolved

| ID | Trigger | Server does | Status and codes | Agent next | Diagram | Evidence |
|---|---|---|---|---|---|---|
| R1 | Strong match on triggers, operation and description | Scores the leader at or above the high-confidence threshold | `resolved`, `confidence: high`, e.g. `task_match`, `trigger_match` | `skill_get` the primary, use it | D1 | live |
| R2 | A match above the floor but below the high threshold | Same, lower score | `resolved`, `confidence: medium` | Check `applicability` against the task before using it | D1 | live |
| R3 | The primary declares supporting skills for this `operation` | Adds up to two that score above the supporting floor and pass hard checks; drops any that cannot be served | `resolved`, `supporting[]` with `role`, `activation: on-demand` | Load a supporting skill only when its role is needed | D1 | live |
| R4 | Two close candidates declared equivalent, with a preference | Picks the preferred one | `resolved`, `canonical_equivalent_preference`, warning `equivalent_candidates_canonicalized` | Use the primary | D1 | test |
| R5 | The description names an active skill id ("Use the code-review skill") | Adds `identity_match`; the skill clears the floor if no penalty applies | `resolved`, `identity_match` | Use the primary | D1 | live, test |
| R6 | The description names a one-word alias | Smaller identity score than an id | `resolved`, `identity_match` | Use the primary | D1 | test |
| R7 | The asking project holds a checkout (draft) of the skill | Routes the draft for this request only, outside the shared cache | `resolved`, `primary.checkout` set | Use the checkout folder | D1 | code |
| R8 | The top-ranked skill cannot be served | Removes it and decides again | The next result: `resolved` with another skill, or `no_skill` | Act on the new status | D1 | code |

## Needs context

| ID | Trigger | Server does | Status and codes | Agent next | Diagram | Evidence |
|---|---|---|---|---|---|---|
| Q1 | The leader has `min_scope` and `task.scope` is absent | Asks one scope question | `needs_context`, `missing_scope` | Answer from context or the user; resend with `prior.kind: clarification` | D2 | live |
| Q2 | The leader requires a capability you did not declare | Asks one capability question | `needs_context`, `missing_required_capability` | Same | D2 | test |
| Q3 | The leader requires a fact you did not send | Asks one fact question | `needs_context`, `missing_required_fact` | Same | D2 | code |
| Q4 | Two close candidates linked by `distinguish_from`, and the request does not pick one | Asks the curated question | `needs_context`, `missing_discriminator` | Same | D2 | code |
| Q5 | The answer is one of the choices | Replays the original, folds the answer in, resolves | `resolved` or `no_skill`, `context_revision` + 1 | Act on the new status | D2 | live |
| Q6 | The answer is `unknown` | Resolves without the missing evidence; never asks again | `no_skill` (for example `capability_unavailable` or `missing_scope`) | Continue without a skill | D2, D3 | test |
| Q7 | The answer is not one of the choices | Refuses | error `invalid_request` | Resend with a valid choice | D2 | live |
| Q8 | `prior.resolution_id` was not issued for this request | Replay gives another id | error `unknown_resolution` | Start a new resolution | D2 | live, test |
| Q9 | `context_revision` or `question_id` does not match | Refuses | error `stale_context` | Start a new resolution with current context | D2 | live, test |
| Q10 | The catalog changed between question and answer | A prior needs a current catalog | error `index_stale` while stale; after a rebuild the replay gives a new id, so `unknown_resolution` | Resolve again without the prior | D2 | live (index_stale), code (after rebuild) |
| Q11 | `prior.kind: clarification` without `question_id` or `answer` | Normalization refuses | error `invalid_request`, MCP and CLI, naming both fields | Add both fields | D2 | test |
| Q12 | A clarification `prior` on `context_revision` 2 or more (the task already had its question) | Refuses before replaying | error `clarification_budget_exhausted` | Narrow the task or continue without a skill | D2 | test |

## No skill

| ID | Trigger | Server does | Status and codes | Agent next | Diagram | Evidence |
|---|---|---|---|---|---|---|
| N1 | No active skill matches, or the hub has no active skills | Finds no candidate | `no_skill: catalog_gap` | Continue without a skill | D3 | live (gap), code (empty hub) |
| N2 | The best match is weak (for example a multi-word alias only) | Top score below the floor | `no_skill: below_applicability_floor` | Same | D3 | live |
| N3 | `task.exclusions` or a "do not use X" constraint names the only match | Hard-excludes it | `no_skill: constraint_conflict` | Same, or change the exclusions | D3 | live |
| N4 | The task strongly resembles a skill's `not_for` or counter-example | Hard-excludes it, even when named | `no_skill: constraint_conflict` | Same | D3 | test |
| N5 | A weaker `not_for` resemblance | Lowers the score | Often `no_skill: below_applicability_floor` | Same | D3 | live |
| N6 | A required capability is in `unavailable_capabilities` | Hard-excludes | `no_skill: capability_unavailable` | Same | D3 | test |
| N7 | A user, tool or host fact contradicts a requirement | Hard-excludes | `no_skill: constraint_conflict` | Same | D3 | code |
| N8 | `task.scope` is smaller than the match's `min_scope` | Hard-excludes | `no_skill: below_min_scope` | Same, or resolve again if the scope was wrong | D3 | code |
| N9 | Evidence is missing but no question may be asked (a prior is present) | Abstains | `no_skill: missing_scope`, `capability_unavailable` or `catalog_gap` | Same | D3 | code |
| N10 | Two close matches with no curated question | Abstains | `no_skill: unresolved_ambiguity` | Same, or make the description more specific | D3 | code |
| N11 | `activation_context.mode` is `supplement-only` or `coverage-check`, or a locked, non-replaceable primary is active | Refuses a new primary | `no_skill: constraint_conflict`, `active_primary_conflict`, `retry_when: activation_context_changes` | Keep the active procedure | D3 | live |

## Already covered

| ID | Trigger | Server does | Status and codes | Agent next | Diagram | Evidence |
|---|---|---|---|---|---|---|
| A1 | An active primary procedure, not `agent-reported`, whose summary and scope overlap the task | Reports coverage | `already_covered`, `active_procedure_coverage`, `covered_by` | Keep using that procedure | — | live |

## Rejection

| ID | Trigger | Server does | Status and codes | Agent next | Diagram | Evidence |
|---|---|---|---|---|---|---|
| J1 | The agent rejects the primary and resends the same task, unchanged or with a corrected scope, constraint or fact, with `prior.kind: rejected` | Finds the rejected answer in the session or by replay, hides the rejected skill, counts one re-resolution; records a rejection case (`verified_reformulation` when the prior was issued in this session, else `rejected`) | `resolved` with another skill, or `no_skill` (often `catalog_gap` or `below_applicability_floor`), `context_revision` + 1 | Act on the new status | D4 | test |
| J2 | The description of the same task names the rejected skill by id or one-word alias | Keeps the named skill in the running | It may be `resolved` again | Act on the new status | D4 | test |
| J3 | The rejection comes with a changed task description | A new task: hides nothing, starts the count again | Normal statuses, `context_revision` + 1 | Act on the new status | D4 | test |
| J4 | A third rejection of the same task | Refuses | error `resolution_retry_exhausted` | Continue without a hub skill, or change the task description | D4 | test |
| J5 | A rejected prior the session does not remember, with a request that is not the one it was issued for (for example from the CLI or another session, or after two hours) | Replay gives another id | error `unknown_resolution` | Resolve again without `prior` | D4 | test |
| J6 | Rejected prior plus an exclusion of the rejected skill | Hides it and hard-excludes it | `resolved` with another skill, or `no_skill: constraint_conflict` | Act on the new status | D4 | code |
| J7 | The agent used another skill | — | — | `skill_feedback` with `outcome: rejected` and the real `selected_skill` | D4 | code (instructions) |

## Loading the skill

| ID | Trigger | Server does | Status and codes | Agent next | Diagram | Evidence |
|---|---|---|---|---|---|---|
| G1 | `skill_get` on a recommended active skill | Exports a verified read-only copy | `local.status: ready`, `path`, `env`, `resources` | Use the skill | D1 | live |
| G2 | The skill was archived, deprecated or removed after the resolve, or the id is unknown | Refuses without content | error `not_found` | Resolve again | D1 | live |
| G3 | The skill is active but its files cannot be delivered | Refuses without content | error `not_servable` | Choose another skill from a new resolve | D1 | code |
| G4 | The skill was edited after the resolve | Serves the current version | `skill_get` succeeds with new content | Use it | D5 | live |
| G5 | `skills/get` or `resources/read` with a URI pinned to an old version | Refuses | JSON-RPC `-32602`, `data.code: snapshot_expired`, `data.current_uri` | Load `current_uri`, or `skill_get` by id | D5 | live, test |
| G6 | `local.path` disappeared during a long session | — | — | Call `skill_get` again | D5 | code (instructions) |
| G7 | Third-party content not approved | Returns no content, no path | `local.status: review_required`, `review_command`; `resources/read` gives `content_review_required` | Do not use; tell the user to run `skillstation skill review <id>` | D6 | code |
| G8 | The copy could not be made | Degrades instead of failing | `local.status: unavailable` with `not_servable`, `resource_content_unavailable` or `export_failed` | Do not run scripts; resolve again later | D6 | code |

## Preflight, setup and secrets

| ID | Trigger | Server does | Status and codes | Agent next | Diagram | Evidence |
|---|---|---|---|---|---|---|
| P1 | The skill has a runtime block | Adds `local.preflight` with `check`, `setup`, `requires`; checks only the platform | `primary.setup.state` such as `unknown` with `doctor_not_run` | Load the env file, export `local.env`, run `check` | D6 | live |
| P2 | `check` fails | — | — | Ask the user before `setup`; install into `SKILLSTATION_STATE_DIR` only | D6 | code (instructions) |
| P3 | `$SKILLSTATION_STATE_DIR/.setup.lock` is recent | — | — | Do not run `setup` | D6 | code (instructions) |
| P4 | A required secret is missing | The hub does not check variables | — | Tell the user to run `skillstation skill env set <id> <NAME>`; never ask in chat | D6 | code (instructions) |
| P5 | A script fails on a missing dependency | Records the feedback | feedback `status: ok` | Stop, tell the user, send `outcome: failed`, `reason_code: setup_failed` | D6 | live |
| P6 | The platform is not supported | Marks the hint | `setup.state: unsupported_platform`, `platform_unsupported` | Do not run the skill here | D6 | code |

## Feedback

| ID | Trigger | Server does | Status and codes | Agent next | Diagram | Evidence |
|---|---|---|---|---|---|---|
| F1 | Valid feedback | Records local telemetry | `status: ok`, `deduplicated: false` | — | D1 | live |
| F2 | The same `event_id` and content again | Returns the stored event | `deduplicated: true` | — | D1 | live |
| F3 | The same `event_id` with other content | Refuses | error `invalid_request` | Use a new `event_id` | D1 | live |
| F4 | Unknown `resolution_id` | Refuses | error `unknown_resolution` | Use the id from the resolve answer | D1 | live |
| F5 | `selected_skill` neither recommended nor on the hub | Refuses | error `invalid_request` | Use a real skill id, or leave it out | D1 | live |
| F6 | `utility` without `basis` | Refuses | error `invalid_request` | Send both or neither | D1 | live |

## Malformed requests and failures

| ID | Trigger | Server does | Status and codes | Agent next | Diagram | Evidence |
|---|---|---|---|---|---|---|
| E1 | An unknown field, a wrong type, a length or count over the limit | Schema validation refuses | `isError` text starting `validating "arguments"`; CLI `invalid_request` | Fix the request | — | live, test |
| E1a | `schema_version` other than `"1"`, with or without fields of that version | Checks the version first | error `unsupported_schema`, MCP and CLI | Send `schema_version` `"1"` | — | test |
| E2 | Blank description, unsafe `path_hint`, a capability in both lists | Normalization refuses | error `invalid_request` | Fix the request | — | live |
| E3 | A `prior` call while the catalog is stale or missing | Refuses | error `index_stale`; the action says to resolve again without `prior`, then to tell the user to run `skillstation doctor --fix` | Resolve again without `prior` | D2 | live (error code), test (action) |
| E4 | The host cancels the call | Stops | error `operation_cancelled` | Retry when needed | — | code |
| E5 | Any unexpected failure | Logs it with a correlation id | error `internal_error`, `correlation_id` | Retry once | — | code |

## Cache and catalog changes

| ID | Trigger | Server does | Status and codes | Agent next | Diagram | Evidence |
|---|---|---|---|---|---|---|
| C1 | The same request again (any `request_id`) on the same catalog | Cache hit | The same answer and `resolution_id`, with your `request_id` | Nothing different | D1 | live |
| C2 | First time a request is seen | Cache miss, full pipeline | Same answer a hit would give | Nothing different | D1 | live |
| C3 | A skill changes while no request runs | Next request without a prior rebuilds a stale catalog | New `catalog_snapshot`, possibly a new answer | Use the new answer | — | live |
| C4 | The Git files are invalid when a rebuild is needed | Serves the last good generation | Normal status plus a fallback warning | Use the answer; tell the user if the warning persists | — | code |
| C5 | A rebuild publishes while a request runs | The request finishes on the generation it opened | Answer pinned to the old `catalog_snapshot` | A later `skills/get` may get `snapshot_expired` (G5) | D5 | code |

## Remote hub

| ID | Trigger | Server does | Status and codes | Agent next | Diagram | Evidence |
|---|---|---|---|---|---|---|
| H1 | `skill_get` for a version already cached | Answers from the cache | `remote.cache: hit` | Use the skill | D7 | code |
| H2 | `skill_get` for a new version | Fetches the bundle, verifies its digest | `remote.cache: fetched` | Use the skill | D7 | code |
| H3 | The hub cannot be reached | `skill_resolve` fails; `skill_get` falls back to the newest cached copy | resolve: `hub_unreachable`; get: `remote.cache: offline` with a message, or `hub_unreachable` | Continue without a skill, or use the cached copy | D7 | code, guide |
| H4 | The hub refuses the token | Stops forwarding | `hub_token_rejected` | Tell the user to copy the current token file | D7 | code |
| H5 | The hub rate-limits | Retries briefly | `hub_busy` if still busy | Retry in a few seconds | D7 | code |
| H6 | The hub answers but cannot serve the skill | No cache fallback | `hub_error` | Continue without the skill | D7 | code |
| H7 | The hub no longer serves the skill | Removes its cached copies, forwards `skill_get` | the hub's `not_found` | Resolve again | D7 | code |
| H8 | The skill awaits review, is a draft, or is the curator | Forwards `skill_get` to the hub | the hub's own answer | As G7 | D7 | code |
| H9 | A bundle fails its digest check | Discards it | `resource_digest_mismatch` | Retry once | D7 | code |

## Sources

[`resolver.go`](../../internal/resolver/resolver.go),
[`rejection.go`](../../internal/resolver/rejection.go),
[`errors.go`](../../internal/resolver/errors.go),
[`evidence.go`](../../internal/resolver/evidence.go),
[`normalize.go`](../../internal/resolver/normalize.go),
[`app/resolver.go`](../../internal/app/resolver.go),
[`app/feedback.go`](../../internal/app/feedback.go),
[`app/skill_snapshot.go`](../../internal/app/skill_snapshot.go),
[`app/skill_doctor.go`](../../internal/app/skill_doctor.go),
[`app/error_classify.go`](../../internal/app/error_classify.go),
[`app/resolve_errors.go`](../../internal/app/resolve_errors.go),
[`resolver_tools.go`](../../internal/delivery/mcpserver/resolver_tools.go),
[`skill_tools.go`](../../internal/delivery/mcpserver/skill_tools.go),
[`skills_extension.go`](../../internal/delivery/mcpserver/skills_extension.go),
[`instructions.go`](../../internal/delivery/mcpserver/instructions.go),
[`proxy.go`](../../internal/delivery/mcpserver/proxy.go),
[`resolver_test.go`](../../internal/resolver/resolver_test.go),
[`rejection_test.go`](../../internal/resolver/rejection_test.go),
[`resolve_protocol_test.go`](../../internal/delivery/mcpserver/resolve_protocol_test.go),
[`snapshot_expired_test.go`](../../internal/delivery/mcpserver/snapshot_expired_test.go),
[`cli/resolver_test.go`](../../internal/delivery/cli/resolver_test.go),
[remote hub guide](../remote-hub-guide.md#when-the-hub-or-the-tunnel-is-down).
