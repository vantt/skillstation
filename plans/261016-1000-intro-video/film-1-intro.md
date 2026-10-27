# Phim 1: Giới thiệu (9:16, chỉ có nhạc)

Phim ngắn, dọc, cho người chưa biết Skillstation. Mục đích: hiểu vấn đề và lý do dùng sản phẩm trong khoảng 1:30,
không hướng dẫn thao tác. Làm bằng skill `explainer-ai-for-business-music` (HyperFrames, nét vẽ chì, không lời đọc).
Brief chung: [plan.md](plan.md). Phim hướng dẫn: [film-2-tutorial.md](film-2-tutorial.md).

## Trạng thái

| Mục | Giá trị |
|---|---|
| Bước hiện tại | **Chờ đổi tên sang Skillstation**, xem [Việc chung](plan.md#việc-chung-cho-hai-phim). Sau đó render lại và duyệt ba gate. |
| Bản xem mới nhất | `video/intro/output/intro-explainer-9x16.mp4` (2026-10-18, còn tên Skill Hub), 1080×1920, 1:25 |
| Gate A, B, C | A và B đạt. C mất hiệu lực vì file MP4 đã chuyển sang `video/intro/output/`. Phải render lại sau khi đổi tên. |
| Ngôn ngữ | Tiếng Anh |

## Nằm ở đâu

| File | Nội dung |
|---|---|
| `video/intro/script.json` | **Toàn bộ chữ trên màn hình.** Muốn đổi câu chữ hay số liệu thì sửa ở đây. |
| `video/intro/README.md` | Cách dựng lại, các lỗi đã gặp trên máy này |
| `video/intro/review/` | Báo cáo và dấu duyệt của ba gate (`gates.json`, `content-review.md`, `ui-review.md`, `ui-contact-sheet.jpg`) |
| `video/intro/widgets/intro-notify.js` | Bản sao của template, có một chỗ sửa: khoảng cách giữa hai vế câu mở đầu |
| `video/intro/output/` | Video đã render (không vào Git) |

## Cách góp ý

Ghi vào bảng bên dưới, mỗi vòng một dòng, nói theo cảnh: "intro", "cảnh 1" đến "cảnh 7", "outro". Có thể ghi mốc thời
gian trong video. Agent trả lời trong cột "Xử lý". Sửa `script.json` thì gate A và B phải duyệt lại, còn render lại thì
gate C phải duyệt lại.

| Vòng | Ngày | Góp ý | Xử lý | Trạng thái |
|---|---|---|---|---|
| 1 | 2026-10-18 | Bản Remotion "hình ảnh xấu, thiếu điểm nhấn"; thử skill explainer; tách thành phim giới thiệu và phim hướng dẫn | Làm phim này bằng skill explainer: 7 cảnh, 1:25, ba gate do subagent độc lập duyệt | Xong, chờ duyệt |

## Việc tiếp theo

1. Đổi tên trong `script.json`, gồm `labels.after`, `scenes[*].widget`, `outro.line3` "Search: skillhub-mcp" và
   watermark. Logo mới là `docs/images/brand/skillstation-mark-light.svg`.
2. Chạy lại theo `video/intro/README.md`, duyệt lại ba gate, render.
3. Chủ project duyệt.
