# Một owner cho "tìm skill theo id" và các phần single-authority còn lại

Trạng thái: xong, chưa commit. `make check` (một lệnh) xanh ngay lần đầu, không cần đặt
`GOTMPDIR` (không gặp "disk quota exceeded"). `GOOS=windows go vet ./...` sạch. Sau mỗi bước
đã chạy `make lint` (0 issues) và `make deadcode` (baseline giảm từ 21 xuống 19).

## 1. `skill.Load` là owner duy nhất

- File mới `internal/skill/load.go`:
  - `Load(root, id) (Loaded, error)`. `Loaded` gồm `Collection`, `Dir`, `MetadataPath` và
    `Metadata` (đã merge).
  - `IDs(root)` trả danh sách id.
  - `ReadEntrypoint(root, dir)` đọc SKILL.md, trả nil khi không có file.
  - Sentinel `ErrInvalidSkillTree`.
- Policy giữ nguyên như cũ: `os.OpenRoot` + `Lstat`, từ chối symlink ở collection, ở thư mục
  skill và ở file metadata, và báo lỗi khi một id nằm ở hai collection. Đã sửa hai lỗi:
  - File thường nằm ngay trong `skills/` giờ được bỏ qua.
  - Khi không có `skills/` thì trả `ErrNotFound`.
- `Load` không parse YAML. Nhờ vậy `skill review` vẫn báo được một skill có metadata hỏng.
  `loadSkill` (đường mutation) gọi `Load`, rồi mới decode và kiểm tra `id`.
  `rootReader`/`readOptional` bọc lỗi bằng `ErrInvalidSkillTree`.
- Đã chuyển toàn bộ 28 call site của `locateSkillDir` (kể cả hai test) sang `skill.Load` và
  xoá `locateSkillDir`.
- `inspectCanonicalEntrypoint` đọc qua `skill.ReadEntrypoint` và giờ trả lỗi. `skill lint` và
  routing preview trước đây gọi `os.ReadFile(SKILL.md)`, giờ cũng đi qua `skill.ReadEntrypoint`.
- Ngoài phạm vi tối thiểu nhưng cần để "từ chối thay vì đọc xuyên":
  - `inventorySkillResources` và `readLocalSkillFiles` trước đây bỏ qua symlink trong im lặng.
    Giờ chúng từ chối với `ErrInvalidSkillTree`. Nếu không làm vậy, digest audit vẫn thiếu file
    và checkout vẫn sao chép thiếu (kịch bản S1/S2).
  - Helper `skillFolderFiles` gộp cặp "entrypoint + inventory" mà bốn nơi đang lặp lại.
  - `workingTreeSkillFilesDigest` trả `""` khi bị từ chối, nên trạng thái hiện "modified", không
    bao giờ hiện "clean".
- Lister:
  - `hubSkillIDs` và `listAllSkillIDs` thực chất là một: mọi caller đều `Load` từng id sau đó.
    Đã xoá cả hai và thay bằng `skill.IDs`, nên không còn phụ thuộc catalog.
  - `listWorkspaceSkillIDs` không phải bản trùng. Nó trả các id bị chiếm, gồm cả folder không có
    metadata và id trong catalog. Đã đổi tên thành `takenSkillIDs` và ghi comment giải thích.
- Đã xoá alias `skill.ResolveWorkspace`. 28 chỗ dùng (app và `cli/skill.go`) giờ gọi
  `workspace.Discover`.
- Thông báo lỗi:
  - `ClassifyError` map `ErrInvalidSkillTree` sang `validation_failed`, mã này đã có trong
    contract. WHY là đường dẫn cụ thể.
  - `SkillCommandError` giữ nguyên bản render đó. Nếu không, CLI sẽ in FIX chung chung.
- Test:
  - Owner test `TestLoadRefusesWhatItWouldReadThrough` gồm 6 hàng: happy path kèm file lạc,
    thiếu `skills/`, symlink thư mục skill, symlink hub.yaml, symlink SKILL.md, id trùng. Bảng
    kiểm tra cả `IDs`.
  - Thêm một bước vào test checkout có sẵn: thư mục skill là symlink thì bị từ chối và không ghi
    gì vào project.
  - Thêm một bước vào `TestSkillShowReportsTheRecordedAudit`: hub.yaml là symlink sang verdict
    `block` thì `audit-status` trả exit 2 và không in `block`. Trên mã cũ cả hai bước này đều
    hỏng, vì mã cũ đọc xuyên symlink.
- Kiểm tra trên hub tạm `scratchpad/hub` bằng script
  `/tmp/claude-1000/-home-vantt-projects-skillstation/c8d79aa5-7e84-4d4a-89e5-f4de6a2e2f8a/scratchpad/verify.sh`:
  - S1–S4 (symlink file, SKILL.md, thư mục skill, hub.yaml): `audit-status` và `checkout` đều
    trả exit 2 với "A skill folder in the hub cannot be read safely."
  - S0 (baseline) và S5 (`skills/README.md`) vẫn chạy bình thường.
  - Hub đã được reset về commit gốc. Không còn tiến trình nào chạy.

## 2. Kiểm tra digest khi approve nằm trong app

- `SkillCommitApplyOptions.Approve bool` được thay bằng `ApproveContentDigest string`.
  `approvalRefusal` trong `skill_commit_apply.go` xử lý ba trường hợp:
  - Thiếu digest: lỗi `content_review_required` như trước.
  - Digest lệch: `invalid_request`, giữ nguyên WHY/FIX mà CLI đang in.
  - Skill first-party: bỏ qua digest.
- Các nơi gọi:
  - CLI `--yes` chỉ truyền `flags.approveContent`.
  - Câu hỏi tương tác của commit và submission truyền `preview.Trust.ContentDigest`, tức digest
    người dùng vừa thấy.
- Flag và output CLI không đổi. Test CLI hiện có ("does not match the new content") vẫn qua.
- Thêm một bước vào test app có sẵn: digest sai thì bị từ chối và HEAD không đổi.
- Không cần sửa docs/contracts.

## 3. Các bản digest còn sót

- Đã chuyển sang `digest.Bytes`: `web/routes_read.go` (`workspace_id`) và
  `hostintegration/preview.go` `contentDescriptor`.
- `app/distribution.go`: digest manifest của skill đóng gói đang tự băm JSON, giờ dùng
  `skillruntime.ManifestDigest`. Bytes giống hệt bản cũ, các test digest và golden vẫn qua.
- Kiểm tra dạng digest:
  - `skillruntime.checkBundleMetadata` dùng `digest.Valid`, tương đương điều kiện cũ.
  - `app/submissions.go` `BaseVersion` trước chỉ kiểm tra tiền tố `sha256:`. Giờ dùng
    `digest.Valid` nên chặt hơn ở biên nhận bundle từ máy khác.
- Giữ nguyên có chủ ý:
  - Hash dạng stream: mutation proposal/request, `catalog/skill_issues`, `remote_cache` khi ghi
    file.
  - Hex cắt ngắn hoặc không có tiền tố: id, tên cache, `git_index_snapshot`, `Spec.Fingerprint`,
    preview `[:8]`.
  - `skillruntime/trust.go`: nằm bên trong owner `ContentDigest`.
  - `selfupdate`: không được phép chạm.
  - `cmd/distill-scorecard` và các oracle trong test.

## 4. archtest, gating của `skill_get`, export chỉ test dùng

- Hai exception trong archtest thực ra vẫn khớp import thật (test tự báo exception thừa), nhưng
  chỉ để lấy sentinel. Đã làm như sau:
  - File mới `internal/app/sentinels.go` đặt tên lại cho `ErrWorkspaceBusy`,
    `ErrCatalogUnavailable`, `ErrSkillNotServable` và `ServingFallback`.
  - `cli/status_doctor.go` và bốn file mcpserver dùng các tên đó.
  - Đã xoá cả hai exception. Delivery giờ không còn import `catalog` hay `mutation`.
- Gating của `skill_get` là logic nghiệp vụ: skill không active, không phải curator, bên thứ ba
  chưa duyệt thì không được trả nội dung. Đã chuyển nó thành
  `app.SkillService.InactiveSkillReviewGate`. MCP chỉ còn áp `Local` và xoá `Content`, giống
  trước.
  - `omitAwaitingReview` trong `skills_extension.go` để nguyên: nó lọc theo trạng thái
    remote/snapshot riêng của adapter MCP.
- `skillmeta.WriteFiles` được chuyển thành `testenv.WriteSkillMetadata`, gồm 6 file test. Đã xoá
  dòng tương ứng trong baseline deadcode.
- `transcripts.EncodeProjectDir` là wrapper export chỉ để test dùng. Đã xoá; hai test giờ tự
  dựng tên thư mục theo quy tắc của Claude Code, đóng vai oracle độc lập.

## 5. AGENTS.md

- Thêm các dòng:
  - "Finding a skill in the hub by id, and listing the ids" (`skill.Load`, `skill.IDs`,
    `ReadEntrypoint`, `ErrInvalidSkillTree`, cùng lý do `takenSkillIDs` rộng hơn).
  - "Approving a third-party skill's content".
  - "Withholding unreviewed third-party content from agents".
- Dòng "Import rules" giờ nhắc `internal/app/sentinels.go`.

## Khác biệt hành vi

- Hub có symlink trong `skills/` giờ bị từ chối với `validation_failed` (web trả 400) ở mọi
  lệnh đọc FS: audit-status, checkout, review, runtime status, distill, sources, upstream,
  backfill, review queue, draft serving.
  - Các vòng lặp nuốt lỗi sẽ bỏ qua skill đó: source role, linked skills, backfill.
  - Trước đây những đường này hoặc đọc xuyên symlink, hoặc bỏ qua nó trong im lặng.
- Một id nằm ở hai collection giờ là lỗi. Trước đây hệ thống lấy bản đầu tiên tìm thấy.
- Id không hợp lệ đưa vào `Load` trả "skill id must be...", không còn là `ErrNotFound`. Các
  caller người dùng gọi tới đều đã kiểm tra id từ trước.
- Metadata split hỏng trước đây cho ra `ErrNotFound`, giờ trả lỗi parse thật.
- `upstream` và `source` liệt kê id từ FS, không đọc catalog nữa.
- Nội dung WHY của lỗi digest lệch khi commit không đổi. Lỗi đó giờ đi qua result của Apply
  thay vì do CLI tự in, exit code vẫn là 2.

## File ngoài vùng dự kiến

- `internal/app/error_classify.go` và `skill_error_fix.go`: phân loại và render lỗi. Không chạm
  `errors.go`.
- `internal/testenv/skill_metadata.go` (mới) và `scripts/deadcode-baseline.txt` (bớt 2 dòng).
- Code: `internal/transcripts/claude_code.go`, `internal/hostintegration/preview.go`,
  `internal/skillruntime/remote_cache.go`.
- Test: `internal/catalog/catalog_test.go` và `internal/canonical/canonical_test.go`, chỉ đổi
  helper.

Không chạm: `web/`, `.github/`, `internal/selfupdate`, cấu hình release, `errors.go`,
`workspace_init.go`, `docs/contracts`, `snapshot_cache.go`.

## Không làm và lý do

- `cli/workspace_resolve.go:164` chấp nhận folder có `.skillstation/` mà không có marker. Đây
  không phải bản sao của `Discover`: đó là luật nới lỏng riêng cho hub mất marker, và bảng đã ghi
  file này. Giữ nguyên.
- Các mục còn lại trong baseline deadcode chưa xử lý:
  - Seam dùng chéo package: `WithProbeClient`, `PinnedOpens`, `NewWithWidth`.
  - Danh sách của owner được test nhiều package đọc: `RequiredDirectories`, `MachineFiles`.
  - Chỉ test cùng package dùng: `mutation.RollBack`, các accessor trong `schemas`. Đổi sang
    unexport không làm baseline nhỏ đi.
  - Đây là nợ đã ghi sẵn trong baseline. Gỡ chúng cần thiết kế lại test, nằm ngoài phạm vi
    single-authority.
- `systemskills.IsSystemSkillID` không có caller nào. Comment của nó nói "A hub skill may not
  take such an id", nhưng tôi không tìm thấy chỗ nào áp luật này. Tôi không xoá, vì xoá sẽ che
  mất một luật có thể đang thiếu. Cần người quyết định.
- `skill add` vẫn dùng `Approve bool`. Bản approval của nó đã gắn với proposal pin lúc preview,
  và CLI không có `--approve-content` cho add (test hiện có buộc từ chối flag này).
- Phần còn lại của M11 trong audit chưa làm, vì brief chỉ nêu `skill_get`:
  - Map sentinel lặp ở `mcpserver/distribution.go:165`.
  - Việc dọn queue ở `web/routes_submissions.go`.

## Câu hỏi mở

- Có muốn áp luật "hub skill không được lấy id của system skill" bằng `IsSystemSkillID`, hay xoá
  hàm này?
