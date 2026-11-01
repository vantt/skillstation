# Lệch contract, mã lỗi và hai phần còn sót ở delivery

Trạng thái: xong, chưa commit. `make check` (một lệnh) xanh ngay lần đầu, không cần `GOTMPDIR`.
`GOOS=windows go vet ./...` sạch. Sau mỗi bước đã chạy `make lint` (0 issues) và `make deadcode`
(không có mục mới; đã xoá dòng `IsSystemSkillID` khỏi baseline vì hàm giờ đã có caller).

## 1. Mã lỗi

- `ErrorGitFailed` (`git_failed`) chuyển từ `workspace_init.go` vào khối hằng của `internal/app/errors.go`.
- Đã thêm `git_failed` vào bảng của `docs/contracts/error-codes.md`, vào enum của
  `schemas/error-envelope.schema.json`, vào `statusByCode` của web (502, cùng nhóm với
  `source_unavailable`) và vào bảng HTTP trong `web-api.md`.
- CLI: `writeInitError` giờ in qua `termui.Printer.Error`, đúng 3 nhãn ERROR/WHY/FIX; thông báo
  nhiều dòng của Git nằm trong WHY. Exit code vẫn là 1 (lỗi runtime), không phải 2. Các lệnh khác trả
  `git_failed` vốn đã đi qua `p.Error`. MCP không có đường nào trả mã này.
- Năm mã không có code nào sinh ra: `ambiguous_locator`, `partial_distill_failure`,
  `resource_read_failed`, `run_interrupted`, `source_changed`.
  - Quyết định: giữ cả năm, liệt kê trong mục mới "Reserved codes" của `error-codes.md`.
  - Lý do: xoá chúng sẽ thu hẹp enum công khai, tức đổi hình dạng contract. Đoạn đầu
    `error-codes.md` nói registry không khẳng định lệnh đó tồn tại.
    `docs/use-cases/04-…:663` cũng đã coi chúng là mã "chưa phát ra". Dòng đó từng liệt kê sai
    `validation_failed` và `resource_limits_exceeded` (hai mã này giờ đã được sinh ra), nên tôi
    sửa dòng đó để trỏ về danh sách mới.
- Các mã chỉ dùng cho network listener (`skill_not_found`, `submission_busy`,
  `submission_queue_full`, `request_timeout`, `method_not_allowed`) đã được ghi riêng trong mục
  "Submissions on the network listener". Chúng không phải mã của `app.Error`, nên tôi giữ nguyên.
- Owner test `internal/app/errors_contract_test.go` (`TestErrorCodeRegistryMatchesContract`)
  parse mã nguồn không phải test của `internal/app`. Test hỏng khi gặp một trong các trường hợp sau:
  - Một hằng `ErrorCode` khai báo ngoài `errors.go`.
  - Viết `ErrorCode("x")` hoặc `Error{Code: "x"}` bằng chuỗi thay vì dùng hằng.
  - Registry, bảng trong doc và enum của schema không khớp nhau.
  - Một mã được sinh ra nhưng lại nằm trong danh sách Reserved, hoặc ngược lại.

  Đã kiểm tra: bỏ `git_failed` khỏi schema thì test hỏng đúng lý do. Test cũ
  `TestErrorEnvelopeSchemaAcceptsNewErrorCodes` (danh sách 15 mã viết tay) đã bị xoá vì test mới bao
  trùm nó. Test web không còn ghim con số 33.

## 2. Result envelope

- Cái đúng là code. `app.Result` luôn có `schema_version` ("1") và `details`. `details` hiện chỉ
  `init` dùng (`details.init`).
- Doc sai ở chỗ nói `schema_version` không thuộc envelope chung. Schema vốn đã bắt buộc
  `schema_version` ngay từ commit đầu.
- Đã sửa `result-envelope.md`:
  - Thêm `schema_version` vào ví dụ JSON và vào danh sách Public. Nói rõ nó chỉ đánh version cho
    envelope này; `schema_version` của resolver thuộc protocol.
  - Thêm `details` vào nhóm Experimental.
  - Thêm câu: một lệnh có thể thêm field ở cấp cao nhất, và consumer bỏ qua field nó không biết.
  - Ghi hai ngoại lệ không dùng envelope: `doctor --json` in `{role, sections, fixed, not_fixed}`,
    `telemetry cases --json` in một mảng. Tôi chỉ ghi lại, không bọc, vì bọc sẽ đổi hình dạng output
    và `status_doctor.go` nằm ngoài phạm vi được sửa.
- Đã sửa `result-envelope.schema.json`:
  - Thêm `details` (experimental).
  - Đổi `additionalProperties` ở cấp cao nhất từ false sang true. Các `$defs` lồng bên trong vẫn
    đóng.
  - Bằng chứng: doc đã viết "A checkout result adds `skill_id`, …", "These fields are additive",
    "An unclassified field is internal", và output thật của checkout/commit có các field đó. Như vậy
    schema đóng là mô tả sai. Output không đổi gì.
- Test `internal/delivery/cli/result_envelope_schema_test.go` validate output `--json` thật bằng
  jsonschema-go, có resolve `$ref` sang error envelope:
  - `init --json`: thành công, có `details.init`.
  - `skill show missing-skill --json`: `not_found`, exit 2.

  Chạy với schema cũ thì test hỏng với lỗi "unexpected additional properties [details]".

## 3. `IsSystemSkillID`

- Luật có trong tài liệu:
  - `docs/design/03-resolver-design.md:232`: system skill dùng "reserved identity".
  - Commit `05d75b1`: "both ids stay reserved".
- Trước khi sửa, luật chỉ được áp ở `canonical` `checkSkillID` và chỉ cho hai id curator. Tái hiện
  trên hub tạm: `skill create distill-lab` rồi `skill commit --yes` được commit bình thường.
- Coordinator cho phép sửa `canonical`. Tôi đã thử thay bằng `IsSystemSkillID`, rồi bỏ lại: `status`
  của một hub đã có skill `distill-lab` chuyển thành "Workspace invalid", và mọi lần ghi khác (commit
  skill khác) đều bị chặn. `canonical/skill.go` đã được trả về đúng như trước.
- Cách áp đã chọn: `mutation.refuseReservedSkillIDs` gọi trong `validateVirtualSnapshot`. Đây là
  phễu chung của mọi lần ghi hub (create, add, import, commit folder mới, submission). Nó từ chối một
  thay đổi thêm `skills/<c>/<id>/…` khi `id` là system id và hub chưa có thư mục id đó ở collection
  nào. Lỗi là `ValidationError`, nên ra `invalid_request` với ERROR/WHY/FIX. Ví dụ trên hub tạm:
  `WHY: … skill id "skill-audit" is reserved for the bundled system skill`.
- Đã kiểm tra trên hub tạm: skill `distill-lab` có sẵn vẫn `skill edit` được, `status` vẫn hợp lệ,
  commit một skill khác vẫn được, còn `skill-audit` mới thì bị từ chối.
- Grep repo, testdata, các system skill đóng gói và hub thật `/home/vantt/projects/skillstation-hub`:
  không có hub skill nào dưới `skills/*/` dùng system id. Chỉ có các bản cài system skill trong
  `.agents/`/`.claude/`, và hai thư mục `system-curator` lồng bên trong skill `distill-repo` (không
  phải id của hub skill).
- Test: `TestNewSkillCannotTakeASystemSkillID` trong `internal/mutation/skill_metadata_test.go`
  gồm hai phần: skill mới bị từ chối, và skill đã có sẵn vẫn sửa được. Test hỏng khi tắt check.
- AGENTS.md có thêm dòng "Skill ids reserved for system skills", ghi lý do `canonical` vẫn giữ check
  cho hai id curator. Không có contract doc nào liệt kê id reserved, nên không phải sửa doc.
- Khác biệt hành vi: commit hoặc add một skill mới tên `distill-lab` hay `skill-audit` giờ bị từ chối.
  `skill create <system-id>` phía project vẫn tạo folder như trước; chỉ lúc ghi vào hub mới bị chặn.

## 4. Phần còn sót ở delivery

- `web/routes_submissions.go` `dropAppliedSubmission` là logic nghiệp vụ: chỉ bỏ submission khỏi
  queue khi proposal đã được làm từ chính submission đó. Nó còn khác CLI: web chỉ ghi `slog`, còn app
  (`ApplySubmission`) trả warning `submission_not_dropped`.
  - Đã chuyển thành `app.DropAppliedSubmission(root, id, proposalID) []Warning`. Nó dùng chung
    `dropSubmission` với `ApplySubmission`.
  - `SubmissionBoundToProposal` được unexport.
  - Web nối warning vào result. Đã cập nhật `web-api.md`.
- `mcpserver/distribution.go` `bundleHTTPError`: để nguyên. Đây là ánh xạ transport (HTTP status,
  body `{code, message}`, log correlation), giống `web/errors.go`. Nó có ngoại lệ được ghi trong doc:
  trả `skill_not_found` cho proxy đã phát hành, và gộp not_found, not_servable, snapshot_expired thành
  404. `ClassifyError` không phân loại `ErrResourceContentUnavailable`. Nếu đi qua `ClassifyError` thì
  phải thêm phân loại đó, và việc này đổi mã trên CLI, web và tool MCP, đồng thời đổi thứ tự ưu tiên
  với lỗi bọc cả `ErrResourceContentUnavailable` lẫn `ErrSnapshotExpired` (`distribution.go:571`).
  Tức là đổi hành vi, không chỉ là bỏ bản trùng.

## File ngoài vùng dự kiến

- `internal/mutation/filesystem.go`, `skill_metadata.go`, `skill_metadata_test.go`: để áp luật
  reserved id mà không làm hỏng hub đang có.
- `internal/systemskills/bundles.go`: chỉ sửa comment.
- `scripts/deadcode-baseline.txt`: bớt 1 dòng.
- `docs/use-cases/04-webui-user-flows-and-screen-specs.md`: 1 dòng.
- `schemas/embed_test.go`: xoá test bị test mới bao trùm.
- `internal/delivery/web/errors_test.go`: bỏ con số ghim cứng.
- Không chạm: `web/`, `.github/`, `internal/selfupdate`, `snapshot_cache.go`. `internal/canonical`
  không đổi gì so với trước.

## Câu hỏi mở

- Có nên bọc `doctor --json` và `telemetry cases --json` vào envelope không? Việc này đổi hình dạng
  output, nên hiện tôi chỉ ghi chúng là ngoại lệ.
- Có giữ năm mã reserved lâu dài không, hay xoá khỏi enum ở major sau?
- Có nên cho `ClassifyError` biết `ErrResourceContentUnavailable`, để MCP/web/CLI trả
  `resource_content_unavailable` thay vì `internal_error`, rồi mới gộp phần ánh xạ sentinel của
  mcpserver không?
