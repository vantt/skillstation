# Phim 2: Hướng dẫn sử dụng (16:9, có lời đọc)

Phim dài, ngang, cho người đã muốn dùng. Mục đích: làm theo được từ cài đặt đến skill đầu tiên, Web UI và agent trên
máy khác. Quay thật bằng VHS và Playwright trong sandbox, có lời đọc tiếng Anh (edge-tts), phụ đề EN/VI, nhạc nền
CC0, dựng bằng Remotion. Brief chung: [plan.md](plan.md). Phim giới thiệu: [film-1-intro.md](film-1-intro.md).

## Trạng thái

| Mục | Giá trị |
|---|---|
| Bước hiện tại | **Chờ đổi tên sang Skillstation**, xem [Việc chung](plan.md#việc-chung-cho-hai-phim). Sau đó quay lại, dựng, rồi tới CHECKPOINT 2. |
| Bản xem mới nhất | `video/tutorial/output/draft.mp4` (2026-10-18, còn tên Skill Hub, quay trên v0.7.2), 960×540, 4:11 |
| Storyboard | Đã duyệt ở CHECKPOINT 1 (2026-10-16) |
| Còn treo | Hai cảnh Claude Code đang là hình minh hoạ, ghi chữ "Illustration". Quay thật cần một thư mục cấu hình demo đã đăng nhập. |

## Nằm ở đâu

| File | Nội dung |
|---|---|
| `video/tutorial/script.md` | **Storyboard và lời thoại.** Hàng "Narration" của mỗi cảnh là câu đọc. |
| `video/tutorial/edit.json` | **Bản dựng:** mỗi cảnh dùng đoạn clip nào, câu nào đọc lúc nào, thẻ chú thích nào bật lúc nào |
| `video/tutorial/narration.vi.json` | Phụ đề tiếng Việt, mỗi dòng một câu thoại |
| `video/tutorial/scripts/voice.mjs` | Tạo giọng đọc, timeline và phụ đề |
| `video/tutorial/remotion/` | Phần dựng: khung cửa sổ, tiêu đề bước, thẻ chú thích, motion graphics |
| `video/tutorial/output/youtube.md` | Tiêu đề, mô tả, chapter, tags khi đăng YouTube |
| `video/tutorial/README.md` | Cách dựng lại từ đầu |
| `video/demos/`, `video/recordings/` | Dùng chung: sandbox, tape VHS, cảnh web. Xem [video/README.md](../../video/README.md). |

## Cách góp ý

Ghi vào bảng bên dưới, mỗi vòng một dòng, nói theo cảnh (1 đến 10, xem `script.md`) hoặc theo mốc thời gian trong
video. Góp ý về lời thoại thì sửa ở `script.md`. Góp ý về nhịp, thời điểm hay chú thích thì sửa ở `edit.json`. Góp ý
về hình thì sửa ở `remotion/src/`.

| Vòng | Ngày | Góp ý | Xử lý | Trạng thái |
|---|---|---|---|---|
| 1 | 2026-10-16 | Duyệt storyboard; cảnh hai máy dựng trên một máy; giữ IP LAN; giọng TTS | Ghi vào `script.md`, `plan.md` | Xong |
| 2 | 2026-10-18 | "Nội dung tạm ok, nhưng hình ảnh xấu và thiếu điểm nhấn" | Làm lại phần hình: khung cửa sổ trên nền thương hiệu, tiêu đề bước lớn rồi thu nhỏ, thẻ ✓ đúng lúc có kết quả, camera đẩy nhẹ, chuyển cảnh mờ dần, thanh tiến độ theo chapter. Cảnh Claude Code thành hình minh hoạ. | Xong, chờ duyệt |
| 3 | 2026-10-23 | Có skill `skill-curator`: curate được từ project đã connect, chỉ cần báo một lần ("curate my Skillstation" hoặc `/skill-curator`), sau đó dùng câu ngắn | `script.md`: cảnh 6 thêm bước agent nạp curator, chạy `skillstation version`, và ba câu ngắn (lint, preview commit, áp dụng); cảnh 10 thêm mẹo "chat mới thì báo lại". Cần bản release sau 0.8.0 (0.8.0 còn tên `system-curator`). Chưa căn `edit.json`, chưa làm lại giọng đọc. | Đã sửa kịch bản, chờ duyệt |

## Việc tiếp theo

1. Đổi tên (việc chung), rồi quay lại cả tape và cảnh web trên bản release mới. Bản đó phải có `skill-curator`
   (các commit trên `origin/main` sau 0.8.0), vì cảnh 6 cần nó.
2. Căn lại `edit.json` theo tape mới (`from`, `to`, `say`, `callouts`). Sửa tên trong `script.md` và
   `remotion/src/Graphics.tsx` (logo `skillstation-logo-dark.svg`, URL repo).
3. Dựng bản nháp, chủ project duyệt (CHECKPOINT 2).
4. Bản cuối: `full.mp4`, `short.mp4`, `preview.gif`.
