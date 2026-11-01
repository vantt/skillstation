# Vì sao có hai hàm tìm skill theo id, và vì sao chỉ một hàm chặn symlink

Ngày: 2026-10-31. Phạm vi: chỉ đọc; thử nghiệm trên hub tạm trong scratchpad, không chạm hub thật.
Mọi số dòng lấy từ working tree tại commit `b2ecf28`.

## Kết luận ngắn

**Trôi dạt (drift) do ngẫu nhiên, không phải quyết định bảo mật có chủ ý.** `skill.loadSkill` (29-09-2026, commit `1a31703`) ra đời cùng ngày với thiết kế "reject symlink/path escape" và chặn symlink ngay từ bản đầu. Thân hàm của `app.locateSkillDir` được viết lại bằng tay hai ngày sau trong `ReviewSkill` (`425c7ee`, 01-10-2026) với `os.ReadDir` + `os.ReadFile` vì `loadSkill` không được export; `a15a8b1` (02-10-2026) chỉ tách nó thành hàm riêng, và từ đó 28 call site trong `internal/app` bám vào nó. Không có plan, journal, hay design nào nói "bản app được phép lỏng hơn".

**Bản lỏng không phải lỗ hổng thật, nhưng là khoảng trống có hậu quả quan sát được.** Mọi cửa nhập (git source, zip, folder commit, git index) đã loại symlink; và quét canonical (`workspace.RelativeFiles`) *fail-closed* trên bất kỳ symlink nào trong `skills/`, nên catalog, `skill list/show/review/lint/activate/edit/audit-record`, MCP `skill_get`/`skill_review` đều từ chối (đã thử). Phần còn lại đi thẳng FS: `skill audit-status` đọc `hub.yaml` qua symlink ra ngoài hub và trả digest bỏ sót file bị symlink (đã chứng minh), `skill checkout` sao chép skill "0 files" thay vì từ chối. Không rò rỉ nội dung ra agent trong mọi kịch bản thử, nhưng kết quả là **không nhất quán**, và chỉ an toàn nhờ cổng catalog chứ không nhờ chính hàm.

**Khuyến nghị:** một owner `internal/skill` — export `loadSkill` (ví dụ `skill.Load(root, id)`), giữ đúng policy của nó (OpenRoot + Lstat, từ chối symlink ở collection, thư mục skill và file metadata, phát hiện id trùng), sửa hai điểm nhỏ trước khi mở rộng (bỏ qua file thường nằm lạc trong `skills/` thay vì lỗi; map `skills/` vắng mặt thành `ErrNotFound`), rồi xoá `locateSkillDir` và chuyển 28 call site sang. Không cần abstraction mới.

## 1. Lịch sử

| Mốc | Commit | Bằng chứng |
|---|---|---|
| 29-09-2026 | `1a31703` "feat(skillhub): implement curated workspace core" | `loadSkill` xuất hiện với `os.OpenRoot`, `Lstat`, "unsafe skill directory", "unsafe skill metadata path", duplicate-id ngay từ bản đầu (`git show 1a31703:internal/skill/lifecycle.go` dòng 507-547). 58/70 dòng hiện tại vẫn thuộc commit này (`git blame -L 952,1021`). |
| 29-09-2026 | review của bản V1 | `archive/plans/260928-1435-skillhub-v1/reports/code-reviewer-260929-2352-resolver-mcp-review.md:88` và `...-mutation-storage-review.md:22-36` nêu rõ yêu cầu `os.Root` + `Lstat`, "untrusted workspace (cloned repo)". Đây là nguồn của policy. |
| 01-10-2026 | `425c7ee` "feat(curation): deliver intent-first CLI and MCP curation workflows" | `ReviewSkill` viết tay `os.ReadDir(skillsDir)` + `os.ReadFile(metaFile)` dưới comment "Locate the skill directory directly from canonical storage" (`git show 425c7ee:internal/app/skill_review.go:99-124`). Lúc đó `skill` chỉ export `ReadRouting`, `ReadRationale`, `ReadEditableSkill`, `ReadEditableContent`, `IsUntouchedScaffold`, `ResolveWorkspace` (`git show 425c7ee:internal/skill/lifecycle.go` grep `^func [A-Z]`), không có cửa nào trả (collection, dir, metaBytes) — nên app tự viết. Commit message 8 gạch đầu dòng, không câu nào nói về symlink. |
| 02-10-2026 | `a15a8b1` "refactor(app): split SkillService.ReviewSkill into named steps" | Chỉ sửa `internal/app/skill_review.go` (+160/−138), cắt khối trên thành `locateSkillDir`; thân hàm giữ nguyên (`git show a15a8b1` hunk `+func locateSkillDir`). Không có journal ngày 02-10 (`ls plans/journals | grep 2026-10-02` rỗng). |
| 09-10-2026 | `8225938` | Thêm map `os.ErrNotExist → skill.ErrNotFound` khi `skills/` vắng ("Tolerate missing layout directories"). `loadSkill` không có map này (`lifecycle.go:960-963` trả lỗi thô). |
| 27-10-2026 | `513344e` | Cả hai chuyển sang `skillmeta.Read`; `loadSkill` thêm vòng `AnchorPaths` + `Lstat` cho từng anchor (`lifecycle.go:984-998`); `locateSkillDir` dùng `skillmeta.DirReader` = `os.ReadFile(root+"/"+rel)` (`internal/skillmeta/skillmeta.go:112-114`), đi theo symlink. |
| 31-10-2026 | `9fd1042` "one authority per concept" | Chỉ đổi `idPattern` → `skillmeta.ValidSkillID` ở `lifecycle.go:953`; thêm `hubSkillIDs` dùng `locateSkillDir` làm bộ lọc (`skill_audit_status.go:133-146`). Không hợp nhất hai hàm. |

Thiết kế nêu policy một chiều: `docs/design/01-system-architecture.md:553` "Normalize và containment-check mọi resource path; reject symlink/path escape"; `docs/design/07-storage-and-mutation-model.md:875` "Resolve every path against workspace root and reject escape/symlink traversal", `:652` quét canonical "Reject ... unsafe path/symlink". Không văn bản nào cho phép ngoại lệ ở tầng app. Không có test nào bảo vệ policy symlink của `loadSkill` (grep "unsafe skill directory|unsafe skill metadata path|invalid skill collection path" trong `*_test.go` = 0 kết quả) — policy sống trong code mà không có owner test.

Ba hàm liệt kê id cũng là tự phát theo nhu cầu: `listWorkspaceSkillIDs` (`8cadb79`, 30-09, `source_import.go:613`: `os.ReadDir` hai tầng + `SELECT id FROM skills`), `listAllSkillIDs` (`f71c80f`, 05-10, `upstream.go:177`: catalog trước, nuốt lỗi, fallback), `hubSkillIDs` (`9fd1042`, 31-10). Alias `skill.ResolveWorkspace = workspace.Discover` có từ `1a31703` (`lifecycle.go:1331`), 27 lần gọi trong app và 1 trong CLI.

## 2. Hành vi: call site

`loadSkill` (unexported) có 4 caller nội bộ, tất cả là đường ghi hoặc đọc để sửa: `Manager.PreviewUpdate` (`lifecycle.go:287`), `Manager.PreviewTransition` (`:449`), `ReadRouting` (`:1219`), `ReadEditableSkill` (`:1265`). App gọi ba cửa export này ở `app/skill_lifecycle.go:329,344,376,380,564`. Vì vậy **mọi mutation của skill (edit, activate/deprecate/archive, runtime block) đã đi qua bản an toàn**.

`locateSkillDir`: 28 call site, 23 file trong `internal/app` (grep `locateSkillDir(` trừ định nghĩa và test). Cột "cổng catalog" = file có gọi `catalog.`/`observeSkill`/`ReadSkillCore` trước hoặc sau; thử nghiệm §3 cho thấy cổng này từ chối hub có symlink.

| Hàm bao (file:dòng) | Tầng gọi / nguồn id | Việc làm sau đó | Cổng catalog |
|---|---|---|---|
| `SkillService.ReviewSkill` (`skill_review.go:136`) | CLI `skill review`, web `routes_read/routes_review`, MCP `skill_review` / id người dùng, request | đọc SKILL.md bằng `os.ReadFile` (`:376-379`, theo symlink), walk + băm resource (`:408-430`, bỏ qua symlink ở `:417`), hiển thị | có (`:144 catalog.SkillCanonicalIssues`) — từ chối |
| `SkillService.ContentTrustFor` (`skill_detail.go:73`) | MCP `skill_get` cho skill không active (`mcpserver/skill_tools.go:194`); `auditContentDigest` (`skill_audit_record.go:145`); `contentTrustWithChanges` (`skill_review_content.go:262`) | băm → digest phê duyệt / audit | không trực tiếp; `skill_get` bị chặn trước bởi `ReadSkillCore` |
| `skillAuditStatus` (`skill_audit_status.go:100`) và `hubSkillIDs` (`:140`) | CLI `skill audit-status`, script skill-audit / id người dùng | đọc hub.yaml, tính digest, in | **không** — đọc qua symlink (đã chứng minh) |
| `SkillAuditService.Record` (`skill_audit_record.go:79`) | CLI `skill audit-record` / id người dùng | đọc hub.yaml rồi ghi qua `PlanMutation` | gián tiếp: mutation quét canonical → từ chối |
| `SkillCheckoutService.Checkout` (`skill_checkout.go:70`) | CLI `skill checkout` | sao chép cây skill vào project (`readHubSkillTree` → `readLocalSkillFiles` bỏ qua symlink `upstream_update.go:212`) | **không** — sao chép thiếu file (đã chứng minh) |
| `loadHubSkill` (`skill_commit.go:301`) | CLI `skill commit`, submission | so sánh với folder, ghi | gián tiếp qua mutation |
| `approvalLostWarning` (`skill_lifecycle.go:197`) | `skill edit` mọi cửa | chỉ đọc provenance để cảnh báo | đã qua `loadSkill` trước đó |
| `lintRoutingEntry` (`skill_lint.go:120`) | CLI `skill lint` | đọc meta | danh sách lấy từ catalog → từ chối |
| `RuntimeStatus` (`skill_runtime_status.go:50`) | web `routes_runtime` | đọc meta, kiểm tra runtime | có |
| `GetSkillDistill` (`skill_distill.go:34`) | web `routes_distill` | đọc `.meta/distill.yaml` | không |
| `SourceService.SkillSources/PreviewAttach/PreviewDetach`, `linkedSkills`, `onboardingLinkChange` (`skill_sources.go:48`, `source_links.go:164,227,388,587`) | CLI `source`, web `routes_sources` | đọc/ghi khối sources | attach/detach ghi qua mutation |
| `loadTrackedSkills`, `GetSkillUpstream`, `validateUpdatePreconditions` (`upstream.go:215,720`, `upstream_update.go:377`) | CLI `skill outdated/upstream`, web sources, MCP `upstream_tools` | băm files_digest, so với upstream | listAllSkillIDs nuốt lỗi catalog → chạy trên FS |
| `PreviewBackfill` (`source_backfill.go:145`), `commonParentDirForSource` (`source_import.go:388`), `computeSourceRoleAndSkills` (`source.go:357`) | CLI/web source | đọc provenance | một phần |
| `queueFacts` (`review_queue.go:94`), `draftDocument` (`draft_serving.go:98`), `skillOnHub` (`feedback.go:89`), `canonicalRuntimeHints` (`metadata_lint.go:60`), `routingCheckHook.Evaluate` (`routing_suggestion_preview.go:95`) | web review queue, web drafts, MCP/CLI feedback, lint, edit hook | đọc meta | hỗn hợp |

Khác biệt hành vi ngoài symlink: `locateSkillDir` không kiểm `ValidSkillID` (một số caller tự kiểm: `skill_audit_status.go:72`, `skill_audit_record.go:54`; CLI từ chối id chứa `/` ở parse — thử `skill review ../../outside` → "not supported"); không phát hiện id trùng giữa hai collection (lấy collection đầu tiên theo thứ tự `ReadDir`); không kiểm `document["id"] == id`.

## 3. Threat model và thử nghiệm

**Hub lưu gì:** cây Git `skills/<collection>/<id>/` (SKILL.md, skill.yaml, `.meta/hub.yaml`, resource), catalog SQLite chiếu từ cây đó, `runtime/config/<id>/env` (secret, 0600). **Lộ ra đâu:** nội dung skill tới agent qua MCP `skill_get`/web/checkout; digest tới người phê duyệt.

**Symlink vào hub bằng đường nào:** mọi cửa nhập đã chặn — git source (`source/git_repository.go:301,393,713`), zip (`skill_zip.go:162`), folder commit (`skill_commit_folder.go:122`), git index (`git_index_snapshot.go:161`). Còn lại: `skillstation init <git-url>` clone hub từ remote có symlink (hub nhiều máy), `git pull` của người dùng, hoặc ai đó có quyền ghi FS. `docs/design/01:549` tuyên bố cổng trust là "cooperative-not-adversarial", không chống agent có quyền FS — nhưng hub clone từ remote là trường hợp thực tế, không cần adversary local.

**Thử nghiệm** (binary build từ `b2ecf28`, hub tạm `scratchpad/hub`, skill `demo` draft; mỗi kịch bản reset về commit gốc rồi commit symlink vào Git của hub để mô phỏng hub clone về; script `scratchpad/run.sh`, log `scratchpad/out-S1..S4.txt`):

| Kịch bản | status / doctor | list, show, review, lint, audit-record, MCP skill_get/skill_review | edit, activate (`loadSkill`) | audit-status (`locateSkillDir`, không catalog) | checkout |
|---|---|---|---|---|---|
| S1 `demo/leak.md → outside/secret.txt` | Content FAILED "unsafe symlink: skills/default/demo/leak.md" | tất cả từ chối cùng thông điệp; MCP trả `snapshot_expired` / `internal_error` | từ chối (qua quét canonical) | exit 0, digest **= baseline** (bỏ qua leak.md ở `skill_review.go:417`) | — |
| S2 `demo/SKILL.md → outside/evil-SKILL.md` | FAILED | từ chối | "unsafe canonical read path "skills/default/demo/SKILL.md"" (`lifecycle.go:911`) | exit 0, digest **đổi** (`f4d2b3…`): SKILL.md biến mất khỏi digest, không báo | "Copied demo (draft, 1 file)" — **thiếu SKILL.md** |
| S3 `skills/default/linked → outside/linkedskill` | FAILED | từ chối | "unsafe skill directory "skills/default/linked"" (`lifecycle.go:979`) | exit 0, digest `f4d2b3…` (chỉ skill.yaml); `--all` không thấy `linked` vì `entry.IsDir()` false | "Copied linked (draft, **0 files**)" |
| S4 `demo/.meta/hub.yaml → outside/hub.yaml` (verdict đổi thành block) | FAILED | từ chối | "unsafe skill metadata path" (`lifecycle.go:997`) | exit 0, in "recorded **block**" — **đọc file ngoài hub** | — |
| S5 file thường `skills/README.md` (không symlink) | ok | `skill list` ok | **`skill edit` từ chối** "invalid skill collection path "README.md"" (`lifecycle.go:967`) | ok | — |

Không kịch bản nào in `SECRET-CONTENT` hay `EVIL` ra stdout/MCP. Lý do: `workspace.RelativeFiles` (`workspace/workspace.go:854-855`) trả lỗi trên symlink đầu tiên, catalog không build được, mọi đường qua catalog fail-closed. Hai policy khác cũng tồn tại độc lập: `inventorySkillResources`/`readLocalSkillFiles` *bỏ qua* symlink (im lặng), `loadSkill`/`RelativeFiles`/`rootReader` *từ chối*. Vậy hub có ba cách xử lý symlink: từ chối, bỏ qua, đi theo (`locateSkillDir` + `inspectCanonicalEntrypoint`).

S5 là phát hiện ngược chiều: `loadSkill` chặt hơn quét canonical ở tầng collection (lỗi trên bất kỳ entry không phải thư mục), nên nếu đem nguyên xi làm owner cho 28 caller thì một `skills/README.md` hay `.DS_Store` sẽ làm `audit-status`, `checkout`, `review`… đổ hết.

## 4. Phán quyết và khuyến nghị

**Chủ ý hay ngẫu nhiên:** ngẫu nhiên. Bằng chứng: bản lỏng ra đời vì thiếu cửa export, không có văn bản biện minh, refactor sau chỉ cắt dán; design 01 §11 và 07 §20 yêu cầu một chiều "reject symlink".

**Mức độ của bản lỏng:** không phải lỗ hổng rò rỉ (cổng catalog chặn mọi đường phục vụ nội dung), là khoảng trống có hậu quả: audit digest sai và đọc metadata ngoài hub (S2-S4), checkout thiếu file (S2-S3), và quan trọng hơn là an toàn phụ thuộc vào thứ tự gọi catalog chứ không vào hàm — một caller FS-only mới (như `hubSkillIDs` vừa thêm 31-10) sẽ lại lỏng.

**Owner duy nhất:** `internal/skill`, export `loadSkill`. Đề xuất tên `skill.Load(root, id string) (Loaded, error)` với `Loaded{Collection, Dir, MetadataPath string; Metadata []byte; Document map[string]any}` (collection và dir suy từ `metadataPath` bằng `skillDirFromMetadataPath`, `lifecycle.go:1279`). Tránh tên `Locate` vì `skillmeta.Locate` đã tồn tại (`skillmeta.go:146`) với nghĩa khác. Thêm dòng vào bảng Single authority trong AGENTS.md: "Tìm skill theo id trong hub | `skill.Load`".

**Policy symlink duy nhất (giữ của `loadSkill`):** `os.OpenRoot(root)`; collection phải là thư mục thật và id hợp lệ; thư mục skill phải là thư mục thật; anchor metadata phải là file thường; từ chối (không bỏ qua) với lỗi nói rõ đường dẫn; id trùng giữa collection là lỗi; `document["id"]` phải khớp.

**Hai sửa nhỏ trong `loadSkill` trước khi mở rộng:**
1. `lifecycle.go:967`: entry trong `skills/` không phải thư mục và không phải symlink → `continue` (khớp quét canonical, tránh S5); symlink vẫn lỗi.
2. `lifecycle.go:960-963`: `skills/` vắng → `ErrNotFound` (giữ hành vi `8225938` mà các caller `errors.Is(err, skill.ErrNotFound)` như `loadHubSkill`, `skillAuditStatus` đang dựa vào).

**Caller phải chuyển:** cả 28 call site (danh sách §2) sang `skill.Load`, xoá `locateSkillDir` và `skillReviewMeta`-adjacent helper nếu không còn dùng. Riêng `inspectCanonicalEntrypoint` (`skill_review.go:376-379`) nên đọc SKILL.md qua `os.Root`/`Lstat` cùng policy (có thể gọi `skill.ReadEditableContent`, đã an toàn) — nếu không, review vẫn đọc entrypoint qua symlink trước khi cổng catalog kịp chặn. `hubSkillIDs` và `listWorkspaceSkillIDs` nên dùng `collection.Type()&ModeSymlink` cùng tiêu chí (ngoài phạm vi tối thiểu; ghi nhận).

**Thứ tự nhỏ nhất:** (a) sửa 2 điểm + export trong `internal/skill` kèm một test owner cho policy (symlink ở 3 tầng + stray file + duplicate id, hiện chưa có); (b) thay 28 call site cơ học; (c) `make lint`, `make deadcode`, `go test ./internal/app/... ./internal/skill/...`. Không đụng `ResolveWorkspace` và ba lister trong cùng commit (M9 của audit trước là việc riêng).

**Rủi ro khi chuyển:** caller nhận lỗi mới ở hub bẩn (id trùng, metadata id lệch) thay vì lặng lẽ lấy bản đầu — đây là hành vi mong muốn, nhưng cần map sang ERROR/WHY/FIX qua `app.ClassifyError` để CLI không in lỗi thô.

## 5. Giả định và điều chưa xác minh

- Chưa chạy web routes (`routes_runtime`, `routes_distill`, `routes_sources`, drafts, review queue) trên hub có symlink; suy từ code rằng các route không qua catalog sẽ đọc được như `audit-status`. Độ tin: trung bình.
- Dòng gây "unsafe symlink" trong `skill review` được quy cho `catalog.SkillCanonicalIssues` (`skill_review.go:144`) theo thông điệp lỗi trùng với `workspace.go:855`; chưa trace bằng debugger. Độ tin: cao.
- Giả định hub nhiều máy (`init <git-url>`, `git pull`) là đường thực tế để symlink vào hub; chưa kiểm tra `init <git-url>` có quét symlink sau clone hay không. Nếu có quét, rủi ro giảm thêm một bậc nhưng kết luận "drift, cần một owner" không đổi.
- Không có tiến trình nào còn chạy sau thử nghiệm (`pgrep -af scratchpad/bin/skillstation` chỉ thấy shell của phiên này).
