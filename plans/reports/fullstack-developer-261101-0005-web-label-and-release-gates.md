# Web: nút Upload zip riêng; Release: gate CI, chữ ký, notices

Trạng thái: DONE. Chưa commit.

## A. Web UI (theo hướng dẫn cuối của user: hai tính năng riêng)

- Nhãn: giữ "Add from GitHub" (`action.add_from_github`), thêm "Upload zip" (`action.upload_zip`). Đây là locale duy nhất: `web/src/i18n/en.ts`.
- `web/src/domain/add-skill-source.ts` (mới): `parseAddSource`, `addSkillPath`, `ADD_SOURCE_PARAM`. Nguồn nằm trong query `?source=github|zip`; thiếu hoặc sai giá trị thì về `github`.
- Skills (thanh công cụ + empty state) và Learning/Sources (thanh công cụ + empty state): mỗi chỗ có hai link `/skills/add?source=github` và `/skills/add?source=zip`. Help text của Sources đã đổi để nhắc cả hai cách.
- AddSkillScreen: nguồn đọc từ URL; nút toggle ghi URL (replace), nên tiêu đề trang (AppShell) đi theo. GitHub chỉ hiện ô "Paste a GitHub URL". Zip chỉ hiện "Upload a .zip file", vẫn giữ gợi ý 10 MiB / SKILL.md. Nút chính: "Discover skills" (GitHub) và "Upload and discover skills" (zip).
- Lỗi rò state đã sửa. Trước đây, đổi nguồn vẫn giữ file zip đã chọn: input bị render lại rỗng nhưng nút Preview vẫn bật. Lời nhắc chọn skill và banner lỗi của nguồn kia cũng còn lại. Ngoài ra, ở bước review của zip, ô Locator hiện URL GitHub cũ. Không có ô ref/branch nào.
- Test: unit cho hai link (Skills, Sources), `?source=zip`/giá trị lạ, đổi nguồn. Thêm e2e `tier2.spec.ts`: bấm Upload zip trên Skills thì vào trang file picker, toggle vẫn hoạt động. `docs/web-ui.md` đã cập nhật.
- `make web-check` pass. `make web-e2e` pass (44 test pass, 2 bị skip). Không cần dev server; không còn process nào sót lại.
- dist: chỉ `internal/delivery/web/dist/.gitkeep` được track (`.gitignore`). dist được build bằng `make web-build` / job web của CI, không commit, nên không cần rebuild để commit.

## B. Release

1. `ci.yml` có thêm `workflow_call`. Trong `release.yml`, job `verify` và `web` được thay bằng `ci: uses: ./.github/workflows/ci.yml`; build/draft/sign/publish `needs: ci`. Lint khi push tag hoặc chạy tay sẽ lấy base là tag release trước (vì `before` rỗng hoặc toàn số 0). Job web của CI chép `THIRD_PARTY_NOTICES.md` vào dist sau e2e, nên artifact `web-dist` giữ nguyên nội dung. Identity ký vẫn là `release.yml`. actionlint sạch.
2. selfupdate: tài liệu cho phép chỉ kiểm checksum (`docs/installation.md` "Check signatures"; `docs/design/01-system-architecture.md:558` "ký artifact khi hạ tầng cho phép"; installer cũng làm như vậy). Hành vi giữ nguyên nhưng được nói rõ: cảnh báo đi ra stderr nên vẫn thấy khi dùng `--json`, và message kết quả ghi "checksum verified; signature not checked…" hoặc "checksum and Sigstore signature verified". Đã cập nhật docs/test.
3. `web/scripts/notices.mjs --check`, chạy trong `make web-test` (tức là cả `web-check` và CI). Nó báo ERROR/WHY/FIX khi file đã cũ. Release runbook đã cập nhật.

## Câu hỏi mở
- Có nên bắt buộc chữ ký mặc định (fail closed) không? Đây là quyết định sản phẩm, trái với docs hiện tại, nên chưa đổi.
- Release giờ chạy toàn bộ CI, gồm cả fuzz/e2e/cross-compile, nên lâu hơn trước một chút.
