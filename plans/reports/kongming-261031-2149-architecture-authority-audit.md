# Audit kiến trúc, authority và độ sẵn sàng phát hành — Skillstation

Ngày: 2026-10-31. HEAD `b2ecf28` (= tag `v0.12.0`). Audit chỉ đọc; không sửa file nào trong repo. Mọi lệnh chạy trên bản build từ source trong scratchpad và trên hub tạm, không chạm hub thật của bạn (`~/projects/skillstation-hub`, chỉ `ls`/`cat`).

## Kết luận ngắn

Nền tảng **tốt và ổn định**: 33 package test xanh (`go test ./... -count=1`, 74 s), `-race` xanh trên 8 package lõi, lint 0 issue mới, deadcode 0 mới, `GOOS=windows go vet` sạch, archtest giữ đúng chiều import, mọi ghi canonical (kể cả hub.yaml) đều đi qua `mutation.PlanMutation` — tôi không tìm thấy delivery nào ghi hub trực tiếp. Release pipeline ký keyless cosign, SBOM, archive deterministic, pure-Go SQLite.

Nhưng nghi ngờ của bạn **đúng ở tầng dưới app**: "single authority" được *tuyên bố* trong AGENTS.md nhưng ở mức helper thì có nhiều bản sao: 3 bản `atomic replace` giống hệt nhau, 7 chỗ tự tính `sha256:`+hex, 7 hàm kiểm tra dạng digest, **3 cách ghi hub.yaml cho ra 3 bố cục khác nhau** (đã tái hiện: `skill edit` đảo thứ tự key và sẽ làm mất comment), 2 hàm tìm thư mục skill theo id, 3 hàm liệt kê id. Chính vì "sửa một key trong hub.yaml" không có helper chung và không có cửa chung, nên mỗi key mới cần một service riêng; agent của bạn sáng nay không thấy cửa `audit-record` vì **curator SKILL.md, MCP, `skill review`, `skill show` và web đều không nhắc tới audit** — cửa tồn tại nhưng chỉ nằm trong `skill --help` và trong SKILL.md của skill-audit.

Hiệu năng: mỗi tiến trình CLI quét và băm lại toàn bộ cây hub để kiểm tra catalog còn tươi (cache stat chỉ sống trong RAM của tiến trình). Hub 300 skill / 1 502 file / 13 MB: `status` 0,7–0,85 s và 74 743 `openat` mỗi lần; `skill list` 39 292 `openat` vì mở catalog hai lần. Chưa đau ở 6 skill, sẽ đau ở 1 000.

Release-ready: **có, với 3 điều kiện** — gate release theo CI (hiện tag push bỏ qua lint/deadcode/vulncheck/web test), mã lỗi `git_failed` chưa có trong contract và render sai định dạng ERROR/WHY/FIX, envelope `Result` lệch với doc và schema của chính nó.

## Top 10 việc cần làm, theo thứ tự tầng thấp trước

| # | Tầng | Việc | Finding |
|---|---|---|---|
| 1 | fsutil | Xoá `mutation.atomicReplace` + `mutation.syncDir` + phần temp-write của `mutation.writeFileSync`, và `cli.writeNewFileAtomic`; tất cả gọi `fsutil.WriteFileAtomic`/`Replace`/`SyncDir` | M1 |
| 2 | digest (mới, dưới skillruntime) | Một `FileDigest([]byte) string` và một `ValidDigest(string) bool`; thay 7 + 7 bản sao; thêm dòng vào bảng authority | M2, M3 |
| 3 | skillmeta | Xuất helper sửa **một key** của hub.yaml trên `yaml.Node` (giữ thứ tự, comment); `withAuditRecord`, `stampHub` và `skill.applyUpdateFields` đều dùng nó; bỏ đường `map[string]any → yaml.Marshal` | D1, M4 |
| 4 | app | Một `locateSkillDir` (lấy bản an toàn symlink của `skill.loadSkill`), một `hubSkillIDs`; xoá alias `skill.ResolveWorkspace` | M5, M9 |
| 5 | app | Đưa kiểm tra digest khi `--approve-content` vào `SkillCommitService.Apply` (nhận `ExpectedContentDigest`, không chỉ `bool`); đưa gating `skill_get` vào `ReadSkillCore` | D2, M11 |
| 6 | app + docs | Hiện khối `audit` trong `skill review`/`skill show` (text + JSON) và `skill_review` MCP; thêm dòng `audit-status`/`audit-record` vào bảng của curator SKILL.md | G1 |
| 7 | contracts | Thêm `git_failed` vào error-codes.md, schema enum, `web/errors.go`; render đúng 3 dòng; sửa `result-envelope.md` + schema cho khớp `Result` (`schema_version`, `details`) và thêm test schema; bọc `doctor --json`, `telemetry cases --json` vào envelope | D3, D4 |
| 8 | catalog/canonical | Lưu stat-stamp cache ra `runtime/catalog/` để CLI không băm lại cây mỗi lần; `ListSkillCatalog` mở catalog một lần | D5 |
| 9 | release | `release.yml` chờ CI (hoặc chạy lint/deadcode/vulncheck/web test trong verify); `git diff --exit-code` sau `npm run notices`; cân nhắc mặc định `SKILLSTATION_REQUIRE_SIGNATURE=1` | D6 |
| 10 | archtest + deadcode | Bỏ 2 exception trong `layers_test.go` bằng cách xuất sentinel từ app; xoá `skillmeta.WriteFiles` (export chỉ test dùng) | M10, M11 |

## 0. Bằng chứng đã chạy

| Lệnh | Kết quả |
|---|---|
| `go test ./... -count=1` | 33 package ok, 74 s wall; chậm nhất `delivery/cli` 70 s, `delivery/mcpserver` 69 s |
| `go test -race` (fsutil, skillmeta, mutation, workspace, catalog, skillruntime, canonical, app) | ok (app 104 s) |
| `make lint` | 0 issue mới |
| `make lint-all` | 70 issue nợ cũ: errcheck 25, staticcheck 32, ineffassign 5, unused 6, funlen 2 — 6 trong `web/fixtures_test.go` |
| `make deadcode` | 0 mới, 21 trong baseline |
| `GOOS=windows go vet ./...` | sạch |
| `make test-perf` | resolver: uncached resolve 256 skill p50 25,5 ms / p95 45,9 ms / p99 63 ms. mcpserver: resolve p50 6,0 ms / p95 15,7 ms; RSS 30,3 MB → 43,0 MB sau 1 001 resolve → 38,3 MB sau 1 phút idle; 2002/2002 event |
| Hub tạm 300 skill (1 502 file, 13 MB) | `status` 0,69/0,81/0,71/0,80/0,85 s, RSS 33 MB, 74 743 `openat`; `skill list` 1,73 s lạnh / 0,57 s ấm, 39 292 `openat`; `audit-status --all` 0,44 s, 37 013 `openat`; `skill review` 0,75 s; `doctor` 0,69 s |

## 1. Đối chiếu bảng "Single authority" trong AGENTS.md

| Concept | Owner tuyên bố | Thực tế | Kết luận |
|---|---|---|---|
| Skill metadata files | `internal/skillmeta` | Locate/Read/Merge/Split đúng là duy nhất. Nhưng **ghi** hub.yaml có 3 bộ mã YAML khác nhau: `app/skill_audit_record.go:154-180` (node, giữ thứ tự), `mutation/skill_metadata.go:198-258` (node, giữ thứ tự), `skill/lifecycle.go:325-365` + `:895-901` (`map[string]any` → `yaml.Marshal`, **sắp xếp lại theo alphabet, mất comment**) | Thiếu owner cho "sửa một key" (M4, D1) |
| Writing canonical hub files | `mutation.PlanMutation`/`ConfirmMutationWithOptions` từ app | Đúng. Không có `PlanMutation` trong `internal/delivery` | Đạt |
| Skill content digest | `skillruntime.ContentDigest`, `app.SkillService.ContentTrustFor` | Hàm đúng là một, nhưng 5 nơi trong app tự lắp danh sách `ResourceDigest` để gọi nó: `skill_review.go:459`, `draft_serving.go:115`, `skill_commit_plan.go:449`, `upstream.go:163`, `upstream_origin.go:46`; 2 nơi sau truyền `Spec{}, false` (khái niệm `files_digest` cho upstream, không phải digest approval) | Một hàm, hai khái niệm chung tên — nên tách tên (M6) |
| Distributed bundle digest | `skillruntime.ManifestDigest` | Không thấy bản thứ hai | Đạt |
| Skill audit record | `app.SkillAuditService` | Đúng là một; chỉ CLI gọi (`cli/skill_audit_record.go:29`, `cli/skill_audit_status.go:35`); MCP, web, `skill review` không biết | Đạt về owner, hụt về cửa (G1) |
| Sources | `app.SourceService` | Không kiểm tra sâu; MCP không có tool attach/detach (scan phụ) | Chưa xác minh toàn bộ |
| `sources` block của distill.yaml | `distill.ReadSources`/`WithSources` | Không kiểm tra sâu | Chưa xác minh |
| Atomic file write | `internal/fsutil` | **Vi phạm**: `mutation/atomic_replace_{unix,windows}.go` sao chép nguyên văn `fsutil/replace_{unix,windows}.go`; `mutation/filesystem.go:44-78` `writeFileSync` làm lại `WriteFileAtomic`; `mutation/filesystem.go:431` `syncDir` làm lại `fsutil.SyncDir`; `cli/evaluation.go:510` `writeNewFileAtomic` bản thứ ba | M1 |
| Skill id form | `skillmeta.ValidSkillID` | Duy nhất. Các regex khác (`distill` key, hub id ở `remote_cache.go:184`, `hostintegration/remote.go:112`) là khái niệm khác | Đạt |
| Hub folder discovery | `workspace.Discover` | `skill.ResolveWorkspace` (`skill/lifecycle.go:1331`) là alias thuần, app dùng cả hai tên; `cli/workspace_resolve.go:164` tự `os.Stat(".skillstation")` | Hai tên một cửa (M9) |
| ERROR/WHY/FIX | `app/errors.go`, `ClassifyError` | 33 mã khớp doc/schema; **`git_failed`** định nghĩa ở `app/workspace_init.go:24`, không có trong `errors.go`, doc, schema, `web/errors.go`; CLI render riêng ở `cli/workspace.go:93-98` | D3 |
| Import rules | `internal/archtest` | Đúng, nhưng 2 exception đã lỗi thời (M11) | Đạt, cần dọn |

Khái niệm **không có dòng nào** trong bảng nhưng có nhiều bản cài: digest của một mảng byte (M2), kiểm tra dạng digest (M3), tìm thư mục skill theo id (M5), liệt kê id skill (M5), sửa một key hub.yaml (M4).

## 2. Findings

Mức: **cao / vừa / thấp**. Mỗi finding: sai gì — bằng chứng — owner nên là ai — sửa nhỏ nhất.

### A. Lỗi thật

**D1 (cao) — `skill edit` ghi lại toàn bộ hub.yaml theo thứ tự alphabet, mất comment.**
Tái hiện trên hub tạm: sau `skill audit-record` hub.yaml có `quality, status, created_at, updated_at, audit`; chạy `skill edit demo --description ... --yes` xong, file thành `audit (audited_at, content_digest, rules_digest, verdict), quality, status, created_at, updated_at` — khối audit nhảy lên đầu, bên trong cũng xếp lại. Nguyên nhân: `skill/lifecycle.go:952-1021` decode vào `map[string]any`, `:325-365` sửa map, `:895-901` `yaml.Marshal(map)`. Trái với `docs/design/07-storage-and-mutation-model.md` §6.3 "Stable serialization" và với cam kết đầu `internal/skillmeta/skillmeta.go:12-13` "comments stay with their keys either way". Hệ quả: diff Git nhiễu, comment người dùng viết trong hub.yaml mất ở lần `skill edit` đầu tiên.
Owner: `skillmeta` (đã có `mappingOf`, `Encode`). Sửa: xuất `skillmeta.SetKey(contents []byte, key string, value *yaml.Node) ([]byte, error)` (và `DeleteKey`), cho `skill.applyUpdateFields` làm việc trên `yaml.Node` thay vì map; `withAuditRecord` và `stampHub` gọi cùng helper.

**D2 (cao) — Kiểm tra digest khi approve nằm ở CLI, app chấp nhận `Approve bool`.**
`internal/delivery/cli/skill_commit.go:57-63` so `flags.approveContent` với `preview.Trust.ContentDigest` rồi đặt `options.Approve = true`; `internal/app/skill_commit_apply.go:88-93` chỉ nhìn `options.Approve`. Caller khác của `SkillCommitService.Apply` (web, MCP tương lai, test) có thể approve nội dung bên thứ ba mà không nêu digest. Đường submission đã làm đúng trong app (`app/submission_review.go:153`, theo scan phụ).
Owner: `app.SkillCommitService`. Sửa: `SkillCommitApplyOptions.ApproveContentDigest string`; app so với `plan.contentDigest`, trả `NewInvalidRequestError` khi lệch; CLI chỉ truyền chuỗi.

**D3 (vừa) — `git_failed` ngoài contract.**
Định nghĩa `app/workspace_init.go:24`, dùng ở `:195`, `app/workspace_remote.go:90`. Không có trong `app/errors.go` (nơi bảng mã sống), `docs/contracts/error-codes.md`, `schemas/error-envelope.schema.json` enum, `web/errors.go` (web trả 500). `cli/workspace.go:93-98` in `ERROR:` rồi nhiều dòng thô rồi FIX không nhãn — phá quy tắc "đúng 3 dòng có nhãn" ở `error-codes.md:20-24`.
Owner: `app/errors.go`. Sửa: chuyển hằng vào `errors.go`, thêm vào doc + schema + map HTTP (502 hoặc 500 có chủ đích), render qua `termui.Printer.Error` với WHY nhiều dòng.

**D4 (vừa) — Envelope `Result` lệch doc và schema; không có test schema.**
`app/result.go:21` có `schema_version`, `:28` có `details`; `docs/contracts/result-envelope.md:119-123` nói `schema_version` không được thêm vào envelope chung; `schemas/result-envelope.schema.json` `additionalProperties:false` và không khai `details` → payload có `details` sẽ fail schema. Không test Go nào tham chiếu `result-envelope.schema.json`. `doctor --json` (`cli/status_doctor.go:443`) và `telemetry cases --json` (`cli/telemetry_cases.go:137-144`) in JSON ngoài envelope (scan phụ, file:line đã nêu).
Owner: `app.Result` là sự thật; doc/schema phải theo. Sửa: cập nhật doc + schema, thêm 1 test trong `schemas/embed_test.go` validate một `Result` thật, bọc 2 lệnh kia hoặc ghi rõ ngoại lệ.

**D5 (vừa) — Mỗi tiến trình CLI băm lại cả cây hub.**
`internal/canonical/snapshot_cache.go:23-24` cache stat chỉ trong bộ nhớ tiến trình (`maxCachedRoots = 16`), không ghi ra đĩa; `catalog/open.go:87` gọi `canonical.ScanCached(root)` mỗi lần mở. Số đo: hub 300 skill, `status` 74 743 `openat`/32 224 `read` mỗi lần, 0,7–0,85 s cả khi không có gì đổi; `skill list` 39 292 `openat` vì `ListSkillCatalog` gọi `ListSkills` (mở catalog, `skill_list.go:76`) rồi mở lại (`:133`). Tuyến tính theo số file; 1 000 skill ≈ 2,5 s cho mỗi lệnh. MCP server lâu dài không bị (cache sống).
Owner: `canonical` (scan) + `catalog` (open). Sửa nhỏ nhất: (a) `ListSkillCatalog` mở một handle và truyền xuống; (b) lưu bảng stamp→digest vào `runtime/catalog/stamps.json` sau mỗi scan, nạp lúc khởi động, vẫn giữ racy-window.

**D6 (vừa) — Release gate.**
`.github/workflows/release.yml:3-6` chạy khi push tag; job verify (`:159-223`) không chạy `make lint`, `make deadcode`, `make vulncheck`, web unit/e2e — chúng chỉ ở `ci.yml`. `internal/selfupdate/selfupdate.go:343-349`: thiếu `cosign` → in "Signature not checked" và đi tiếp; chỉ cứng khi `SKILLSTATION_REQUIRE_SIGNATURE=1` (`:157-175`). `ci.yml:247-250` tái sinh `web/THIRD_PARTY_NOTICES.md` nhưng không `git diff --exit-code`. Không có notice cho Go module trong archive (chỉ SBOM). (Scan phụ đọc; tôi xác nhận các dòng trigger và selfupdate.)
Sửa: `workflow_run` chờ CI hoặc thêm 4 bước vào verify; diff notices; cân nhắc mặc định bắt buộc chữ ký khi cosign có sẵn hoặc tải cosign theo.

### B. Trùng lặp / thiếu authority

**M1 (cao, tầng thấp nhất) — Atomic write có 3 bản.**
`fsutil/replace_unix.go:8` ≡ `mutation/atomic_replace_unix.go:7`; `fsutil/replace_windows.go:9-19` ≡ `mutation/atomic_replace_windows.go:7-17` (cùng `MoveFileEx` flags); `mutation/filesystem.go:44-78` `writeFileSync` lặp `fsutil.WriteFileAtomic` (`fsutil/atomic.go:17-52`) chỉ thêm kiểm tra symlink đích; `mutation/filesystem.go:431` `syncDir` lặp `fsutil.SyncDir`; `cli/evaluation.go:510-560` bản thứ ba. Archtest không cấm `mutation → fsutil`.
Sửa: `writeFileSync` giữ kiểm tra an toàn rồi gọi `fsutil.WriteFileAtomic(target, contents, 0o600)`; xoá 2 file `atomic_replace_*`; CLI gọi `fsutil`.

**M2 (vừa) — Digest của byte có 7 bản.**
`source/policy.go:19 Digest`, `mutation/mutation.go:581 digest`, `canonical/canonical.go:504 contentDigest`, `systemskills/bundles.go:150 FileDigest`, `hostintegration/integration.go:478 digest`, `app/skill_review.go:380-383` và `:423-426` (inline), `app/skill_commit.go:446 digestOf` (alias sang `source.Digest`). Bảng authority chỉ có dòng cho digest *skill* (skillruntime) và *bundle*, không có dòng cho digest *file*.
Sửa: package nhỏ không phụ thuộc gì (`internal/digest`: `Bytes([]byte) string`, `Valid(string) bool`), thêm dòng vào bảng; thay 7 chỗ. Không đặt vào `skillruntime` vì `mutation`/`canonical`/`source` nằm dưới nó.

**M3 (vừa) — Kiểm tra dạng `sha256:<64 hex>` có 7 bản, không đồng nhất.**
`catalog/project.go:245`, `source/filesystem.go:717`, `evaluation/parse.go:562`, `mutation/mutation.go:495`, `canonical/skill.go:609`, `app/skill_audit_record.go:50` (regex), `distill/types.go:22` (regex). Khác nhau nhỏ: `mutation.validOptionalDigest` không ép chữ thường; `canonical.isValidDigest` ép; `evaluation` hard-code 71. Gộp vào `digest.Valid`.

**M4 (cao) — Sửa một key của hub.yaml không có helper.**
Ba bộ mã (xem D1). `mutation/skill_metadata.go:231-258` có `mappingValue/removeMappingKey/setMappingString` chưa xuất; `app/skill_audit_record.go:162-179` viết lại vòng lặp tương tự. Đây là gốc của chuyện "không có chỗ để làm": muốn thêm key `approval`, `audit`, `quality.x` đều phải viết lại vòng lặp node và một service riêng. Sửa như D1.

**M5 (vừa) — Tìm skill theo id và liệt kê id có nhiều bản, an toàn khác nhau.**
`app/skill_review.go:337-364 locateSkillDir` (ReadDir thường, không chặn symlink) vs `skill/lifecycle.go:952-1021 loadSkill` (`os.OpenRoot`, Lstat, từ chối symlink, phát hiện id trùng). Liệt kê: `app/source_import.go:613 listWorkspaceSkillIDs`, `app/upstream.go:177 listAllSkillIDs` (hỏi catalog trước, nuốt lỗi rồi fallback), `app/skill_audit_status.go:133 hubSkillIDs`. Sửa: một `skill.Locate(root, id)` trả `(collection, relDir, metaBytes)`, dựng trên `loadSkill`; một `skill.ListIDs(root)`.

**M6 (thấp) — `skillruntime.ContentDigest` phục vụ hai khái niệm.**
`upstream.go:163` và `upstream_origin.go:46` gọi với `Spec{}, false` để tính `files_digest` của upstream; 3 nơi khác tính digest approval (có runtime). Cùng hàm, khác nghĩa, dễ nhầm khi so sánh. Sửa: `skillruntime.FilesDigest(files)` riêng tên.

**M7 (thấp) — Quyết định trust đọc `content_reviewed_digest` ở 10 file.**
`skill/lifecycle.go` (6), `app/skill_review_changes.go` (4), `canonical/skill.go` (3), `cli/skill.go` (2), `app/skill_snapshot.go` (2), `app/skill_review.go` (2), `app/skill_lifecycle.go` (2), `app/skill_review_content.go`, `app/skill_commit_plan.go`, `app/skill_commit_meta.go`. `skillruntime.Evaluate` là owner đúng, nhưng struct decode hub.yaml là ad-hoc ở mỗi nơi (`contentTrustDocument`, `skillReviewMeta`, `RecordedAudit`, …; 11 file trong app có tag `yaml:"status|quality|provenance|…"`). Sửa: một `skillmeta.Hub` struct typed (status, quality, provenance, audit, sources) decode một lần; 11 struct ad-hoc dùng nó.

**M8 (thấp) — `distill/types.go:20 stableKeyPattern` ≡ `distill/distill_model.go:17 KeyPattern`** cùng package, cùng regex. Xoá một.

**M9 (thấp) — Hai tên cho một cửa discovery.** `skill.ResolveWorkspace` = `workspace.Discover` (`skill/lifecycle.go:1331`); app gọi cả hai. `cli/workspace_resolve.go:164` tự `os.Stat(".skillstation")`. Sửa: xoá alias; CLI hỏi `workspace`.

**M10 (thấp) — `skillmeta.WriteFiles` (`skillmeta.go:346-361`)** export chỉ test dùng, ghi bằng `os.WriteFile`, nằm trong deadcode baseline. Vi phạm "Do not create exports that only tests use". Chuyển vào `internal/testenv`.

**M11 (vừa) — Logic nghiệp vụ trong delivery; exception archtest lỗi thời.**
`mcpserver/skill_tools.go:191-205` quyết định gating review và ngoại lệ curator; `mcpserver/skills_extension.go:79-90` lọc `review_required`; `mcpserver/distribution.go:165-170` map sentinel lặp `app/skill_bundle.go:147`; `web/routes_submissions.go:86-97` dọn queue sau confirm trong khi CLI để app làm (`app/submission_review.go:67`). Exception `cli -> mutation` chỉ còn `cli/status_doctor.go:91 errors.Is(err, mutation.ErrWorkspaceBusy)` (ghi chú "maintain commands run recovery directly" đã sai); `mcpserver -> catalog` chỉ còn sentinel. (Scan phụ đọc; tôi xác nhận `skill_tools.go:186-208`.) Sửa: app xuất `ErrWorkspaceBusy`/`ErrCatalogUnavailable` hoặc phân loại qua `ClassifyError`; `ReadSkillCore` trả `Local`+`Content` đã gated; bỏ 2 exception.

**M12 (thấp) — Hai thông điệp cho cùng `not_found`.** `audit-status demo` → `No skill "demo" in this hub.` / FIX `skill list to see the skill ids`; `audit-record demo` → `skill not found: demo` / FIX `skill list to inspect available skills` (quan sát trực tiếp; `app/skill_audit_status.go:103` vs lỗi thô từ `locateSkillDir` ở `skill_audit_record.go:79`). Cùng `ClassifyError`, khác render.

**M13 (thấp) — Contract web viết tay hai lần.** `web/src/api/types.ts` 1 096 dòng, 97 export, mô phỏng JSON Go bằng tay; chỉ route được pin (`web/route_contract_test.go:21`), không có test pin shape. Khi `Result` thêm `details` (D4) TS không biết. Sửa: sinh TS từ Go (tygo/quicktype) trong `make web-check` hoặc ít nhất một fixture JSON do Go sinh mà vitest nạp.

### C. Hụt discoverability — tái hiện kịch bản "cập nhật audit hash"

**Lộ trình được hỗ trợ hôm nay (đã chạy end-to-end trên hub tạm):**

1. `skillstation skill audit-status <id> --json` → `since_last_audit: content-changed`, in `content_digest` hiện tại.
2. `node .agents/skills/skill-audit/scripts/audit.mjs --record <id> --verdict ok` (script tính `rules_digest` rồi gọi `skillstation skill audit-record <id> --verdict ok --rules-digest sha256:…`).
3. `audit-record` đi qua `PlanMutation` (`app/skill_audit_record.go:102-118`), chỉ thay khối `audit:` (`:154-180`), không đổi `updated_at` (`mutation/skill_metadata.go:121-136`), commit "Record the ok audit of <id>". `audit-status` → `unchanged`.

`content_digest` **do hub tính** (`auditContentDigest` → `ContentTrustFor`), không có tham số để đặt tay — "cập nhật hash" đúng nghĩa là *ghi lại audit sau khi nội dung đổi*. Hub thật của bạn: schema 7, 6 skill, **0 skill có khối `audit:`** nên không có ví dụ nào để agent bắt chước.

**Vì sao agent không thấy cửa (mỗi điểm đã kiểm tra):**

| Nơi agent tìm | Có nhắc audit-record / hub.yaml audit? |
|---|---|
| `system-skills/curator/SKILL.md:184` (skill agent nạp khi làm việc với hub) | Chỉ nêu lệnh scan `audit.mjs <id> --json`; **không** nêu `--record`, `audit-record`, `audit-status`, khối `audit:` |
| MCP (26 tool, `mcpserver/tool_registration.go:10-41`) | **Không** có tool audit; `skill_review` không trả audit |
| `skillstation skill review <id>` / `--json`, `skill show <id>` | **Không** hiện khối audit (kiểm tra output trực tiếp) |
| Web UI | Không có màn/route audit (`grep -ri audit web/src` chỉ ra 1 test không liên quan) |
| `skillstation skill --help` (`cli/help.go:203-206`, `cli/skill_help.go:241-262`) | **Có**, đầy đủ |
| `.agents/skills/skill-audit/SKILL.md` §4 (`:82-100`) | **Có**, đầy đủ |
| AGENTS.md bảng authority | Có dòng `app.SkillAuditService`; câu "per-skill hub state … is a key in .meta/hub.yaml, never a new file" đúng nhưng không chỉ cửa ghi |

Thêm nữa: `skill audit-record` **bắt buộc** `--rules-digest` (`cli/skill_audit_record.go:77`) mà chỉ script biết tính (`audit.mjs:302,382`), nên agent gõ thẳng CLI sẽ bị chặn với FIX chỉ sang script — nếu agent không nạp skill-audit thì vòng lặp kết thúc ở "không có chỗ".

Sửa nhỏ nhất (G1): (a) thêm một dòng vào bảng của curator SKILL.md: "Record/check a security audit → `audit.mjs --record` / `skill audit-status`"; (b) `SkillService.ReviewSkill` trả `Audit *RecordedAudit` + `SinceLastAudit`, CLI/web/MCP hiện; (c) `audit-record` chấp nhận thiếu `--rules-digest` bằng cách tự băm `rules/rules.txt` + `config/audit-rules.txt` của hub (cùng thuật toán script) — hoặc giữ bắt buộc nhưng FIX nêu cả hai đường.

**G2 (vừa) — Thao tác app chỉ có một cửa** (scan phụ, bảng reachability): `SkillAuditService.Record/Status` (CLI), `SkillService.ContentTrustFor` (MCP), `SkillService.PreviewSkillApproval`/`ReviewSkillContent` (web), `SkillCommitService.Apply` (CLI), `SourceService.SkillSources` (web). Approve có 4 đường (CLI edit, CLI commit, CLI submission, web); MCP không có — đúng chủ ý. Kiểm tra từng dòng khi làm #5/#6.

**G3 (thấp) — Design doc chỉ tên API không tồn tại.** `docs/design/07:884-897` liệt kê `ExecuteCanonicalMutation`, `RecoverPendingMutation`, `PublishCatalogGeneration`, `GetWorkspaceStatus` — không có hàm nào tên vậy (grep). Agent đọc design sẽ tìm sai. `docs/use-cases/03-cli-and-curator-mcp-mapping.md:365` nêu tool `source_triage` không có trong code (scan phụ).

### D. Chấp nhận được — và vì sao

- **Ghi canonical qua một cửa**: không có `PlanMutation`/ghi `.meta/` trong `internal/delivery`; mọi `os.WriteFile` ngoài fsutil (danh sách 45 dòng) là staging/scaffold/self-update/scorecard/unit file, không phải hub canonical. Ba chỗ ghi state runtime không atomic (`skillruntime/remote_cache.go:985`, `app/submissions.go:378,460`, `telemetry/cases.go:92`) — rủi ro thấp, dữ liệu dựng lại được.
- **Locking**: `mutation/lock.go` flock, timeout bị chặn, `TryAcquireSharedLock` cho đọc nhanh; catalog pin generation bất biến; `-race` sạch.
- **Windows/EOL**: `.gitattributes` `* text=auto eol=lf`; vet Windows sạch; `fsutil.Replace` dùng `MoveFileEx|WRITE_THROUGH`; frontmatter chấp nhận `---\r\n` (`canonical/skill.go:1050`).
- **Goroutine/handle**: 7 `go func` ngoài test, tập trung ở mcpserver/web; heuristic Open/Close không thấy lệch (trừ `source/nofollow_other.go` trả file cho caller — không rò). `listAllSkillIDs` đóng rows bằng defer.
- **Bộ nhớ MCP**: 43 MB sau 1 001 resolve, co về 38 MB khi idle; heap 6,7 MB. Không có cache không chặn (`maxCachedRoots = 16`, `submissionIntakes` theo root).
- **SQLite**: `modernc.org/sqlite` pure Go, `CGO_ENABLED=0` trong release; projection ở `runtime/catalog/` (42 MB cho 300 skill — kiểm tra đó là generation cũ chưa dọn hay không là việc nhỏ; chưa đào).
- **Release**: keyless cosign + attest, SBOM syft, archive deterministic (`--mtime=SOURCE_DATE_EPOCH`, `gzip -n`), `-trimpath -buildvcs=false`, version stamp qua ldflags vào `internal/version`. `known-digests`/`known-wrappers` được test từ tag với `fetch-depth: 0` ở cả CI và release.
- **Web dist**: không track trong Git (chỉ `.gitkeep`); release build UI trước rồi `go build`; dev build thường trả 503 "web UI is not built" — chấp nhận được, nhưng là lý do `make check` không bắt được lỗi UI.
- **Test lớn chậm** (`cli` 70 s, `mcpserver` 69 s) vì chạy binary thật; chấp nhận, nhưng 2 package này chiếm 95 % thời gian — cân nhắc `-short` cho vòng lặp cục bộ.

## 3. Ranh giới và layering

- `internal/app` là **god package**: 117 file không-test, 34 `*Service`; archtest chỉ ràng theo thư mục nên bên trong app mọi service gọi chéo helper của nhau tự do (ví dụ `locateSkillDir` sống trong `skill_review.go` nhưng được audit/trust/approval dùng). Đó là nơi trùng lặp mọc (M5, M6, M7). Không cần tách package ngay; cần **một file "owner" cho mỗi khái niệm** (`skill_locate.go`, `hub_meta.go`) và dọn 3 bộ helper.
- `internal/skill` (1 331 dòng `lifecycle.go`) là tầng domain thật sự nhưng chỉ app và CLI import; nó vừa lập kế hoạch mutation vừa có writer YAML riêng (D1). Sau khi #3 xong, `skill` không còn ghi YAML.
- Hàm dài: 2 funlen nợ cũ (lint-all), không hàm nào mới vượt 120. 25 errcheck và 32 staticcheck nợ cũ tập trung ở test và `source/`; không ảnh hưởng release.
- archtest thiếu: không ràng `skillruntime`, `skill`, `workspace`, `fsutil` (tầng thấp nhất chưa có rule "không import app/catalog/mutation"); không chặn `mutation → fsutil` (tốt) nhưng cũng không *bắt* dùng.

## 4. Hợp đồng

Xem D3, D4, M13. Thêm: 6 schema không có test nào validate (`distill`, `result-envelope`, `source-record`, `recommendation-policy-v1`, `action-confirmation-policy`, `canonical-entity`); `distill.schema.json` không có `schema_version`; `schema_version` khi là chuỗi `"1"` (envelope, resolve) khi là số `1` (hub, settings) — Go chấp nhận cả hai nhưng TS/JSON consumer thì không. Tool-list digest test (`mcpserver/tools_list_digest_test.go:13-25`) là **change detector**, không phải contract: commit `c028932` re-pin −81 byte mà không nêu tool nào đổi. Web routes: 44/44 khớp `web-api.md` nhờ test — mẫu tốt, nên nhân rộng cho MCP tool list (pin tên + required fields thay vì digest byte).

## 5. Ổn định

Không tìm thấy lỗi crash-safety ở đường ghi canonical: temp → fsync → rename → fsync dir, WAL + recovery, before-digest check. Rủi ro còn lại là ở tầng *đọc*: `locateSkillDir` không chặn symlink trong khi `loadSkill` có (M5) — nếu hub bị cấy symlink ra ngoài, `ContentTrustFor`/audit vẫn băm file ngoài hub. Mức thấp vì `canonical`/`workspace.RelativeFiles` từ chối symlink khi build catalog, nhưng đường audit/trust đi thẳng FS.

## 6. Hiệu năng và bộ nhớ

Số liệu ở §0; phân tích ở D5. Điểm nóng theo thứ tự: (1) scan+hash toàn cây mỗi tiến trình CLI; (2) `ListSkillCatalog` mở catalog hai lần; (3) `ContentTrustFor` đọc và băm mọi file của skill mỗi lần gọi (`skill_review.go:408-430`, SKILL.md băm hai lần: `:375-383` rồi `:423`), `audit-status --all` gọi nó N lần — 0,44 s cho 300 skill, ổn; (4) `skill list` cold 1,7 s / RSS 73 MB — một phần page cache. Resolver và MCP đạt budget.

## 7. Release readiness

Xem D6 và §2.D. Tóm lại: **phát hành được**, với điều kiện gate CI trước tag và vá `git_failed` (một người dùng Windows gặp lỗi Git sẽ thấy output không đúng contract). Bước tiếp cho người bảo trì: khi sửa bất kỳ file trong 3 system skill, nhớ thêm digest vào `known-digests.go`/`known-wrappers.go` — test đã ép.

## 8. Giả định và điều chưa xác minh

- Hub tạm dùng skill nhỏ (2 file + 20 KB notes); hub thật có thể nặng hơn, số `openat` tỉ lệ theo số file, không theo byte. Tin cậy: cao.
- Các dòng của scan phụ (M11 phần web/submission, D6 phần notices, G2 bảng reachability) tôi chỉ xác nhận lại những dòng ghi rõ "tôi xác nhận"; phần còn lại là file:line do scan nêu, chưa tự đọc. Tin cậy: vừa.
- Chưa đọc sâu `SourceService`/`distill.WithSources` (2 dòng cuối bảng authority) — không có bằng chứng vi phạm, cũng chưa có bằng chứng đạt.
- Không chạy `make vulncheck` (CI chạy; cần mạng). Không chạy web e2e.
- Thư mục `runtime/catalog` 42 MB sau vài lệnh trên hub 300 skill: chưa kiểm tra chính sách dọn generation cũ.

## 9. Checklist thực thi

1. `fsutil`: thêm test cho `Replace` trên Windows nếu chưa; đổi `mutation.writeFileSync`, xoá `atomic_replace_*.go`, `syncDir`; đổi `cli.writeNewFileAtomic`. Chạy `go test ./internal/mutation ./internal/delivery/cli`.
2. Tạo `internal/digest` (2 hàm, 1 test bảng); thay 14 chỗ; thêm dòng bảng AGENTS.md; thêm rule archtest cấm `crypto/sha256` ngoài `digest`, `skillruntime`, `invite`, `hubtoken`, `paging`.
3. `skillmeta.SetKey/DeleteKey` + test giữ comment/thứ tự; `skill.applyUpdateFields` sang node; `withAuditRecord`, `stampHub` dùng chung. Test hồi quy: hub.yaml có comment, chạy `skill edit`, comment còn, thứ tự giữ.
4. `skill.Locate`, `skill.ListIDs`; xoá `locateSkillDir`, `listWorkspaceSkillIDs`, `listAllSkillIDs`, `hubSkillIDs`, `ResolveWorkspace`.
5. `SkillCommitApplyOptions.ApproveContentDigest`; `ReadSkillCore` gated; bỏ exception archtest.
6. `ReviewSkill` trả audit; curator SKILL.md thêm dòng; cập nhật `known-digests` cho curator.
7. Contract: `git_failed`, envelope doc/schema/test, hai lệnh JSON.
8. Stamp cache persist + mở catalog một lần; đo lại `status`/`skill list` trên hub 300 skill (mục tiêu: < 0,2 s, < 5 000 `openat` khi không đổi).
9. `release.yml` gate + notices diff.
10. Xoá `skillmeta.WriteFiles` khỏi export; thu baseline deadcode.

Tiêu chí thành công: bảng authority AGENTS.md có thêm 4 dòng (file digest, digest form, sửa key hub.yaml, locate skill) và grep mỗi khái niệm chỉ ra **một** định nghĩa; `skill edit` không làm đổi thứ tự/comment hub.yaml; agent nạp curator skill trả lời được "ghi audit ở đâu" mà không cần `--help`; `status` trên hub 300 skill không đổi < 0,2 s.
