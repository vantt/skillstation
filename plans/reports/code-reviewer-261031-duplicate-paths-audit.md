# Duplicate paths and bypassed capabilities: audit

Date: 2026-10-31 (Asia/Saigon). Worktree `skillstation-fix`, branch `fix/audit-findings`, HEAD `75c8b2b` plus the uncommitted edits present during the review. Other agents were editing files at the same time, so line numbers can move by a few lines. Read-only review: nothing was changed except this report.

## Tóm tắt (Vietnamese summary)

Có nhiều khái niệm được cài đặt hai đến bảy lần. Nghiêm trọng nhất:

1. **P0.** `distill-lab/SKILL.md` bảo agent tự sửa `sources` trong `.meta/skill.yaml`. Từ layout 6 file này không còn dùng nữa (sources nằm trong `.meta/hub.yaml`), và validator coi nó là lỗi "metadata vẫn ở layout trước 6", nên hub bị đánh dấu invalid. Lệnh đúng đã có sẵn: `skillstation source attach`.
2. **P1.** Snapshot catalog (`catalogAffecting` và `aggregate`) được tính ở cả `canonical` lẫn `catalog`. Nếu hai bản lệch nhau thì mọi lần rebuild đều thất bại.
3. **P1.** Digest manifest phân phối được tính ở hai nơi: server dùng bản riêng, client dùng `skillruntime.ManifestDigest`.
4. **P1.** Hai bộ cache skill (snapshot cục bộ và remote cache) có cùng thuật toán staging/rename/trash. Bản sửa race mới nhất (4cc652d) chỉ được áp vào một bản.
5. **P1.** Audit record vẫn để JS tự tính content digest và tự parse `hub.yaml`. Go ghi lại digest do client gửi lên mà không kiểm chứng được.
6. **P1.** Có hai mô hình lưu source cùng tồn tại: `sources/catalog` + `sources/intake` so với "sources trong metadata skill". Migration v4 xoá `sources/catalog`, trong khi code hiện tại vẫn ghi vào đó.
7. **P1.** `SKILLSTATION_FIXTURE_VERIFIER` thay thế việc xác minh Sigstore cho mọi URL trong binary, còn installer chỉ cho phép ở chế độ fixture cục bộ.

Các nhóm còn lại (P2): đọc file canonical có giới hạn (3 bản), schema-version (7+ chỗ), frontmatter (8 parser), digest helper (6) và validator (9, một bản chấp nhận hex không có tiền tố `sha256:`), path-safety (7), token (3), atomic write, error message ở web, type TS chép tay, và tài liệu thiết kế mô tả layout cũ.

Đề xuất:
- Hợp nhất từ tầng thấp nhất lên: `fsutil`/`workspace`, rồi `skillmeta`/`canonical`, rồi `skillruntime`, rồi `app`, cuối cùng là script và tài liệu.
- Thêm luật vào `archtest`, một registry "single authority" trong AGENTS.md, và test so khớp schema với Go.

---

## Scope and method

- Read AGENTS.md, docs/design/01 §7, docs/design/07 §3–4, §12.3, §20–25, and docs/contracts/web-api.md.
- Searched for every concept named in the brief across `internal/`, `cmd/`, `.agents/skills/*/scripts`, `system-skills/`, `scripts/` and `web/src`.
- Read each candidate at both locations before listing it.
- "Verified" means I read both implementations and confirmed the divergence or identity in the code. Where a failure is inferred and not reproduced, the text says so.
- No tests were run, because the tree was mid-edit by other agents.

Already being consolidated by other agents in this branch (seen in `git status`, not re-reported): `internal/delivery/web/throttle.go` was deleted in favour of `internal/delivery/auththrottle`; `internal/archtest` was added; audit writes now go through `skillstation skill audit-record` → `app.SkillAuditService` → `mutation`.

---

## Findings

### P0

**D1. distill-lab tells the agent to hand-edit metadata in a file layout 6 rejects; the command that does this already exists**
- Re-creation: `.agents/skills/distill-lab/SKILL.md:291-318` ("Data (the only files you write)"). It tells the agent to edit the `sources` key of `skills/<c>/<id>/.meta/skill.yaml` by hand. It also gives its own hub-root rule: `$SKILLSTATION_WORKSPACE`, else cwd with a marker, else ask.
- Existing capability:
  - `skillstation source attach|detach <source-id> --skill-id <id>` (`docs/command-reference.md:74`, `internal/app/source_links.go:128` `PreviewAttach`).
  - From layout 6, sources live in `.meta/hub.yaml` (`internal/skillmeta/skillmeta.go:6`, `:68` lists `sources` in the hub keys).
  - docs/design/07:463 says `internal/skillmeta` "is the only code that knows these files".
- Verified failure:
  - On a layout-6 hub, `skillmeta.Locate` finds `hub.yaml` first (`skillmeta.go:154-185`) and ignores `.meta/skill.yaml`.
  - `canonical.validateSkills` reports `skills/<c>/<id>/.meta/skill.yaml` as "skill metadata is still in the layout before 6" (`internal/canonical/skill.go:63-65`).
  - So the agent's edit is not applied, and the canonical tree becomes invalid. Catalog build is then blocked (`catalog/input.go:73-79`).
- Also contradicts `system-skills/curator/SKILL.md:275` and `:371` ("never edit `.meta/`").
- Authority: the source service through the CLI. The skill should run `skillstation source attach … --skill-id …` (and `source watch` for upstream).
- Fix: rewrite that section of distill-lab SKILL.md to use the CLI and drop the YAML example. Resolve the hub through the binary instead of a third discovery rule. Mirror the change into `internal/systemskills/distill-lab` and add the shipped digest per the release rule.
- Effort S. Priority **P0**.

### P1

**D2. Catalog snapshot identity is computed in two packages that must agree byte-for-byte**
- `internal/canonical/canonical.go:549` `catalogAffecting` and `:556` `aggregate` (`"%s\x00%s\n"`).
- `internal/catalog/input.go:430` `catalogAffecting` and `:437` `aggregate`. They are identical copies.
- `catalog.captureInputWhileLocked` compares its own result with `canonical.Scan` (`input.go:84-90`) and fails with `ErrCanonicalChanged` on any difference.
- Drift risk: if someone adds a path class to one `catalogAffecting` and not the other, every rebuild and every mutation's PostCanonical publish fails.
- A third copy of the aggregate formula is in `.agents/skills/skill-audit/scripts/audit.mjs:296` (`${rel}\0${digest}\n`).
- Authority: `canonical` (the catalog already imports it).
- Fix: export `canonical.CatalogAffecting` and `canonical.Aggregate`, and make catalog call them.
- Effort S. **P1**.

**D3. The distributed manifest digest has a private copy on the server side**
- `internal/app/distribution.go:426` `hashDistributedManifest` (used at `:396` and at `internal/app/skill_snapshot.go:967`).
- `internal/skillruntime/remote_cache.go:114` `ManifestDigest`. This is the same algorithm: a JSON list of `{path,digest,size}` sorted by path.
- `app` already uses the skillruntime version elsewhere (`internal/app/skill_bundle.go:113`, `internal/app/submission_bundle.go:31`).
- Drift risk: this digest is the version pin in skill URIs between hub and proxy. A field or ordering change in one copy makes every proxy reject every bundle as `ErrBundleInvalid`.
- Fix: delete `hashDistributedManifest` and `manifestDigestEntry`, and call `skillruntime.ManifestDigest`.
- Effort S. **P1**.

**D4. Two content-addressed skill-tree caches with the same placement algorithm; a race fix landed in only one**
- Local snapshots: `internal/app/skill_snapshot.go:541-586` (`materializeSnapshot`), `:788` `removeSnapshotTree`, `:801` `randomSnapshotSuffix`, and GC at `:825-845`.
- Remote cache: `internal/skillruntime/remote_cache.go:645-710` (`Install`), `:1032` `removeTree`, `:1047` `randomSuffix`, and GC at `:803`.
- Both do the same steps: stage, rename, on conflict verify the existing copy, otherwise move it to trash and retry, then remove the tree after making it writable.
- Verified drift: commit 4cc652d added `lookupSettled` (`remote_cache.go:716`) and a retry when the target is missing (`:688-694`). `materializeSnapshot` still has neither. On a missing target it returns an error immediately (`skill_snapshot.go:566-569`), and it verifies once without a settle retry (`:571`). That is the Windows sharing-violation case the fix was made for.
- Authority: one placement helper in `skillruntime` (or `fsutil`), with a verify callback. Both caches keep their own layout and marker.
- Fix: extract `PlaceVerifiedTree(staging, target, trashDir, verify func() bool)` plus a shared `RemoveTree`.
- Effort M. **P1**.

**D5. Skill audit: JS still owns the digest and reads hub.yaml itself; Go stores a digest it cannot check**
- Writes now go through `app.SkillAuditService.Record` (`internal/app/skill_audit_record.go:53-128`), which is correct.
- What remains duplicated or bypassed:
  - `audit.mjs:283-294` `readAudit` parses `.meta/hub.yaml` with a regex. This is a second YAML reader of a file that docs/design/07:463 assigns to `skillmeta` only. Flow style or a comment in the block makes it return `null`, so the skill counts as never audited.
  - `audit.mjs:184-216` and `:296` compute a "content digest" with their own file set: `.git` and `.meta` are skipped at any depth (`:25`), `skill.yaml` is included, and links are hashed as `link:<target>`.
  - The hub already has a content digest: `skillruntime.ContentDigest` (`internal/skillruntime/trust.go:56`). It leaves out hub metadata and `skill.yaml` and includes the runtime spec, and `quality.content_reviewed_digest` refers to it.
  - `SkillAuditService.Record` only checks the format of the client's digest (`skill_audit_record.go:60-64`), so `audit.content_digest` cannot be verified by the hub and cannot be compared with the approval digest.
- Concrete risk: a UI or doctor that wants to say "audited content == approved content" has no way to do it. A buggy or older script can record a digest for content it did not scan, and Go will accept it.
- Authority: Go. Covering `skill.yaml` in an audit is a legitimate difference from the approval digest, but the algorithm should still live in Go.
- Fix:
  - `audit-record` computes `content_digest` itself from the canonical tree, so the client does not send it.
  - Add `skillstation skill audit-status [--all] --json`, which returns the record and whether it is fresh. The script calls it instead of `readAudit`/`compare`.
  - Mirror the change into `internal/systemskills/skill-audit`.
- Effort M. **P1**.

**D6. Two source-storage models coexist; the v3→v4 migration deletes the directory the current code writes**
- `internal/migration/v4.go:72-86` deletes every file under `sources/intake`, `sources/catalog` and `sources/skills` (plus the legacy `distill/*`). `planV3ToV4` (`:128-185`) converts only distill entities, not source records.
- Current code still writes and reads source records in `sources/catalog/<id>.yaml` (`internal/app/source_links.go:176`, `:314`, `:393`, and `skill_add.go:1189`). It projects them (`catalog/input.go:400`), and `source capture/triage` still write `sources/intake/` (`internal/app/source.go:185`, `:430`; CLI `internal/delivery/cli/source.go:150`, `:218`).
- `workspace.requiredDirectories` still creates `sources/intake`, `sources/skills`, `distill/sources`, `distill/comparisons` and `distill/skills` (`internal/workspace/workspace.go:30-33`).
- docs/design/07 §25 (lines 1012-1017) says intake and source link files were removed and that sources are "declared directly in `.meta/skill.yaml`".
- Risk: a v3 hub that migrates loses its watched-source records from the working tree; they survive only in Git history. Docs, migration and services describe three different truths.
- Authority: needs an owner decision on which model is current. Code says `sources/catalog` records plus `hub.yaml` `sources` links.
- Fix: decide the model. Then make v4 stop deleting `sources/catalog`, or convert the records. Remove or retire intake and the dead `requiredDirectories`, and fix doc 07 §25.
- Effort M. **P1**. This is an open question for the owner.

**D7. Release authentication: the binary honours the test verifier for any base URL; the installer only for local fixtures**
- `internal/selfupdate/selfupdate.go:165` and `:674-681`: if `SKILLSTATION_FIXTURE_VERIFIER` is set, it replaces `cosign verify-blob` for every update, including the default HTTPS release URL.
- `scripts/install.sh:158-161` and `:241-246` allow the verifier only when `SKILLSTATION_LOCAL_FIXTURE` is set.
- An injection seam already exists: `Options.CosignVerifier` (`selfupdate.go:165`, `:670`).
- This is also a test-only seam in production, which AGENTS.md (Testing) forbids.
- Fix: delete the env hook in Go. Tests use `Options.CosignVerifier`. If an end-to-end binary test needs it, gate it on a local `file://` base URL as the installer does.
- Effort S. **P1**.

**D8. Resolver error classification exists twice, matched on error strings**
- `internal/delivery/cli/resolver.go:91-116` `classifyResolveError` duplicates `internal/app/error_classify.go:291-319`: the same substrings, with different Fix text.
- The app copy has a Fix that names the MCP tool `workspace_rebuild`. That is why the CLI made its own copy with `maintain rebuild`.
- The source errors are `fmt.Errorf` strings (`internal/resolver/resolver.go:41`, `:44`), not sentinels.
- Risk: rewording the resolver message silently changes the code on one surface.
- Fix: export sentinels from `resolver`, match them with `errors.Is` in `app.ClassifyError`, and keep the Fix surface-neutral or parameterised. Delete the CLI copy.
- Effort S. **P1**.

### P2

**D9. Bounded canonical file read, three identical copies**
- `internal/workspace/workspace.go:637` `readBoundedFromRoot`, `internal/catalog/input.go:175` `readCanonicalInput`, `internal/canonical/canonical.go:512` `readCanonicalFile`.
- Each does lstat, then open, then stat, then a `LimitReader` of `MaxCanonicalFileBytesV1`.
- Fix: export one `workspace.ReadCanonicalFile(root *os.Root, rel)`.
- Effort S.

**D10. Effective per-file limit is 4 MiB, but four constants claim 16 MiB "matching canonical"**
- Effective limit: `workspace.MaxCanonicalFileBytesV1 = 4 MiB` (`workspace.go:26`), enforced by `RelativeFiles` (`:774`). Mutation validates through it (`mutation/filesystem.go:504-523`).
- The 16 MiB constants:
  - `internal/delivery/cli/skill.go:432-434` ("matches the largest resource the canonical validator accepts");
  - `internal/app/skill_discovery.go:19` ("matching canonical");
  - `internal/canonical/skill.go:21` `maxSkillResourceBytes` (never reached, because the scan fails first);
  - `internal/mutation/mutation.go:29` `maxChangeBytes`.
- Result: a 5 to 16 MiB file passes the early checks and fails late with a lower-level message.
- Fix: derive these from `workspace.MaxCanonicalFileBytesV1`, or delete the unreachable ones.
- Effort S.

**D11. Hub marker and schema version read in seven or more ways, while `skillmeta.HubLayout` exists**
- `skillmeta.HubLayout` (`internal/skillmeta/skillmeta.go:297`) and `app.IsHubRoot` (`internal/app/role.go:309`) already exist.
- Re-implementations:
  - `role.go:166`;
  - `internal/canonical/skill.go:37` (unbounded `os.ReadFile` plus `Atoi`);
  - `internal/app/health_client.go:316`;
  - `internal/skillmeta/skillmeta.go:162` (inside `Locate`, using `Sscanf` again);
  - `internal/workspace/workspace.go:86` (`Discover` uses `Stat`, accepts any file type, and when a path is given returns `filepath.Abs` without checking for a marker);
  - `internal/delivery/cli/workspace_resolve.go:162`;
  - `internal/hostintegration/remote.go:135`;
  - `.agents/skills/distill-lab/scripts/distill.mjs:1010-1013`.
- The skill-audit script does no discovery: `--hub` defaults to cwd.
- Hub-root resolution order differs:
  - CLI uses five steps (`workspace_resolve.go:35-57`);
  - `workspace.Discover` walks up from cwd;
  - distill.mjs derives the hub from the file path;
  - distill-lab SKILL.md uses env, then cwd, then ask.
- Fix: `workspace.MarkerPath`, `workspace.ReadSchemaVersion(root)` and `workspace.IsHub(dir)` used everywhere. Scripts should resolve the hub through the binary, for example `skillstation status --json`.
- Effort M.

**D12. SKILL.md frontmatter: eight parsers with different rules**
- Go:
  - `internal/catalog/servable.go:41` (normalises CRLF, requires `\n---\n`);
  - `internal/skill/lifecycle.go:1100` (no CRLF normalisation);
  - `internal/app/skill_discovery.go:141` (also accepts a closing `---` at end of file) and `:188`;
  - `internal/hostintegration/skill-wrapper.go:53`;
  - `internal/canonical/skill.go:1048` (line-based `name:` only, which breaks on quoted or block scalars);
  - `internal/migration/v6.go:215` (terminator `\n---` without a newline).
- JS: `audit.mjs:109-125` (regex).
- Web: `web/src/components/Markdown.tsx:43` (display only, acceptable).
- Risk: the same SKILL.md is accepted by import and refused by edit or distribution. For example, `skill/lifecycle.go:1101` returns `ok=false` for CRLF files that `catalog.ParseSkillFrontmatter` accepts.
- Fix: one `skillmeta.SplitFrontmatter` (or a new `skill/frontmatter`) with a single decode. The canonical name check uses it.
- Effort M.

**D13. sha256 digest helpers (6) and validators (9) with diverging rules**
- Helpers: `internal/source/policy.go:19`, `internal/systemskills/bundles.go:150`, `internal/mutation/mutation.go:590`, `internal/canonical/canonical.go:507`, `internal/hostintegration/integration.go:402` (has a missing-file sentinel, which is deliberate), and `internal/app/skill_commit.go:446`. There is also inline code in `app/skill_review.go` `inspectCanonicalEntrypoint`.
- Validators: `source/filesystem.go:702`, `catalog/project.go:235`, `evaluation/parse.go:585`, `mutation/mutation.go:504`, `canonical/skill.go:606`, `distill/types.go:22`, `app/skill_audit_record.go:48`, `delivery/cli/skill.go:475`, `skillruntime/remote_cache.go:184`.
- Verified divergence:
  - `canonical.isValidDigest` (`skill.go:606`) also accepts bare 64-hex without `sha256:`. It is used without the prefix check for `provenance.origin.{folder,files,content}_digest` (`:476-481`) and `sources[].{files,folder}_digest` (`:527-531`).
  - The public schema requires `^sha256:` (`schemas/skill-metadata.schema.json:147-148`, `:183-185`).
  - String comparisons such as `upstream.go:583` (`headRev.ContentDigest == sk.Origin.FolderDigest`) then never match a bare-hex value.
- Fix: a `digest` leaf package (`Of([]byte)`, `Valid(string)`), and make `canonical` require the prefix.
- Effort S.

**D14. Skill id to folder lookup, four implementations**
- `internal/app/skill_review.go:347` `locateSkillDir`: first match wins, read errors are swallowed.
- `internal/catalog/servable.go:257`: same loop.
- `internal/skill/lifecycle.go:904` `loadSkill`: `os.Root`, refuses symlinks, errors on duplicates and on any non-directory under `skills/`.
- `audit.mjs:305-324` `findSkillDirs`: keyed by folder name, last match wins, no metadata needed.
- Plus globs: `app/curation_home.go:389` and `app/skill_record.go:26`.
- On a valid hub the results agree, because duplicates and stray files are canonical errors (`canonical.go:204`, `skill.go:572`). On an invalid hub, the surfaces disagree about which skill they act on.
- Fix: one `skillmeta.LocateSkill(read, id)`, and have the script ask the binary for the skill path.
- Effort S.

**D15. Metadata-path predicates: the same name means different things**
- `workspace.IsHubMeta` (`workspace.go:687`) does not include `skill.yaml`.
- `skillruntime.IsHubMeta` (`skillruntime/trust.go:22`) is `workspace.IsSkillMetadata` (`workspace.go:730`), which does include it.
- `skillmeta.IsMetadataPath` (`skillmeta.go:312`) is case-sensitive, accepts workspace-relative paths only, and covers `skill.meta.yaml` only.
- Example of the effect: `inventorySkillResources` counts `skill.yaml` as a content file (`app/skill_review.go`, `FileCount`), while the trust digest leaves it out.
- Fix: keep `workspace.IsSkillMetadata` and `workspace.IsHubMeta`, and rename or delete `skillruntime.IsHubMeta` and `skillmeta.IsMetadataPath`.
- Effort S.

**D16. Hub token handling, three readers with different validity rules**
- `internal/delivery/cli/mcp_proxy.go:118` `proxyToken` with `:159` `headerSafeToken` (minimum 16 characters).
- `internal/hubtoken/hubtoken.go:197` `validToken` (minimum 32), exported as `hubtoken.ValidToken` (`client_store.go:68`).
- `internal/app/doctor_remote.go:226` `ReadHubToken` (accepts any non-empty env value) and `internal/app/connect_remote.go:106` `checkTokenFile`.
- Risk: doctor reports a token as good that the proxy refuses, or the proxy sends a 16 to 31 character env token the hub can never issue, which ends in a 401 instead of a local error.
- Fix: one `app.ReadHubToken` that returns a typed problem, used by the proxy, doctor and connect, with `hubtoken.ValidToken` as the only validity check.
- Effort S.

**D17. The CLI re-implements the editor-recovery store, with a verified naming bug**
- `internal/delivery/cli/skill_editor.go:147-175` `cleanupProposalRecovery` hard-codes `runtime/edits` and the `REC-<proposalID>-` prefix. The store already exists: `internal/skill/proposal_store.go:232`, `:249`, `:305`, `:318`, exposed as `app.SkillService.DeleteEditorRecovery`.
- The editor saves with the skill id as the "proposalID" (`skill_editor.go:82`, giving `REC-<skillid>-<hash>`, `proposal_store.go:275`). The cleanup at `cli/skill.go:196` matches on the real proposal ID, so it never deletes anything. Only the 24-hour TTL removes those files.
- Fix: delete `cleanupProposalRecovery`. Have the app return the recovery ID with the proposal, or key the recovery by proposal ID.
- Effort S.

**D18. distill.yaml model: three validators, two serializers, two rounding rules**
- Validators:
  - Go: `internal/distill/distill_model.go:17-24` and `:119` (the comment says it follows distill.mjs);
  - JS: `distill.mjs:707-711` (`\p{Nd}` and the Python whitespace class, against Go's ASCII `\d` and `\s`);
  - schema: `schemas/distill.schema.json:98`, `:115`, `:167`, `:172`, which no test reads.
- Serializers: Go `distill.MarshalDocument` (`distill_model.go:278`, used by `migration/v4.go:160`) and the JS "canonical writer" (`distill.mjs` around line 1066, with folding and width 88). The first JS write after a migration reformats the whole file.
- Rounding: Go `FinalScore` uses `math.Round`, which rounds half away from zero (`distill_model.go:109-116`). JS rounds half to even (`distill.mjs:318`, pinned by `internal/systemskills/distill_script_test.go:318`). `cmd/distill-scorecard/score.go:457` compares Go's recomputed values with values JS stored, so ties such as 0.125 do not match.
- `app/curation_home.go:399-401` skips invalid documents without a word, so the Home counts drop with no warning.
- The script writing `distill.yaml` directly is documented as deliberate (`docs/contracts/web-api.md:47`, doc 07 §25), so it is accepted. The duplicated rules are the problem.
- Fix:
  - Go is the validator of record. Add a test that validates the JS fixtures against `distill.schema.json` (the round-trip test at `distill_script_test.go:342-375` already calls `distill.ValidateDocument`; add the schema).
  - Use half-to-even in Go.
  - Drop `MarshalDocument`, or make it match the JS layout.
- Effort M.

**D19. distill-lab keeps its own git mirrors next to the source adapter's mirror cache**
- `.agents/skills/distill-lab/SKILL.md:426-436` (manual `git clone --bare --filter=blob:none` into `runtime/distill-lab/mirrors/<source-id>`) and `distill.mjs:894`.
- `internal/app/source.go:132` uses `runtime/sources/git` through `GitRepositoryAdapter.mirrorPath` (`internal/source/git_repository.go:511`).
- Effect: two clones of each source, and the agent runs git network commands by hand.
- Fix: a `skillstation source mirror <id> --path` command, or document why the evidence check needs a separate partial clone.
- Effort M. Owner decision.

**D20. Atomic write: one helper plus copies**
- `fsutil.WriteFileAtomic` and `fsutil.Replace` (`internal/fsutil/atomic.go:17`, `replace_windows.go:9`) are the authority.
- Exact copy: `internal/mutation/atomic_replace_windows.go:7` and `_unix.go:7`, identical to `fsutil.Replace`.
- Weaker copy: `internal/app/checkout_marker.go:58-69` uses a fixed `.tmp` name (two writers collide) and no fsync.
- `internal/invite/filestore.go:56-73` has no fsync.
- Fix: replace these with `fsutil.Replace` and `fsutil.WriteFileAtomic`. The accepted cases are listed below.
- Effort S.

**D21. Path-safety predicates: seven different rules**
- `internal/source/policy.go:117`, `internal/skill/resource.go:108`, `internal/app/skill_add.go:1202`, `internal/app/operations.go:275` (does not reject `a/../../b`), `internal/app/skill_tree.go:50` (`filepath.IsLocal`), `internal/canonical/skill.go:588`, and `internal/app/skill_discovery.go:314`.
- The last two are byte-identical.
- No exploit found: callers either work with Git-reported paths or open through `os.Root`.
- `app/operations.go:515` `workspacePathDigest` does read a receipt-supplied path with plain `os.ReadFile`. A hand-edited receipt could point outside the hub, but only a digest-match boolean comes out.
- Fix: one `relpath.Safe` in a leaf package based on `filepath.IsLocal`, and `os.Root` for reads.
- Effort S.

**D22. Operation receipts parsed in three places**
- `internal/mutation/receipt.go:85` (`existingOperation`), `internal/app/operations.go:492` and `:589` (`walkOperationsYAML`, its own struct, follows file symlinks), and `internal/distill/workspace_validation.go:110`.
- Fix: `mutation.ReadReceipts(root)` returning `Receipt`.
- Effort S.

**D23. Canonical root path lists**
- `internal/workspace/workspace.go:30-33`, `internal/app/operations.go:288-298`, `internal/catalog/input.go:427`.
- Fix: one exported list in `workspace`.
- Effort S.

**D24. Web: error text formula repeated in 11 places, two variants**
- Variant A, `WHY || ERROR`, drops FIX: `CreateSkillScreen.tsx:160` and `:180`, `AddSkillScreen.tsx:147` and `:184`, `SkillDetailScreen.tsx:186` and `:207`, `LabelsEditor.tsx:65`, `AddSourceForm.tsx:36`, `FilesTab.tsx:58`, `ReviewChangesScreen.tsx:40`.
- Variant B, `ERROR WHY FIX`: `FullReviewScreen.tsx:74`, `ReviewChangesScreen.tsx:47`, `DiscardCheckoutDialog.tsx:17`.
- Most screens therefore lose the FIX line of the ERROR/WHY/FIX contract.
- Fix: one `errorText(err, {withFix})` in `web/src/api/client.ts`.
- Effort S.

**D25. Web: API types hand-copied**
- `web/src/api/types.ts` holds 97 interfaces and types written by hand.
- Go goldens exist (21 files in `internal/delivery/web/testdata/golden`), but only one web test reads them (`web/src/test/golden.ts`).
- Fix: type-check each golden against its TS type in a vitest suite, or generate the types.
- Effort M.

**D26. Two clock idioms**
- The `app.Clock` interface (`internal/app/clock.go:6`) is used only by `skill_add.go:182`.
- Elsewhere: `Now func() time.Time` fields (`skill_audit_record.go:45`, `hub_calls.go:213`, `transcript_import.go:53`, among others), and 42 direct `time.Now()` calls in `internal/app`.
- Fix: settle on the func field and delete `Clock`.
- Effort S.

**D27. Test-only exports in production code**
- `skillmeta.WriteFiles` (`internal/skillmeta/skillmeta.go:374-392`) writes canonical metadata with `os.WriteFile`, outside the mutation path. Its only callers are six test files. AGENTS.md says not to create exports that only tests use.
- Fix: move it to a `_test` helper package such as `internal/testenv`.
- Effort S.

**D28. Docs describe superseded layouts**
- doc 01 §7.2 (`docs/design/01-system-architecture.md:256-276`) shows `skill.meta.yaml`, `sources/intake`, and the `distill/` insight tree.
- doc 07 §4 (`07-storage-and-mutation-model.md:72-77`) shows `.meta/skill.yaml`.
- The current layout 6 (`skill.yaml` plus `.meta/hub.yaml`) is described only inside the migration step (`07:461-463`). This is the likely reason the audit author concluded "no place exists" for an audit record.
- doc 07 §25 (line 1049) says `/api/v1/skills/{id}/distill` is "GET/POST". Only GET exists (`internal/delivery/web/routes_distill.go:12`), and `web-api.md:47` says no write.
- Fix: update both layout diagrams, add a short "where each kind of data lives" table, and fix line 1049.
- Effort S.

---

## Same operation across delivery surfaces (category 2)

Sampled and confirmed to go through a single app service, with delivery only translating:
- curation home and status: `CurationService.GetCurationHome` from `cli/status_doctor.go:90`, `web/routes_read.go:37`, `mcpserver/workspace_tools.go:32`;
- content approval: web `PreviewSkillApproval` reaches `PreviewSkillUpdate`, and the CLI `--approve-content` reaches the same `checkApprovedDigest` (`app/skill_lifecycle.go:116`, `:162`);
- distill read: `GetSkillDistill`;
- audit record: `SkillAuditService.Record`.

Exceptions found: D8 (resolver error mapping in CLI), D16 (token reading in CLI), D17 (editor recovery in CLI). The archtest exception `cli -> mutation` for `maintain` recovery is documented.

## Duplicates judged acceptable

- `internal/hostintegration/files.go:422` `atomicWrite`: `os.Root`-scoped with symlink-component rejection, for host config files outside the hub. The semantics are stronger, not a copy.
- `internal/delivery/cli/evaluation.go:510` `writeNewFileAtomic`: publishes without overwriting, which is a different contract from replace.
- `internal/catalog/build.go:505` `writePointer`: needs the fault-injection points of doc 07 §23, and already uses `fsutil.Replace` and `SyncDir`.
- `internal/delivery/mcpserver/distribution.go` `bundleHTTPError` with its own `skill_not_found` code: documented at `:166-168` for compatibility with shipped proxies.
- `selfupdate.CompareVersions` (release semver with prereleases) and `skillruntime.parseVersion` (tool probe constraints): different domains and inputs.
- `schemas/skill-hub.schema.json` and `skill-metadata.schema.json` both defining `audit`: they are the split and merged shapes, and their top-level keys are tied to Go by `schemas/metadata_keys_test.go`. Nested rules are not tied; see D13.
- distill.mjs writing `.meta/distill.yaml` without the mutation path: documented in `docs/contracts/web-api.md:47` and doc 07 §25. Catalog builds re-check inventory under lock (`catalog/input.go:84-90`), so a concurrent write produces a retryable `ErrCanonicalChanged`, not a corrupt generation. The cost is that the file is never auto-committed.
- The web UI: one client (`web/src/api/client.ts`), one query module, no raw `fetch` in screens, and routes contract-tested against `web-api.md` (`internal/delivery/web/route_contract_test.go`).

## Ranked consolidation plan (lowest layer first)

1. **Leaf utilities** (`fsutil`, a new `digest` leaf, `workspace`), covering D9, D13, D20, D21, D23 and the `workspace` part of D11. These are pure moves with no behaviour change, except that `canonical` will require the `sha256:` prefix (D13).
2. **skillmeta and canonical**: D2 (export the aggregate and catalogAffecting), D11 (`ReadSchemaVersion` and `IsHub`), D12 (one frontmatter splitter), D14 (`LocateSkill`), D15 (predicate names), D27.
3. **skillruntime and mutation**: D3 (one ManifestDigest), D4 (shared tree placement with the 4cc652d retry), D22 (receipt reader), D10 (derive limits from `workspace`).
4. **app**: D5 (Go-owned audit digest and `audit-status`), D8 (resolver sentinels), D16 (token reader), D17 (recovery), D26 (clock), D6 after the owner decides the source model.
5. **Delivery, scripts and skills**: D1 (distill-lab SKILL.md), the D5 script side, D7 (self-update hook), D18 (distill rules), D19, D24, D25.
6. **Docs**: D28, and the registry below.

## How to prevent it

- **archtest additions** in `internal/archtest`:
  - (a) forbid `os.WriteFile`, `os.Rename` and `os.CreateTemp` in `internal/delivery/**` and `internal/app/**`, apart from an allowlist with reasons;
  - (b) forbid the literal `".skillstation", "schema-version"` and `"hub.yaml"` outside `workspace` and `skillmeta`;
  - (c) forbid `crypto/sha256` plus `"sha256:"` concatenation outside the `digest` package;
  - (d) forbid `regexp.MustCompile` patterns for `sha256:` outside it.
  These follow the pattern of the existing rule-and-exception table.
- **Script-side guard**: a Go test that greps `.agents/skills/*/scripts/*.mjs` for `.meta/` and `hub.yaml` reads and writes, and for `createHash`, against an allowlist. The scripts should call the binary (`--json`) for hub facts.
- **Single-authority registry**: a short table in AGENTS.md (or `docs/design/authority.md` linked from it) with one row per concept: concept, owning package or function, and the file it owns. Examples: skill metadata → `skillmeta`; canonical write → `mutation.PlanMutation`; content digest → `skillruntime.ContentDigest`; manifest digest → `skillruntime.ManifestDigest`; hub discovery → `workspace`/`app.DetectRole`; atomic write → `fsutil`. Add the rule: "Before adding storage, a digest, a parser or a helper, look it up in this table and grep the owner; a second implementation needs a row explaining why."
- **Schema parity tests**: extend `schemas/metadata_keys_test.go` from top-level keys to nested patterns, by validating Go-written fixtures against the JSON schemas, including `distill.schema.json`.
- **Cross-copy tests while copies remain**: for example, one test that hashes a fixture with both manifest digest functions until D3 lands.

## Unresolved questions

1. D6: which source model is current, `sources/catalog` records plus `hub.yaml` links, or "sources only in skill metadata" as doc 07 §25 says? Are any v3 hubs still in use that would hit the v4 deletion?
2. D5: should the audit digest cover `skill.yaml` (wider than the approval digest) on purpose? That decides whether to reuse `ContentDigest` or add a second Go-owned digest.
3. D19: does the distill evidence check need its own blob-less mirror, or could it read from `runtime/sources/git`?

Status: DONE_WITH_CONCERNS
Summary: Found 28 verified duplicate or bypass findings: 1 P0 (distill-lab's instructions make a layout-6 hub invalid), 7 P1, and 20 P2. Each has file:line evidence, an authority, a fix and an effort, followed by a consolidation order and prevention mechanisms.
Concerns/Blockers: The tree was being edited during the review, so line numbers may shift. No tests were run. D6 and D19 need an owner decision.
