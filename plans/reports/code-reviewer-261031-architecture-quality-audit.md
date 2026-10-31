# Skillstation architecture and quality audit (2026-10-31, commit 3e6766d)

## Tóm tắt (Tiếng Việt)

- Lõi lưu trữ (WAL mutation, recovery theo digest, catalog SQLite bất biến, khoá workspace) được làm cẩn thận và có test fault-injection, fuzz, race, perf. Mọi ghi file canonical bằng Go đều đi qua `mutation.PlanMutation` (20 call site, đã kiểm). Không tìm thấy lỗi P0.
- Lỗi P1 quan trọng nhất ở tầng thấp: `commitWhileLocked` nuốt lỗi khi đọc `.skillstation/transactions` (mutation.go:298), nên một commit có thể chạy chồng lên giao dịch đang dở. Sửa một dòng.
- Có "cửa phụ" thật: script `skill-audit` (commit mới nhất) ghi thẳng `.meta/hub.yaml`: không khoá, không atomic, không chặn symlink, làm file tracked bị dirty (khiến lần sửa skill kế tiếp bị từ chối với `UncommittedEditsError`) và làm đổi catalog snapshot. Route web `/skills/{id}/distill` đọc file trực tiếp, bỏ qua tầng app, không validate `id`.
- Giao dịch recovery bị "conflict" không có lối ra: `mutation.RollBack` tồn tại nhưng không có lệnh nào gọi tới; doctor chỉ báo "blocked".
- Hợp đồng lệch: `docs/contracts/web-api.md` liệt kê 7 route không còn, thiếu 1 route; `docs/design/01` nói WebUI không được duyệt nội dung nhưng web có route duyệt; mã lỗi "not found" khác nhau ở web/MCP/CLI, CLI còn so khớp chuỗi lỗi.
- Hiệu năng: xem chi tiết một skill chạy `canonical.Validate` toàn hub (đọc mọi file tới 64 MiB và gọi git) mỗi lần; trang Hub calls poll 10 giây, mỗi lần ghi/prune DB telemetry và nạp toàn bộ event ~4 ngày vào RAM.
- Nợ: 109 hàm chết không tới được từ binary, 44 hàm vượt giới hạn lint, grammar skill id lặp 8 nơi, 13 hàm ghi atomic tự viết.
- Thứ tự sửa đề xuất: mutation → canonical/skillmeta → skill-audit/side doors → app (assess, error) → delivery → docs → release.

## Scope and method

- Read: `AGENTS.md`, `docs/design/01`, `docs/design/07` (sections 3, 8 to 12, 21, 22), `docs/contracts/*`, `schemas/*`, code under `internal/` and `cmd/`, the last 15 commits (focus on 3e6766d skill-audit, bfc830b/a8f1f8b hub-call log, a01444e/3e13885 zip upload).
- Ran (read-only, temp GOCACHE): `go build ./...` OK, `go vet ./...` OK, `gofmt -l` clean, `go test -count=1 ./...` all 33 packages pass, `SKILLSTATION_PERF=1 go test -p 1 -run Performance ./...` all pass, `deadcode ./cmd/...` (x/tools v0.30.0), a go/ast function-size scan, `go list` import graph. No `make check`, no lint run (golangci-lint needs a download and `origin/main` diff; CI covers it).
- Size: about 100k lines of production Go (`internal/app` 36k, `internal/delivery` 22k) and about 107k lines of Go tests; web UI about 20k lines TS.
- Not done: web `npm` checks and Playwright (CI runs them), Windows/macOS runtime checks (reasoned from code only), golangci-lint.

Legend: **Verified** = seen in code or command output. **Suspicion** = reasoned, not reproduced.

## Findings by layer (lowest first)

### Layer 1: storage, canonical Git repo, SQLite projection, skillmeta, canonical validation

**F1 (P1, verified). Commit ignores an unreadable or invalid transaction directory.**
- Evidence: `internal/mutation/mutation.go:298-301`:
  ```go
  if ids, err := pendingUnlocked(root); err != nil {
  } else if len(ids) != 0 {
      return Receipt{}, false, ErrRecoveryRequired
  }
  ```
  `PlanMutation` (line 164) returns the same error. `pendingUnlocked` fails on EACCES/EIO and on any entry that is a symlink, a file or a non-opaque name (`recovery.go:85-101`).
- Why it matters: the documented invariant "Refuse normal mutation if another transaction is pending" (07 §10.2 Phase B.2) does not hold on the exclusive-lock path. Between Plan (shared lock) and Confirm (exclusive lock) the directory can become unreadable or gain an invalid entry, and Confirm then writes over a possibly pending transaction. Recovery would then face two overlapping journals.
- Fix: `return Receipt{}, false, err` in the empty branch. Add one table case to the existing mutation hardening tests (a non-opaque entry in `transactions/` makes `CommitWithOptions` fail). The test fails on today's code.
- Effort: S.

**F2 (P1, verified). An interrupted mutation classified as conflicted has no supported exit.**
- Evidence: `mutation.RollBack` and `preflightRollback` (`recovery.go:217`, `:342`) are unreachable from either binary (deadcode). `app/workspace.go:98-106` marks `RecoveryConflict` as not fixable and offers no action. 07 §10.1 promises `RecoveryRequired --> Aborted: explicit safe rollback`, and §12.1 says "Prepared, all paths before → default abort". `docs/troubleshooting.md` has no recovery section.
- Why it matters: every normal command returns `recovery_required` until the journal is gone. The only exit is to delete `.skillstation/transactions/<op>` by hand, which nothing tells the user to do. It is also unsafe to guess.
- Fix: expose the existing rollback as `doctor --fix --rollback <operation-id>` (preview plus confirm, the same pattern as roll-forward). Otherwise print the exact manual steps in the finding and in troubleshooting.md, and delete the dead code.
- Effort: M.

**F3 (P2, verified). The metadata key sets have three authorities and no sync test.**
- Evidence: `schemas/skill-hub.schema.json`, `skill-settings.schema.json` and `skill-metadata.schema.json` (properties). `internal/skillmeta/skillmeta.go:56-63` (`SettingsKeys`, `HubKeys`, `settingsKeys` map). `internal/canonical/skill.go:169` (`allowed` list). Commit 3e6766d had to touch all of them to add `audit`. `schemas/embed_test.go:536` checks example documents only, not the key sets.
- Fix: one test that compares `skillmeta.HubKeys()`/`SettingsKeys()` with the schema `properties`. Derive `canonical/skill.go:169` from the skillmeta lists plus the derived keys.
- Effort: S.

**F4 (P2, verified). The skill id grammar is defined 8 times.**
- Evidence: `^[a-z0-9][a-z0-9-]{0,62}$` appears in `skill/lifecycle.go:77`, `app/skill_add.go:25`, `app/skill_snapshot.go:67`, `canonical/skill.go:23`, `skillruntime/cache.go:21`, `skillruntime/remote_cache.go:181`, `skillruntime/statedir.go:41` and `delivery/mcpserver/proxy.go:67`. The web distill route does not validate at all (see F8).
- Fix: export one `skillmeta.ValidSkillID` (skillmeta has no imports, so every layer can use it) and use it at each boundary.
- Effort: S.

**F5 (P2, verified). Low-level utilities are duplicated.**
- Evidence: `atomicReplace` is copied in `internal/mutation/atomic_replace_*.go` and `internal/catalog/atomic_replace_*.go`. 11 more hand-rolled temp-plus-rename writers exist (`invite/filestore.go`, `hubtoken/hubtoken.go`, `userconfig`, `workspace/workspace.go:434`, `app/health_hub.go:720`, `app/hub_calls.go:133`, `app/draft_registry.go:100`, `app/skill_env.go:255`, `telemetry/export.go:130`, `hostintegration/permission-record.go`, `skillruntime/cache.go`). Their fsync and permission policies differ. Some runtime writes are plain `os.WriteFile` (`delivery/cli/telemetry_baseline.go:131`, `delivery/cli/serve_service.go:216`). `distill.SaveDocument` (`distill_model.go:311`) is commented "atomically" but uses `os.WriteFile`; it is dead code.
- Fix: one small `internal/fsx.WriteFileAtomic(path, data, perm)` (temp in the same dir, fsync, rename, dir fsync on Unix). Delete `SaveDocument`.
- Effort: M (mechanical).

**F6 (P2, verified). Catalog assessment swallows errors.**
- Evidence: `catalog/servable.go:294-301` ignores `rErr` and `scanErr`, so a failed query yields "0 resources" and `ResourcesMatch` stays true. `servable.go:234` ignores the YAML unmarshal error of the status. `app/skill_detail.go:147-150` and the 5 `skill_add.go` call sites drop the assessment error (`assessment, _ :=`).
- Why it matters: the detail and review screens can say "matches" when the generation could not be read.
- Fix: return the error, or set an explicit `Served.Indexed=false` with a reason.
- Effort: S.

**F7 (P2, verified). Missing index for a frequent lookup.**
- Evidence: `catalog/schema.go:41-47`: `resources` has `PRIMARY KEY(path)` only. Every `WHERE skill_id=?` query (`skill/resource.go:68`, `app/generation_memo.go:83`, `catalog/servable.go:294`) scans the table. The same SQL string is also duplicated in those three places.
- Fix: `CREATE INDEX resources_skill ON resources(skill_id, path)` (a derived schema bump triggers a rebuild, no canonical migration). Keep one query helper in catalog.
- Effort: S.

### Layer 2: side doors into the hub (single-door question)

What holds (verified): every Go write to canonical paths goes through `mutation.PlanMutation` then `Confirm*`. The 20 call sites are in `app/*`, `skill/lifecycle.go` and `migration/migration.go`. Direct `os.*` writes in `app` touch project checkout folders, `runtime/`, `.git/` or init skeleton files only, all outside the canonical WAL by design. Classified write sites: `skill_checkout`, `skill_commit_apply.writeFolderTree`, `skill_scaffold`, `project_skill_folder`, `draft_service` (project folders); `hub_calls`, `submissions`, `skill_zip`, `upstream_merge`, `skill_snapshot`, `skill_env`, `draft_registry` (runtime); `health_hub` (Git hooks and scratch indexes); `workspace_init`, `workspace/workspace.go` (init).

Bypasses found:

**F8 (P1, verified). `skill-audit` writes `.meta/hub.yaml` behind the hub's back.**
- Evidence: `.agents/skills/skill-audit/scripts/audit.mjs:291-298`:
  - It reads the file, strips the `audit:` block with a regex, appends a new block and calls `fs.writeFileSync(file, text)`.
  - There is no workspace lock, no temp-plus-rename and no symlink check. `distill.mjs:1896-1943` does all three.
- Why it matters:
  1. **Lost update.** `hub.yaml` holds `status`, `quality.content_reviewed_digest` and `sources`, and every mutation re-stamps `updated_at` in it (`mutation/skill_metadata.go:121`). If an approval or transition is confirmed between the script's read and write, the script's write reverts it. The confirm itself is protected by digests; the script is not.
  2. **Truncation.** A crash mid-write truncates `hub.yaml`, the canonical workspace becomes invalid and serving falls back.
  3. **Dirty tracked file.** `hub.yaml` is tracked. After `--record`, the next CLI edit of that skill hits `refuseUncommittedEdits` (`mutation.go:357-377`), because the mutation stamps `hub.yaml`, and fails with `UncommittedEditsError`. The skill-audit `SKILL.md` (lines 75-85) does not tell the agent to commit.
  4. **Snapshot churn.** `hub.yaml` is catalog-affecting (`canonical/canonical.go:492-497` excludes only `distill.yaml`), so each record changes the catalog snapshot and invalidates pinned resolutions.
  - (3) and (4) are verified by reading the code; (1) is a reasoned race.
- Fix (smallest first):
  - (a) Write atomically with the same `assertNoSymlinks` and `writeAtomic` as distill.mjs. Add a test that the binary's `canonical.Validate` accepts the recorded file.
  - (b) Choose one: keep `audit:` out of the catalog snapshot like `distill.yaml` (it is not routing data), or record the audit through a binary command (`skillstation skill audit-record`) that goes through `PlanMutation` with a commit.
  - (c) Say in `SKILL.md` that the record leaves `hub.yaml` modified and how to commit it.
  - (b) is a design decision for the owner.
- Effort: S for (a) and (c), M for (b).

**F9 (P1, verified). The web distill route bypasses app and does not validate `id`.**
- Evidence: `internal/delivery/web/routes_distill.go:21-49`:
  - `filepath.Glob(root/skills/*/<id>/.meta/distill.yaml)` takes `id` from `r.PathValue` without validation, so glob metacharacters and `..` are accepted.
  - The fallback hard-codes collection `default`.
  - It calls `distill.LoadDocument` directly.
  - `app/skill_sources.go:157-162` has a second copy of the lookup, and `app/curation_home.go:392` a third.
  - A missing file is reported as `invalid_request` with HTTP 404.
  - The route is not in `docs/contracts/web-api.md`.
- Threat model: only a holder of the session token can call it, and only files named `.meta/distill.yaml` are read, so the impact is low. This is a boundary violation, not a data leak.
- Fix: add `SkillService.GetSkillDistill(ctx, root, id)` in app. Validate the id, resolve the folder from the catalog or skillmeta (not by glob), and reuse it from `skill_sources.go` and `curation_home.go`. Document the route.
- Effort: S.

**F10 (P2, verified). Other direct calls from delivery into domain packages.**
- Evidence: `delivery/cli/skill_editor.go` calls `skill.CleanupExpiredRecoveries` (runtime only). Commit policy is decided in three adapters (`cli/confirm_prompt.go`, `mcpserver/tool_registration.go`, `web/security.go`) through a context value read deep in `mutation.CommitWithOptions` (`mutation.go:252`), a hidden cross-layer channel.
- Fix: keep it, but document in 07 §21 that the commit policy is an adapter decision carried by context, and route the cleanup through app. Low priority.
- Effort: S.

### Layer 3: domain and app services

**F11 (P1, verified, performance). One skill's detail runs a full hub validation and spawns git.**
- Evidence: `catalog.AssessSkillStateWhileLocked` (`servable.go:254`) calls `canonical.Validate(root)`.
  - That reads every canonical file, up to `MaxCanonicalBytesV1` = 64 MiB (`workspace/workspace.go:25`), and parses every YAML.
  - It also runs `workspace.UnmergedGitPaths`, which spawns git (`canonical.go:96`), only to filter issues by the skill's prefix.
  - Callers: `GetSkillDetail` (`app/skill_detail.go:146`), `ReviewSkill` (`skill_review.go:548`), `catalog.go:104`, and 5 sites in `skill_add.go` (814, 911, 1008, 1042, 1128).
  - No perf budget covers this path; the budgets cover resolve, catalog open/rebuild and MCP memory only.
- Why it matters: cost grows with hub size per page view. The web detail page and the add flow call it repeatedly, under a shared lock that delays writers.
- Fix: validate only the skill's folder plus its cross-references (canonical already has per-skill validation, `skill.go:298 checkSkillValues`), or memo `Validate` by `ScanCached` snapshot digest. Add a `GetSkillDetail` budget at 256 skills.
- Effort: M.

**F12 (P1, verified). Error classification leaks into delivery and "not found" has no stable code.**
- Evidence:
  - `app/error_classify.go:162-170` maps `skill.ErrNotFound` to `invalid_request` with the text "The request conflicts with the current object state".
  - Web then forces HTTP 404 with that body (`web/errors.go:54-61`).
  - MCP uses `not_found` (`skill_tools.go:222`, `skill_review_tools.go:29`), `skill_not_found` (`rpc_errors.go:25`, `distribution.go:166`) and `not_servable`. None of these are in `docs/contracts/error-codes.md` or the `error-envelope.schema.json` enum.
  - CLI derives fixes by matching message text: `cli/skill.go:1024`, `:1042` (`why == "skill not found"`), `:1043-1050` (`strings.Contains(message, "routing.not_for")`). MCP does the same at `insight_tools.go:41` (`strings.Contains(err.Error(), "does not exist; use skill_create_preview")`).
  - Doc 01 §5 says delivery adapters "không giữ matching rules riêng" (keep no matching rules of their own).
- Why it matters: clients cannot branch on a stable not-found code, and any reworded message silently breaks the CLI's FIX lines.
- Fix: add `not_found` to the registry, schema and `app.ClassifyError`. Move the CLI fix derivation into `app` (typed validation errors carrying a fix). Map MCP's three variants to it, keeping the old codes on the remote bundle protocol if a shipped client depends on `skill_not_found` (`proxy.go:894` does).
- Effort: M.

**F13 (P2, verified). Two service layers for skills.**
- Evidence: `internal/skill` (Manager, lifecycle with `mutation.PlanMutation`/`ConfirmMutationWithOptions` at `lifecycle.go:473,535`, proposal store, resource edit) sits beside `internal/app/skill_*`. Delivery imports `skill` for types and errors in about 17 files. `skill/resource_edit.go` (354 lines) and `skill/proposal_store.go` (dispatcher) are mostly unreachable (deadcode).
- Fix: no rewrite. Delete the dead parts (F20) and treat `skill` as the domain package behind app; stop adding new entry points there.
- Effort: S to M.

**F14 (P2, verified). The skill-zip stage does not reuse the runtime/tmp safety of upstream merge.**
- Evidence: `app/upstream_merge.go:28-75` sweeps `upstream-merge-*` older than an hour and refuses a symlinked `runtime/tmp`. `app/skill_zip.go:63-82` does neither, so a crash leaves `runtime/tmp/skill-zip-*` behind for good. Extraction itself is solid: `os.OpenRoot`, O_EXCL, size and count limits, rejection of case collisions.
- Fix: one `newRuntimeTemp(root, prefix)` helper used by both, which checks for a symlink and sweeps stale entries of any prefix.
- Effort: S.

**F15 (P2, suspicion). The checkout sync writes through symlinks in the project folder.**
- Evidence: `app/skill_commit_apply.go:316-348` (`writeFolderTree`) uses `os.WriteFile`/`os.Remove` on `folder/rel`, which follow a symlinked subfolder such as `scripts`. The project folder is user-owned, but the content comes from the hub.
- Fix: use `os.OpenRoot(folder)` as `skill_zip.go` does.
- Effort: S.

### Layer 4: delivery surfaces (cli, mcpserver, web)

**F16 (P1, verified). The web API contract has drifted.**
- Evidence: comparing `docs/contracts/web-api.md` §3 with the registered routes (`grep mux.HandleFunc`):
  - Documented but not registered: `GET /api/v1/runs/{id}`, `POST /runs/{id}/cancel`, `GET /inbox`, `GET /insights/{id}`, `POST /insights/{id}/decision`, `POST /insights/{id}/apply/preview`, `POST /insights/apply/confirm`.
  - Registered but not documented: `GET /api/v1/skills/{id}/distill`.
  - §2.1 names MCP tools `curation_run_start`, `curation_run_retry` and `curation_run_submit`, which 07 §25 lists as removed (`05-curation-lifecycle.md` still cites `inbox_list`, `insight_get`, `insight_decide`).
  - §4 lists the 500 code as `internal`; the code is `internal_error` (`app/errors.go:17`).
  - 413 (`security.go:75`) and 429 (`security.go:98`) are not in the status table.
  - The `track/preview` row says it fills `.meta/skill.yaml`; from layout 6 that data is in `.meta/hub.yaml`.
  - Doc 01 §11 says content approval is CLI only ("Tuyệt đối không hỗ trợ duyệt qua MCP tools hoặc WebUI", never through MCP tools or the WebUI), but `POST /api/v1/skills/{id}/approve/preview` and `/submissions/{sid}/approve/preview` exist and web-api.md §2.2 accepts that.
- Fix: update web-api.md and doc 01 §11 to the shipped behavior. Add a test that the registered route set equals the documented table (the routes are already collectable through `routeRegistrars`).
- Effort: S.

**F17 (P2, verified). Duplicated auth throttles and placeholder code in web.**
- Evidence: `web/throttle.go` (20 per 60 s, 1024 keys) and `mcpserver/http.go:470-523` (`authFailures`) implement the same thing twice. `web/server.go:83-84` has `_ = srv.skillAdd; _ = srv.sources`.
- Fix: share one throttle in `delivery/hostpolicy`, which both already import; drop the blank assignments.
- Effort: S.

**F18 (P2, verified, perf and memory). The Hub calls page does maintenance work on every poll.**
- Evidence: web polls every 10 s (`web/src/api/queries.ts:467`). Each poll goes through `HubCallService.Log` → `Recorder.RawEvents` → `rawEventsStore` (`telemetry/raw_events.go:13`):
  - It runs `maintainStore` first, which opens the DB, deletes old rows, sums the logical size and may `VACUUM`.
  - It then opens the DB again and loads every event of every kind from the start of the day 3 days ago into memory (`WHERE (? = '' OR occurred_at >= ?)`; the OR form likely defeats the index).
  - It keeps all hub calls before cutting to 200 (`hub_calls.go:292,320`).
  - Telemetry is capped at 100 MiB (`recorder.go:20`), so this is bounded but large on a busy hub, and the writes contend with MCP processes.
  - `RecordHubCall` reads `runtime/config/hub-call-log.json` and builds a redactor on every `skill_resolve` (`hub_calls.go:92-96`).
- Fix:
  - Query `kind IN ('hub.call', resolution kinds)` with plain bounds.
  - Aggregate counts in SQL and `LIMIT 200` for recent calls.
  - Run maintenance from the recorder's own schedule, not from reads.
  - Cache the settings by file mtime.
- Effort: M.

**F19 (question, user decision, not a defect). Web serve listens on every interface over plain HTTP.**
- Evidence: `cli/serve.go:126` defaults to `0.0.0.0` (deliberate, commit 8227140 `feat(cli)!`). The bearer token travels in clear on the LAN, and the web surface can approve third-party content (F16). web-api.md §2.2 accepts the risk of browser automation by a token holder; it does not mention LAN sniffing.
- Options: (a) keep it and document the sniffing risk; (b) keep `0.0.0.0` but refuse the approve routes from non-loopback peers; (c) default to loopback and make LAN opt-in. Needs the owner's call; do not change silently.

### Layer 5: web UI

- Verified: about 20k lines TS, strict tooling (tsc, eslint, vitest, Playwright plus axe in CI). One suppression-like hit in `src/` outside tests, and it is a comment. API types are hand-written (`src/api/types.ts`, 1096 lines) with no generation from Go, so drift is caught only by e2e. P2: a golden-JSON contract test against `internal/delivery/web/testdata/golden/` already exists on the Go side; adding a TS type check over the same goldens would close the loop (Effort M).
- Embedded assets: 5.2 MB, 345 files, most of them webfonts for 15 font families (`web/package.json`) behind the theme switcher. Binary (stripped, CGO off) is 30.0 MB. P2: drop unused themes or the latin-ext subsets if size matters (Effort S).

### Layer 6: release and packaging

Verified good:
- `release.yml` validates the tag and version.
- It reruns tests, race, perf, vet, fault-injection, fuzz, installer lifecycle on 3 OS and the tag-derived digest tests (`SKILLSTATION_REQUIRE_TAGS=1`).
- It builds the web dist fresh from source (it is not committed; `.gitignore` keeps only `.gitkeep`).
- Builds use `CGO_ENABLED=0 -trimpath -buildvcs=false` with injected version, deterministic archives, Syft SBOM, sha256 checksums and keyless cosign signing plus attestation.
- Commit 3e6766d bumped `CuratorSkillVersion` and added the v0.10.0 curator digest to `known-digests.go`. Checked: `git show v0.10.0:internal/systemskills/curator/SKILL.md | sha256sum` = `6dd9a563...`, which matches the added line. skill-audit has not shipped yet, so it needs no entry.

**F20 (P2, verified). 109 functions are unreachable from both binaries.**
- Evidence: `deadcode ./cmd/...`. Among them:
  - `mutation.Commit`, `ConfirmMutation`, `RollForward`, `RollBack`, `Pending`, `TryInspectRecovery`
  - all of `skill/resource_edit.go`, `skill.ProposalDispatcher`, `skill.GetManifest`
  - `distill.SaveDocument`, `LoadRevisionPackage`, `ReadEvidence`, `Marshal`, `InsightEvidenceDigest`
  - `evaluation.ComparePaired` and its helpers
  - `catalog.EnsureFreshOrRebuild`
  - `app.SkillService.PreviewSkillAdd`, `ConfirmSkillAdd`, `ListSkillResources`, `ReadSkillResource`, `PreviewSkillResourceUpdate`
  - `schemas.TelemetryEvent`, `EvaluationCase`, `EvaluationSuite`, `ExperimentManifest`
  - `source.ParseGitHubLocator*`, `ResolveGitHubRoute`, `ResolveGitHubLocator`
  - Several of them are exported seams kept alive only by tests, which AGENTS.md "Testing" forbids.
- Fix: delete them in one cleanup commit per package, keeping F2's rollback if you choose to expose it. Re-run `deadcode` in CI as an informational step.
- Effort: M.

**F21 (P2, verified). Oversized functions and files.**
- Evidence: 44 of 3180 production functions break the lint limits (more than 120 lines or more than 6 arguments). The worst:
  - `cli/skill.go:387 parseSkillFlags` (357 lines) and `:57 runSkill` (325)
  - `distill/workspace_validation.go:24 ValidateWorkspace` (290)
  - `resolver/resolver.go:97 Resolve` (270)
  - `cli/source.go:26 runSource` (260)
  - `app/source_import.go:87 PreviewSourceImport` (245)
  - `mutation/transaction.go:38 commitPrepared` (191, 6 args)
  - `skill/lifecycle.go:461 plan` (10 args)

  Largest files: `app/source.go` 1402, `skill/lifecycle.go` 1298, `app/skill_add.go` 1269. The lint gate blocks only new issues, so this debt does not shrink.
- Fix: split the CLI flag parsers into per-subcommand tables first (pure delivery, lowest risk); leave `commitPrepared` alone unless touched (it is the most safety-critical code and well covered).
- Effort: L in total; do it opportunistically.

**F22 (P2, verified). Release preflight does not prove the tag commit is on `origin/main`.**
- Evidence: `scripts/release-preflight.sh` checks a clean tree, local branch name `main`, tag availability and the latest CI run for the HEAD sha. CI also runs on `pull_request`, so a local `main` whose HEAD was pushed only on a PR branch passes.
- Fix: `git fetch origin main && [ "$(git rev-parse HEAD)" = "$(git rev-parse origin/main)" ]`.
- Effort: S.

**F23 (P2, verified). Panics are swallowed in telemetry paths.**
- Evidence: `defer func() { _ = recover() }()` in `app/curation_telemetry.go:22,118`, `app/resolver.go:507`, `cli/telemetry_lifecycle.go:56` and `mcpserver/session_telemetry.go:86`. Keeping telemetry from breaking a business result is right, but a silent recover hides real bugs.
- Fix: log the panic at debug level with a counter.
- Effort: S.

### Concurrency and platform notes (no defect proven)

- Git commit runs after the exclusive lock is released (`mutation.go:262-283`), serialized only by the commit lock. A mutation without a commit policy that runs in that window on the same path would be folded into this commit. `refuseUncommittedEdits` protects mutations that do have a policy. Suspicion; low.
- Windows: `MoveFileEx(REPLACE_EXISTING|WRITE_THROUGH)` is used, directory fsync is skipped on Windows (`filesystem.go:421`), and snapshot replacement handles races (`skill_snapshot.go:565-588`). Renaming a snapshot directory still open by an agent can fail on Windows; the code returns an error instead of corrupting anything. Acceptable.
- The race suite runs in CI on 3 OS; no goroutine leak was found by reading. The MCP memory budget holds: 30.8 MB at start, 43.7 MB after 1001 resolves, 38.8 MB after 60 s idle; resolve p95 12 ms.

## Performance numbers measured today (`SKILLSTATION_PERF=1`)

| Path | Result |
|---|---|
| Resolver uncached, 256 skills | p50 20.5 ms, p95 28.1 ms (budget 300 ms) |
| Self-resolution, 256 synthetic skills | 3.5 ms per resolve |
| Catalog warm open, 256 skills | p95 23.1 ms |
| Catalog rebuild, 256 skills | p95 317 ms |
| MCP process RSS | 30.8 MB start, 43.7 MB after 1001 resolves, 38.8 MB idle |
| Not measured, no budget | `GetSkillDetail`/`ReviewSkill` (F11), `/api/v1/hub-calls` (F18) |

## What is genuinely good (do not break in cleanup)

- The mutation protocol: digest preconditions, staged after-images, the receipt written last, digest-based recovery classification, named fault points (`mutation.go:116-133`) with tests for each, bounded fuzzing of path confinement.
- Immutable catalog generations with an atomic pointer, reader pins and age-gated garbage collection (`catalog/catalog.go:260-320`). `ScanCached` keeps resolve cheap.
- One door for canonical writes in Go (20 `PlanMutation` call sites, no direct writes to canonical paths).
- The error code registry, `app/errors.go` and the schema enum agree exactly (diffed). The resolver MCP schema is the committed JSON schema, embedded and served, so it has one authority.
- Zip upload hardening (`app/skill_zip.go`): `os.OpenRoot`, O_EXCL, real byte counting, rejection of symlink, encrypted and case-colliding entries.
- The remote MCP HTTP gate (`mcpserver/http.go:270-340`): Host and Origin checks, constant-time token, failure throttle that does not lock out valid tokens behind a tunnel, rate limiter, session cap, upload deadline.
- Release pipeline: reproducible flags, SBOM, cosign, installer lifecycle on 3 OS, tag-derived known-digest tests, correctly applied in the latest commit.
- Test suite: 33 packages green, about 107k lines of tests, perf and memory budgets, mirror-equality tests for the bundled skills.

## Recommended fix order (lowest layer first)

1. F1: return the error in `commitWhileLocked` plus a regression test (S).
2. F2: expose rollback for conflicted recoveries, or document the manual exit (M).
3. F3 and F4: key-set sync test; one skill id validator in `skillmeta` (S).
4. F7: `resources(skill_id)` index and one query helper (S).
5. F8 (a) and (c): make the skill-audit write atomic and symlink-safe, and tell the agent the file is left modified (S). Decide (b) with the owner.
6. F9: move the distill read into app with id validation (S).
7. F6 and F11: stop swallowing assessment errors; validate per skill instead of the whole hub; add a detail budget (M).
8. F12: a `not_found` code plus moving CLI fix-matching into app (M).
9. F14, F15, F5: runtime temp helper, OpenRoot for the checkout sync, one atomic-write helper (S to M).
10. F16: bring web-api.md and doc 01 §11 up to date; route-table test (S).
11. F18, F17: hub-call query and maintenance split; shared throttle (M, S).
12. F20, F21, F23: dead-code removal, then CLI parser splits (M to L).
13. F22: preflight `origin/main` check (S). F19 waits for the owner's decision.

## Plan follow-ups

No plan file was given. All 8 questions are answered above. Nothing in `plans/` was changed.

## Unresolved questions

1. F8 (b): should the skill-audit record stay in tracked `.meta/hub.yaml` (catalog-affecting, dirties Git) or move out of the catalog snapshot, or be written through a binary command?
2. F19: is the plain-HTTP all-interfaces default intended to also expose content approval to the LAN?
3. F12: do shipped remote clients depend on the `skill_not_found` code on `/mcp/bundle`? If so, keep it there and add `not_found` elsewhere.
4. F2: should a conflicted recovery offer rollback, or is a manual exit (documented) the intended product behavior?
