# Phase 6: Docs, curator skill, contracts

## Requirements

- `docs/curating-skills.md`: one model ("one official version, at most one draft per project, one
  door"), where to change what (SKILL.md, `skill.yaml`), what needs content re-approval, drafts closing,
  discarding from the web. Replace the "One copy per skill" section.
- `docs/command-reference.md`, CLI help: checkout/create/commit/pull wording; `skill.yaml`.
- `docs/web-ui.md`: Drafts screen.
- Curator skill (`internal/systemskills/curator/SKILL.md` and `system-skills/curator/SKILL.md`): edit
  `skill.yaml` for routing, the draft rule, never write `.meta/`; record the shipped digest in
  `known-digests.go` if a release shipped the previous text.
- Schemas and JSON contracts updated with the new files and fields.

## Validation

- Docs examples match real output from a built binary.
- `go test ./...`, web build and tests.

## Implementation notes (2026-10-25)

- Updated: curating-skills (new "Where a skill's files live"), command-reference, web-ui, web-api
  contract, design 07 and remote-hub, CLI help, curator SKILL.md (1.11.0, both copies) and its known
  digests (two committed versions were missing), schemas `skill-settings` and `skill-hub` (the merged
  `skill-metadata` schema stays for layouts 3–5 and the combined view).
