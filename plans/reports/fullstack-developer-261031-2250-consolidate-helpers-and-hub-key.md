# Gộp helper ghi atomic, digest và sửa key hub.yaml

Trạng thái: xong, chưa commit. `make fmt-check vet lint deadcode` sạch, `make test` xanh (một lần chạy đầu hỏng vì `/tmp` báo "disk quota exceeded"; chạy lại riêng gói và cả bộ đều xanh), `GOOS=windows go vet ./...` sạch, `SKILLSTATION_REQUIRE_TAGS=1` cho systemskills/hostintegration xanh.

## Bước 1a: ghi atomic, owner là `internal/fsutil`

- Đã xoá `internal/mutation/atomic_replace_unix.go` và `atomic_replace_windows.go`. `mutation.writeFileSync` vẫn giữ kiểm tra an toàn (parent an toàn, đích không phải symlink hay file đặc biệt), rồi gọi `fsutil.WriteFileAtomic(target, contents, 0o600)`. Hàm `mutation.syncDir` đã bị xoá; mọi chỗ gọi giờ dùng `fsutil.SyncDir`.
- `cli/evaluation.go`: đã xoá `publishFileWithoutOverwrite` và phần ghi file tạm của `writeNewFileAtomic`. Bản này nghiêm hơn owner vì nó **không ghi đè**: nó publish bằng `os.Link`, nên vẫn từ chối được một file xuất hiện sau lúc kiểm tra. Tôi chuyển tính chất đó vào fsutil thành `fsutil.WriteNewFileAtomic` (trả lỗi khớp `fs.ErrExist`). Phần ghi file tạm dùng chung một hàm `writeTemporary` với `WriteFileAtomic`. CLI vẫn giữ thông báo "refusing to overwrite". Có thêm một test fsutil cho hàm này.
- Có hai khác biệt nhỏ, cả hai đều chấp nhận được:
  - Bản của mutation báo lỗi khi `Chmod` thất bại trên Windows, còn fsutil cố ý bỏ qua (trên Windows perm chỉ là cờ read-only, với 0600 thì không ảnh hưởng an toàn). Tôi giữ hành vi của fsutil.
  - Tên file tạm đổi từ `.skillstation-*` thành `.<tên>.tmp-*`. Không có mã nào dò theo tên cũ.

## Bước 1b: digest, owner là package lá mới `internal/digest`

- API: `digest.Bytes([]byte) string` trả `sha256:<64 hex thường>`, và `digest.Valid(string) bool`.
- Lý do không đặt vào `skillruntime`: package đó import `workspace`, `skillmeta`, `fsutil` và mang theo cả runtime/http, trong khi `source`, `systemskills`, `canonical`, `mutation` nằm thấp hơn hoặc ngang nó. Package lá không import gì trong module nên mọi tầng đều import được, và archtest vẫn qua.
- Các bản đã xoá và chuyển sang owner:
  - Hàm có tên: `source.Digest`, `mutation.digest`, `canonical.contentDigest`, `systemskills.FileDigest`, `app.digestOf`.
  - Validator: `source.validDigest`, `catalog.validDigest`, `evaluation.validDigest`, `canonical.isValidDigest`, `distill.digestPattern`, `app.auditDigestPattern`, `skillruntime.fileDigestPattern`, `cli.contentDigestPattern`. `mutation.validOptionalDigest` giờ chỉ còn `value == "" || digest.Valid(value)`.
  - Tính inline trong: app (skill_review x2, skill_add x3, skill_discovery, distribution, routing_eval, skill_snapshot, skill_audit_record), skill (lifecycle x4, resource), catalog (input, servable), distill, evaluation, paging, resolver, hostintegration, systemskills/embed và migration v3/v5/v6.
  - Hàm `hostintegration.digest(content, exists)` đổi tên thành `preimageDigest` vì trùng tên package. Nó vẫn giữ logic riêng cho trường hợp file không tồn tại.
- Không có giá trị digest nào đổi. Mọi validator cũ đều đã ép chữ thường; bản của mutation cũng vậy, khác với nhận định trong audit. Test `internal/digest` ghim giá trị của "abc" và dạng chuỗi. Proposal ID của `skill.appliedProposal` giờ lấy 20 ký tự hex đầu của digest, đúng bằng `hex(hash[:10])` như trước.
- Migration v3/v5/v6 không bị đóng băng: không có ghi chú nào nói vậy, và v7 đã import `source`/`mutation` sẵn. Vì vậy tôi đã chuyển chúng sang owner.
- Phần còn lại, có chủ ý không gộp:
  - Các hash dạng stream (`sha256.New()` rồi ghi nhiều lần), như proposal/request digest và inventory aggregate. Đây là khái niệm khác, không phải digest của một khối byte.
  - Các hash hex không có tiền tố hoặc bị cắt ngắn, ví dụ id hoặc tên cache.
  - `delivery/web/routes_read.go:26-30` (`workspace_id`), vì web không có trong danh sách mở rộng.
  - Các phép tính độc lập trong test, giữ lại làm oracle.
- Đã cập nhật AGENTS.md: thêm dòng "Digest of bytes ... | `internal/digest`", và sửa dòng atomic write thành (`WriteFileAtomic`, `WriteNewFileAtomic`, `Replace`, `SyncDir`).

## Bước 2a: `skillmeta.SetKey`

- `SetKey(contents []byte, key string, value *yaml.Node) ([]byte, error)` đặt một key cấp cao nhất, hoặc xoá key đó khi value là nil, bằng cách sửa trên cây `yaml.Node`:
  - Key đã có thì giữ nguyên vị trí và comment.
  - Key mới được thêm vào cuối.
  - Comment ở mức document được giữ.
  - Hàm không ghi file.
- Đã chuyển ba nơi ghi sang hàm này:
  - `mutation.stampHub`: đã xoá `mappingValue`, `removeMappingKey`, `setMappingString`. Ngày tháng giờ được cập nhật tại chỗ thay vì bị gỡ ra rồi nối xuống cuối.
  - `app.withAuditRecord`: đã xoá vòng lặp node viết tay.
  - `skill.PreviewUpdate` và `PreviewTransition`: hàm mới `editedMetadata` so map trước và sau khi sửa, rồi chỉ gọi `SetKey` cho những key thay đổi. Ở layout hub.yaml không còn xoá `created_at`/`updated_at` vì mutation đóng dấu ngày tại chỗ. Layout cũ vẫn xoá như trước, gói trong `dropCarriedFields`. `PreviewCreate` vẫn dùng `yaml.Marshal` vì lúc đó chưa có file.
- Regression test `TestUpdateKeepsHubKeyOrderAndComments` (internal/skill): trên mã cũ nó hỏng đúng lý do (thứ tự key bị đổi thành `quality, status`, comment mất), và qua sau khi sửa. Có thêm một test bảng nhỏ cho `SetKey` (thay thế tại chỗ, xoá, thêm vào cuối).
- Dòng "Skill metadata files" trong AGENTS.md nay ghi thêm `SetKey`.

## Bước 2b: dễ tìm cách ghi audit

- App:
  - Thêm `app.AuditState` (content_digest, recorded, since_last_audit). `SkillAuditStatus` nhúng nó, nên JSON của audit-status không đổi.
  - Hàm `auditStateOf` được dùng chung cho `SkillAuditService.Status` và `ReviewSkill`.
  - `ReviewSkill` tính audit từ `contentTrust.ContentDigest` đã có sẵn, nên không tính lại lần thứ hai.
  - `SkillReviewResult.Audit` (`json:"audit"`) tự xuất hiện qua MCP `skill_review` và route review của web vì chúng dùng chung view model. Không làm UI web mới.
- CLI:
  - `skill review` và `skill show` có thêm khối "Audit". Khối này ghi "ok, fresh (...)", hoặc "ok, stale (...)" kèm digest đã audit và digest hiện tại, hoặc "not audited", và in lệnh `audit.mjs --record <id> --verdict ok|review|block`.
  - `skill show` gọi `SkillAuditService.Status`. Nếu lỗi thì in "unavailable: ..." chứ không làm hỏng lệnh show.
  - JSON của `show` không đổi (đó là contract của ReadSkill).
  - Test bảng `TestSkillShowReportsTheRecordedAudit` có ba bước: chưa audit, vừa ghi, nội dung đổi.
- `audit-record`: khi thiếu tham số, WHY giờ nói rằng `--rules-digest` chỉ script skill-audit tính được, và FIX chỉ sang `node .agents/skills/skill-audit/scripts/audit.mjs --record <id> --verdict ...` (đã kiểm tra flag trong `audit.mjs:316-317`).
- Curator SKILL.md: thêm một dòng vào bảng ý định, nói cách đọc (`skill audit-status <id>`) và cách ghi hoặc làm mới (`audit.mjs --record`). Dòng đó cũng nói rằng script tính `--rules-digest`, hub tính content digest, và không được tự bịa digest. Đã chép sang `internal/systemskills/curator/SKILL.md`.
- Luật release: bản curator đã phát hành trong v0.11.0 và v0.12.0 (`sha256:87f6a633…11fa`) chưa có trong `known-digests.go`, nên test theo tag báo hỏng. Tôi đã thêm digest đó. `known-wrappers.go` không cần đổi vì test của nó vẫn qua.

## File ngoài danh sách được phép

- `internal/digest/` (package mới, nằm trong phạm vi brief cho phép tạo package lá).
- Bị ép compile khi xoá `source.Digest`/`FileDigest`: `internal/distill/types_test.go`, `internal/delivery/mcpserver/{submission_http_test,upstream_tools_test}.go`, `internal/delivery/web/{fixtures_test,routes_submissions_test}.go`, `internal/delivery/cli/skill_upstream_test.go`, `internal/migration/v7.go`.
- Thuộc phạm vi mở rộng do coordinator cho: `internal/catalog`, `internal/evaluation`, `internal/distill`, `internal/delivery/paging`, `internal/resolver`, `internal/migration/v3,v5,v6`. Ở `distill/types.go:330` có thêm phép De Morgan vì lint chặn dòng đã đổi.
- Hai change detector phải cập nhật vì review có thêm trường audit: `internal/delivery/mcpserver/tools_list_digest_test.go` (ghim lại all/curation) và `internal/delivery/web/testdata/golden/skill-review*.json`.

## Những gì không làm

- Không động vào hai hàm tìm skill theo id hay hành vi symlink của chúng.
- Không làm contract drift, cache hash, release workflow hay ngoại lệ archtest.
- Không thêm kiểu cho `web/src/api/types.ts`.
- Không đổi JSON của `skill show`.
- Không sửa `.agents/skills/skill-curator/SKILL.md` hay wrapper `.claude/...` (bản cài đặt trong repo; trước thay đổi này đã khác nguồn).

## Câu hỏi còn mở

- Comment ở đầu `hub.yaml` gắn với `schema_version` vẫn mất khi `Merge`/`Split` dựng lại `schema_version`. Đây là giới hạn có sẵn của Merge/Split, nằm ngoài SetKey, và chưa sửa.
- Các khối lồng nhau bị thay cả khối (ví dụ `quality` khi duyệt nội dung) vẫn sắp xếp key bên trong và mất comment bên trong khối đó. Thứ tự và comment ở cấp cao nhất thì được giữ.
