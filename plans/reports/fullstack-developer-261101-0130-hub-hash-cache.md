# Hub hash cache: bỏ chi phí băm lại cả hub ở mỗi tiến trình CLI

Ngày: 2026-11-01. Phạm vi: chi phí đọc/băm lặp lại của `status`, `skill list`, `skill show`.

## Nguyên nhân gốc (đo bằng memprofile đếm số lần `ReadCanonicalFileIn`, hub tổng hợp 300 skill / 1 517 file)

| Lệnh | Đọc mỗi file | Nguồn |
|---|---|---|
| `status` | 4 lần | `canonical.Validate` chạy 2 lần (hub check `health_hub.go:59` và `GetCurationHome` `curation_home.go:132`), `ScanCached` đọc 2 lần (đọc kép chống ghi đồng thời) vì cache stat chỉ ở RAM |
| `skill list` | 2 lần | `ScanCached` đọc kép; catalog còn được mở 2 lần (`ListSkills` rồi `ListSkillCatalog`), lần hai chỉ tốn một lượt stat vì cache RAM |
| `skill show` | 2 lần | `ScanCached` đọc kép |

Mỗi lần đọc qua `os.Root` tốn khoảng 10 `openat` (Lstat + Open đi từng thành phần đường dẫn), nên 1 517 file × 4 lần ≈ 60k `openat`.

## Đã sửa

1. `internal/app/skill_list.go`: `ListSkillCatalog` mở catalog một lần (`listSkillsIn` dùng chung handle; `validateSkillState` tách ra).
2. `internal/catalog/skill_issues.go`: export `CanonicalIssues(root)` dùng memo validation đã có (khoá = digest mọi canonical file + stamp Git index). `health_hub.go` và `curation_home.go` gọi nó thay vì `canonical.Validate`, nên `status` validate một lần. Khoá bao trùm mọi input của `Validate`: file nằm sai chỗ thư mục bắt buộc (directory_collision) cũng là một canonical file trong danh sách, `.gitignore` và schema marker là canonical, tracked machine files và unmerged paths nằm trong Git index.
3. `internal/canonical/snapshot_cache.go`: stamp file bền `runtime/catalog/canonical-stamps.json` (JSON, `version: 1`).
   - Chỉ ghi entry *trusted*; chỉ ghi khi có entry trusted mới hoặc số entry trusted giảm; ghi bằng `fsutil.WriteFileAtomic` (0600), người ghi sau thắng; chỉ ghi khi `runtime/catalog/` đã là thư mục thật (không tạo `runtime`, không đi qua symlink).
   - Đọc: thiếu / hỏng JSON / sai version / path không canonical / digest sai dạng / path trùng / file > 64 MiB → bỏ qua cả file, đọc lại mọi thứ, không lỗi.
   - Khoá entry: path + size + mode + mtime (ns) + ctime + device/inode (Linux, macOS) hoặc volume serial/file index + ChangeTime (Windows, đã có sẵn trong `stamp_windows.go`); nền tảng khác: size + mtime. Racy rule giữ như Git và như cache RAM: entry chỉ trusted khi mtime cũ hơn thời điểm băm > 5 s; entry racy không được ghi ra file vì không tiến trình nào sau được tin nó.
   - Đã thử thêm ctime vào racy rule; bỏ lại vì `TestSkillDetailPerformanceBudget` (và mọi hub có file `touch`/`Chtimes`) bị băm lại liên tục, trong khi lợi ích thực tế gần như bằng 0: ctime đã nằm trong stamp nên mọi lần ghi sau khi băm đổi stamp.
   - Đặt cạnh catalog chứ không trong `runtime/cache/` vì `runtime/cache/skills` là chỗ agent được phép ghi; file này chỉ có thể làm catalog được coi là "current", nên nằm nơi chỉ ai thay được catalog mới ghi được.
4. Docs: `docs/design/07-storage-and-mutation-model.md` (cây `runtime/` và mục 14.4). AGENTS.md không đổi: không có owner mới.

## Số đo (hub tổng hợp, máy rảnh, 5 lần/lệnh, `strace -f -c`)

| Lệnh | Wall trước | Wall sau | openat trước → sau | read trước → sau |
|---|---|---|---|---|
| `status` | 0,24–0,28 s | 0,18–0,22 s | 78 595 → 42 520 | 32 207 → 12 260 |
| `skill list` | 0,13–0,17 s | 0,089–0,14 s | 45 239 → 16 950 | 14 478 → 2 487 |
| `skill show perf-150` | 0,086–0,107 s | 0,055–0,065 s | 30 294 → 3 856 | 12 073 → 83 |

`skill show` và `skill list` không còn đọc nội dung canonical file nào khi hub không đổi.

## Các call site liên quan approval / trust (không dùng stamp cache)

`ScanCached` chỉ được gọi từ `catalog/open.go:87` (phán quyết catalog còn current) và `catalog/skill_issues.go` (khoá memo validation). Mọi quyết định trust băm byte mới:

- `SkillService.ContentTrustFor` (`app/skill_detail.go`) → `skillFolderFiles` → `os.ReadFile` + `digest.Bytes` từng file. Dùng bởi: audit (`auditContentDigest`, `skill audit-status` / `audit-record`), review (`skill_review_content.go`), lifecycle (`skill_lifecycle.go`), upstream update, `InactiveSkillReviewGate` (chặn nội dung draft/archived trước khi tới agent).
- `--approve-content`: `approvalRefusal` so với `plan.contentDigest` = `proposedContentDigest(plan.final, …)` tính từ byte cây đã đọc (`skill_commit_plan.go`).
- Build catalog (`catalog/build.go`, `catalog/input.go`) và mutation (`mutation/mutation.go`, `transaction.go`, `filesystem.go`) dùng `canonical.Scan` (không cache).
- Nội dung giao cho agent: snapshot được kiểm từng file với digest của catalog (`app/skill_snapshot.go:665`), `skill.ReadResource` (`skill/resource.go:101`), `catalog/servable.go:374` đều băm mới.

Phần duy nhất dựa vào cache là phán quyết "catalog còn current" (đã dựa vào cache RAM từ trước, cùng quy tắc). Stamp chứa ctime + inode/dev, mà ctime không đặt được từ userspace, nên một lần ghi sau khi băm luôn đổi stamp; racy rule che trường hợp ghi trong cùng tick đồng hồ với bản đã băm. Ngoại lệ còn lại: đồng hồ hệ thống bị lùi, hoặc filesystem không có ctime/inode (khi đó chỉ còn size + mtime + racy rule, như Git). Kể cả khi đó, byte giao cho agent vẫn được so với digest catalog nên không lọt nội dung khác bản catalog.

## Test

`internal/canonical/snapshot_cache_test.go`: `TestScanCachedStampFileCarriesTrustedDigestsToTheNextProcess` (hai `snapshotCache` mô phỏng hai tiến trình): (i) sửa cùng size, khôi phục mtime, và entry có stamp trùng nhưng băm trong racy window → đọc lại; (ii) file hỏng / sai version / digest sai dạng → bỏ qua, kết quả bằng `Scan`; (iii) digest giả cấy vào entry trusted được dùng lại → chứng minh không đọc lại (không thêm seam đếm). Đã kiểm mutation: bỏ racy window hoặc bỏ kiểm version/digest làm test fail. Test cũ không đổi.

Kiểm tra: `make check` exit 0 (gofmt, vet, `GOOS=windows go vet`, lint 0 issue, deadcode không thêm, toàn bộ test). `make test-perf` exit 0. Lần chạy `make test-perf` đầu (khi còn thử ctime, load average ~40 do agent khác) fail `TestSkillDetailPerformanceBudget` và `TestResolverPerformanceBudget` (resolver không dùng code này); sau khi bỏ ctime khỏi racy rule thì pass. Không cần `GOTMPDIR`.

## File ngoài vùng dự kiến

Không. Không chạm `internal/app/errors.go`, mã lỗi, `docs/contracts`, `schemas/`, `web/`, `.github/`, `internal/selfupdate`.

## Câu hỏi mở

1. Phần còn lại lớn nhất của `status` là một lượt `Validate` (đọc + parse mọi file, ~80 ms) mỗi tiến trình. Có thể lưu kết quả validation theo khoá memo hiện có + phiên bản binary, nhưng đó là cache bền thứ hai cho một phán quyết (không phải digest); chưa làm.
2. `verifyPointerDatabase` (`PRAGMA quick_check` toàn bộ DB) tốn 40–60 ms mỗi tiến trình vì cache integrity chỉ ở RAM (`catalog/integrity_cache.go`). Lưu nó ra đĩa là quyết định về bảo đảm chống hỏng DB, không thuộc phạm vi băm hub.
3. `ListSkillUpstream` → `loadTrackedSkills` gọi `skill.Load` cho từng skill (~40 ms với 300 skill) trong `status` và `skill list`.
