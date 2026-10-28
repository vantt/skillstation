---
title: "Rename the product from Skill Hub (skillhub-mcp) to Skillstation"
description: "Rename the product, repository, Go module, command, MCP server name, environment variables, on-disk names and protocol strings from skillhub to skillstation before the launch. Clean break: the new binary reads no old names; existing installs migrate by hand once."
status: completed
priority: P1
effort: 2d
branch: main
tags: [rename, release, selfupdate, breaking-change, launch]
blockedBy: []
blocks: [261014-1700-promote-skillhub-mcp]
created: 2026-10-19
---

# Rename Skill Hub to Skillstation

This plan is executed by an agent on another machine. It is self-contained: read it fully, then re-run every
inventory command below in a fresh worktree; do not trust the counts recorded here.

## Outcome, constraints, non-goals

- **Outcome.** The product is **Skillstation**, the repository is `vantt/skillstation`, the Go module is
  `github.com/vantt/skillstation`, and the command is `skillstation`. Every name an agent, a user or a hub sees
  uses the new name. The first release under the new name is signed, installs on Linux, macOS and Windows, and the
  owner's own hubs and machines run on it.
- **Why.** The name `skillhub` is crowded by projects doing the same thing (iflytek/skillhub, Tencent SkillHub,
  skillhub.club), `skillhub-mcp` is taken on npm and PyPI by other projects, and at least six npm packages install a
  command called `skillhub`. Evidence: [name check](../reports/researcher-261018-0611-name-check.md) and
  [rename candidates](../reports/naming-261018-0804-rename-candidates.md). The launch is blocked on this rename
  ([launch strategy](../reports/launch-strategy-261018-0611-beta-then-public.md)).
- **Decisions (owner, 2026-10-19).**
  - Name: repository `skillstation`, command `skillstation`, product name written `Skillstation`.
  - **Clean break.** The new binary does not read `SKILLHUB_*` variables, `.skillhub/`, `~/.config/skillhub` or any
    other old name, and `connect` does not rewrite old agent entries. Existing installs (essentially only the owner's)
    migrate by hand once with the steps in phase 4. No bridge release.
  - Default hub folder `~/skillstation-hub` (owner changed it from `skillstation-repo` on 2026-10-19); first release under the new name is `v0.8.0`.
  - A website or domain is not required. Buying `skillstation.dev` is optional and is the owner's call.
- **Constraints.**
  - Push, tag, release and the GitHub repository rename happen only after the owner says go for that step.
  - Work in a git worktree created from `origin/main`. The owner's main checkout holds uncommitted video work under
    `video/` and in two video plan files; do not touch, commit or overwrite it.
  - Never rewrite published tags or history. Dated records keep the old name: `archive/**`, `plans/**`,
    `docs/plans/**`, `docs/brainstorm/**`, `docs/distillery/**` and past release notes.
  - Do not put plan ids or phase numbers in code, test names or commit messages. Conventional commits.
  - No secrets, tokens or private hub content in commits, logs or screenshots.
- **Non-goals.** A website, a new visual identity beyond swapping the wordmark text, renaming the MCP tool names
  (`skill_resolve`, `skill_get`, `skill_feedback` keep their names), changing the telemetry `EventVersion` or event
  types, an automatic migration command.

## Name mapping

Apply in this order (longest and most specific first) so a shorter rule does not eat a longer match. Review every
hunk; this is not a blind `sed`.

| Old | New | Where it appears |
|---|---|---|
| `github.com/vantt/skillhub-mcp` | `github.com/vantt/skillstation` | `go.mod`, `web/go.mod`, imports, ldflags, URLs |
| `skillhub-mcp` (other) | `skillstation` | README, docs, workflows, installers, selfupdate constants |
| `skillhub-curation` | `skillstation-curation` | curator MCP server name, host integration, docs |
| `skillhub.local` | `skillstation.local` | `$id` in `schemas/*.json` |
| `skillhub.invalid` | `skillstation.invalid` | CI and test Git identities |
| `SKILLHUB_` | `SKILLSTATION_` | all environment variables, installers, tests, docs |
| `.skillhub` and `.skillhub-*` | `.skillstation` and `.skillstation-*` | hub metadata folder, temp, marker, backup and token files |
| `skillhub` (identifier or string) | `skillstation` | command, `cmd/skillhub/`, MCP `Implementation.Name`, `mcpServers` key, `realm`, `service`, `skill://skillhub/` URI host, telemetry client name, config and state folder names, launchd and systemd service names, permission rules `mcp__skillhub__*` |
| `SkillHub`, `skillHub` (Go identifiers) | `Skillstation`, `skillstation` | Go code; keep Go naming style |
| `Skill Hub` (prose) | `Skillstation` | README, docs, web UI text, help text, MCP server instructions, video script later |
| `skill-hub` default hub folder (`~/skill-hub`, `skill-hub-N`) | `skillstation-hub`, `skillstation-hub-N` | `internal/app/workspace_init.go`, help, docs (owner, 2026-10-19) |
| `skill-hub-logo-*.svg`, `skill-hub-mark-*.svg` | `skillstation-logo-*.svg`, `skillstation-mark-*.svg` | `docs/images/brand/`; update the wordmark text inside the SVG too |

Inventory at planning time (2026-10-19, excluding `plans/` and `archive/`): about 5,060 lines with `skillhub`
(any case) in about 670 files, most in `internal/`; 46 distinct `SKILLHUB_*` variables; `.skillhub` appears 185 times
in Go code. Re-run:

```bash
git grep -ci skillhub -- ':!plans' ':!archive' | sort -t: -k2 -rn | head -40
git grep -ohE 'SKILLHUB_[A-Z_]+' | sort -u
git grep -nE '"skillhub"|"skillhub-curation"|\.skillhub|skill-hub' -- '*.go' ':!*_test.go'
```

## Known identity pins and contracts

- Release signature identity: `internal/selfupdate/selfupdate.go` (`DefaultReleaseBaseURL`,
  `DefaultSigstoreWorkflow`), `scripts/install.sh`, `scripts/install.ps1`, and the installer test scripts
  `scripts/test-installer-lifecycle.sh` and `scripts/test-installer-windows.ps1`. They move to the new repository
  path together; there is no second accepted identity.
- Release workflow: binary name, archive names `skillhub-<version>-<os>-<arch>` and ldflags in
  `.github/workflows/release.yml`; also `.github/workflows/evaluation.yml`, `Makefile` (`web/.e2e/skillhub`).
- Agent integration: `internal/hostintegration/` writes the `mcpServers.skillhub` entry and the permission rules
  (`cli-permissions.go`, `integration.go`, `connection.go`, `preview.go`, `remote.go`).
- Remote protocol between machine and hub: `internal/app/doctor_remote.go` (`service`), `internal/delivery/mcpserver/http.go`
  (`realm`). A new client cannot talk to an old hub and the reverse; this is accepted by the clean break, so every
  machine and hub moves to the same release.
- Skill URIs: `internal/app/distribution.go` checks host `skillhub` in `skill://skillhub/...`.
- User-visible state folders: `internal/userconfig/userconfig.go`, `internal/skillruntime/remote_cache.go`.
- Skills inside hubs may read `$SKILLHUB_SKILL_DIR`, `$SKILLHUB_STATE_DIR` or `$SKILLHUB_CONFIG_DIR` (the MCP server
  instructions tell agents to export them). The bundled curator skill in `system-skills/` and any skill in the
  owner's hubs must use the new names.
- Tests with literal paths (for example the checkout path in `internal/telemetry/redactor_test.go` and
  `internal/transcripts/claude_code_test.go`) describe the local directory, not the product. Change them only if they
  encode the product name.

## Phases

| # | Phase | Owner | Depends on |
|---|-------|-------|-----------|
| 1 | Rename code, tests, scripts and workflows in a worktree | agent | none |
| 2 | Rename docs, README (both languages), web UI text, brand files | agent | phase 1 |
| 3 | Rename the GitHub repository and ship the first release | agent, after owner's go for each remote step | phases 1 and 2 merged locally |
| 4 | Migrate the owner's installs and hubs by hand | owner, agent guides | phase 3 |
| 5 | Follow-ups: video, launch docs, listings | agent and owner | phase 4 |

### Phase 1: code, tests, scripts, workflows

1. `git fetch origin && git worktree add -b rename-skillstation .claude/worktrees/rename-skillstation origin/main`;
   run everything inside the worktree.
2. Module path: change `go.mod` and `web/go.mod` (and any `replace`), then all imports and ldflags. `go mod tidy` in
   the root and in `web/`. Build.
3. `git mv cmd/skillhub cmd/skillstation`; update `Makefile`, workflows and scripts that build it.
4. Apply the rest of the mapping table, one category per commit so review stays readable: environment variables;
   on-disk names; protocol and integration strings; telemetry client name; selfupdate and installers; schemas;
   Go identifiers; help and MCP server instruction text; the curator skill in `system-skills/`.
5. Bump the hub schema only if the metadata folder name is part of a versioned format check (`schema-version` under
   the folder); otherwise the folder rename is enough. Note the choice in the commit message.
6. Verification inside the worktree: `gofmt -l .`; `go vet ./...` with default, `GOOS=windows` and `GOOS=darwin`;
   `make check`; `bash -n scripts/*.sh`; the web e2e target in the `Makefile`; and
   `git grep -i skillhub -- ':!plans' ':!archive' ':!docs/plans' ':!docs/brainstorm' ':!docs/distillery' ':!video'`
   returning only lines reviewed and listed as intended leftovers in the progress section below.

### Phase 2: docs, README, web UI, brand

1. README.md and README.vi.md: new name, install URLs, badges, and one line saying "Skillstation was called Skill Hub
   (`skillhub-mcp`) until v0.8.0" so people arriving from old links understand.
2. Evergreen docs in `docs/` (user guide, installation, remote hub guide, curating, command reference, web UI,
   troubleshooting, release runbook `REPO=`, design docs, contracts, `docs/mcp-compatibility-matrix.json`).
   `AGENTS.md`, `CONTRIBUTING.md`, `.agents/skills/`.
3. Web UI: page titles, header, any "Skill Hub" text; rebuild embedded assets if the build embeds them; update
   screenshots in `docs/images/web/` only if they show the old name.
4. Brand: rename the four SVG files and change the wordmark text; update every reference.
5. Release notes draft for v0.8.0 (breaking): what changed, why, the manual migration steps from phase 4, that
   self-update from older versions fails safe and a one-time reinstall is needed, and that the Go module path changed.

### Phase 3: GitHub rename and release (each step needs the owner's go)

1. Merge the worktree branch into local `main` with a merge commit; remove the worktree and branch.
2. Owner's go: rename the repository in GitHub settings to `skillstation` while no workflow is running. GitHub
   redirects old web, clone, API and download URLs. Do not create a new repository at the old name.
   `git remote set-url origin` in every checkout.
3. Owner's go: push, wait for CI on three platforms, tag `v0.8.0` (breaking change), let the release workflow sign
   and publish. Tag only after `gh repo view` shows the new name. Check Actions, secrets and OIDC permissions
   survived the rename.
4. Update the repository description and topics with the new name (values from
   [phase 1 of the promote plan](../261014-1700-promote-skillhub-mcp/phase-01-repository-page.md)).

Verify:

- Fresh install with the new `install.sh` and `install.ps1` URLs verifies the signature
  (`SKILLSTATION_REQUIRE_SIGNATURE=1`).
- The old install URL resolves through the redirect to the new release.
- An old binary's self-update fails safe and changes nothing (expected, as in the previous rename).
- `go install github.com/vantt/skillstation/cmd/skillstation@latest` works.

### Phase 4: migrate existing installs by hand

Order matters: remove old connections with the **old** binary before uninstalling it. Write these steps into the
release notes and `docs/troubleshooting.md`.

On every machine:

1. `skillhub connect --remove` for each connected agent (and `-g` if connected globally), so the old `mcpServers`
   entry and its permission rules are removed by the tool that recorded them.
2. If the hub was started with `skillhub serve --install-service`, stop and remove that service by hand: the old
   binary has no command to remove it. systemd: `systemctl --user disable --now` the unit, then delete the unit file;
   launchd: `launchctl bootout` the job, then delete the plist. Read the exact unit and plist names from
   `internal/delivery/cli/serve_service.go` before writing the steps into the release notes.
3. Uninstall the old binary: `curl -fsSL <old install.sh URL> | sh -s -- --uninstall` on Linux and macOS,
   `$env:SKILLHUB_UNINSTALL = '1'` with the old `install.ps1` on Windows (see `docs/installation.md`). This removes
   the binary, its marker and the `PATH` lines; it keeps hubs and agent connection files.
4. Install Skillstation with the new one-liner.
5. Move user folders: `~/.config/skillhub` to `~/.config/skillstation`, `~/.local/state/skillhub` to
   `~/.local/state/skillstation` (Windows and macOS paths per `internal/userconfig`). Rename any `SKILLHUB_*`
   variables in shell profiles and in `$SKILLSTATION_CONFIG_DIR/env` files.

On every hub repository:

6. `git mv .skillhub .skillstation`; grep the hub for `SKILLHUB_` and `skillhub` in skill scripts and update them;
   commit; push if the hub has a remote.
7. `skillstation doctor` must pass; then `skillstation connect` again on each machine; remote machines need a new
   invite from the hub.
8. Check with the agent: a new session calls `skill_resolve` from the `skillstation` server.

### Phase 5: follow-ups

- Video: after the owner commits the current video work, rename names in `video/` (script, tapes, Remotion,
  narration, subtitles, YouTube text) and re-render. Separate change, not part of the code rename.
- Launch documents: update the promote plan and launch strategy to the new name; rename the promote plan folder
  only if the owner wants (plans are records; a note is enough).
- Any listing or post already made under the old name: update when possible.

## Risks and rollback

| Risk | Effect | Mitigation |
|---|---|---|
| Blind replace changes a word that is not the product name | Broken behavior or text | Mapping order, one category per commit, review every hunk, full test run |
| Old and new binaries on different machines | Remote calls fail (service and realm differ) | Clean break is accepted; migrate all machines in phase 4 the same day |
| Old binary self-update | Fails safe, needs one reinstall | Release notes; same as the previous rename |
| Uninstalling before `connect --remove` | Stale `mcpServers.skillhub` entries and permission rules left in agent configs | Phase 4 order; if it happened, remove the entries by hand from the agent config files |
| Skills in hubs use `$SKILLHUB_*` | Skill scripts fail after migration | Phase 4 step 6 grep |
| Rename while a release runs | Signature identity mismatch | Rename only when no workflow runs; tag after the rename |

Rollback: before phase 3, drop the branch. After the repository rename, GitHub allows renaming back; releases signed
under the new name would then fail the old identity, so roll back only before the first new release.

## Acceptance criteria

- [ ] `git grep -i skillhub` outside the excluded record folders shows only the reviewed leftovers (the "formerly
      called" line in both READMEs, release-note history, path fixtures if any).
- [ ] `gofmt`, `go vet` on three platforms, `make check`, web e2e and the installer test scripts pass; CI green on
      Linux, Windows and macOS.
- [ ] The repository is `vantt/skillstation`; `v0.8.0` is signed and published; fresh installs verify on three OSes.
- [ ] An agent connected with `skillstation connect` resolves and loads a skill from a hub with `.skillstation/`,
      locally and through a remote invite.
- [ ] Release notes and `docs/troubleshooting.md` carry the manual migration steps.
- [ ] The owner's machines and hubs run on Skillstation.

## Open questions for the owner

1. Buy `skillstation.dev` to hold the name? Optional; nothing in this plan needs it.

## Progress

- 2026-10-19: plan written; no code changed.
- 2026-10-19: repository renamed to `vantt/skillstation` (owner's go). Phases 1 and 2 done in worktree branch
  `rename-skillstation`; `make check`, `make web-check`, `make web-e2e`, the Linux installer lifecycle test and
  `go vet` for Linux, macOS and Windows pass. Intended leftovers of the old name: the "formerly called" lines in both
  READMEs, the migration section in `docs/troubleshooting.md`, the naming survey in `docs/PRD.md`, the citation of the
  old hub repository in `internal/app/migration_v3_fixture_test.go`, the distill-lab fixtures under
  `internal/systemskills/testdata` and `internal/migration/testdata` (data written by an earlier implementation),
  and generic "skill hub" wording in distill examples. Done beyond the plan: the macOS CI failure in a commit test
  (symlinked temp dirs) fixed; the release smoke test reads the fourth field of the web UI banner; digests of the
  files earlier releases shipped added to the known-digest tables; wordmark redrawn from Fraunces outlines.
- 2026-10-19: phase 3 done. Repository renamed; `v0.8.0` tagged on `7d8ba5a` after CI was green on the three platforms;
  the release workflow published signed archives, SBOMs and installers. Release notes carry the breaking changes and a
  link to the migration steps. Verified: a fresh `install.sh` run with `SKILLSTATION_REQUIRE_SIGNATURE=1` and cosign
  installs 0.8.0; the old install URL redirects to the new release; a v0.7.2 binary's `update` fails safe (identity
  mismatch, binary unchanged); the pinned v0.7.2 installer still uninstalls an old install. The default hub folder
  became `~/skillstation-hub` and the web screenshots were retaken. Left for the owner: phase 4 (migrate own machines
  and hubs) and the repository description and topics, which belong to the promotion plan.

## Status (2026-10-28)

Completed. The repository is `vantt/skillstation`, `v0.8.0` is signed and published, and the code, docs, README and web use the new name. Re-checked on 2026-10-28: `git grep -i skillhub` outside the archives, `plans/` and `video/` leaves only the migration notes in `docs/troubleshooting.md`, the README "formerly" line, historical records under `docs/distillery/` and `docs/plans/`, and the unrelated identifier `SkillHubChange`. The owner's hub uses `.skillstation` and `~/.config/skillstation`. `make web-e2e` passed (43 passed, 2 skipped screenshot cases) and CI is green.

- Repository description and topics set on 2026-10-28 (`mcp`, `agent-skills`, `claude-code`, `skills`, `golang`, `ai-agents`).
- Handed to `261016-1000-intro-video`: the about 133 `skillhub` lines under `video/`, which need the demos recorded again under the new name.
- Still with the owner by choice: listings and posts, which belong to `261014-1700-promote-skillhub-mcp`.
