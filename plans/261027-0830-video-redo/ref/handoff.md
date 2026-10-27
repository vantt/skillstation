# Handoff: hai video skillstation

Mở chat mới trong `~/projects/skillstation`, thêm `~/projects/vlog` làm thư mục phụ (app: thêm folder vào session; terminal: `claude --add-dir ~/projects/vlog`), rồi dán phần dưới đây.

---

Làm tiếp video cho **skillstation** (repo `~/projects/skillstation`, hub phục vụ skill cho agent qua MCP: "Your skills in one place. Pulled by any agent, on demand."). Anh muốn **hai video riêng**:

1. **Video giới thiệu:** người xem hiểu skillstation làm gì và muốn dùng. Ngắn, kể trước/sau, không dạy cài đặt.
2. **Video hướng dẫn dùng:** cài, tạo hub, kết nối agent, thêm skill đầu tiên, agent tự lấy skill. Người xem làm theo được.

## Đọc trước

- `~/projects/vlog/docs/video-ideas/2026-10-de-xuat-video.md`, mục **A2** (và A3): góp ý cũ, hướng làm lại.
- `~/projects/vlog/docs/video-ideas/mau-phan-nhin.md`: anh chọn phong cách **P3** (terminal sôi động kiểu Charm) cho skillstation.
- `~/projects/vlog/docs/video-ideas/style-frames/README.md` và thư mục `skillstation/` cạnh nó (s0 bảng màu, s1 hook, s2 hub, s3 cảnh cuối; nguồn `frames.html`, render bằng `render.mjs`). **Anh chưa duyệt 4 khung này.**
- `~/projects/vlog/docs/video-ideas/a1-mdview-hero-demo.md` và **`a1-nhat-ky-bai-hoc.md`**: cách đã làm video mdview và 41 bài học. Đọc nhật ký trước khi quay.
- Bộ công cụ A1 đã commit ở `~/projects/mdview/video/hero-demo/` (sandbox.sh, tapes VHS, record.mjs Playwright, Remotion `edit/`). Dùng lại khung này.
- Trong skillstation: `video/README.md`, `plans/261016-1000-intro-video/` (brief, góp ý hai phim cũ), `video/demos/` (sandbox, VHS tapes 01–06, web scenes), `video/tutorial/` (phim hướng dẫn 4:10 cũ, Remotion, giọng edge-tts), `docs/user-guide.md`.

## Đã biết

- Hai phim cũ: intro 9:16 ~1:25 (HyperFrames, chỉ nhạc) và tutorial 16:9 ~4:10 (Remotion, giọng tiếng Việt). **Anh không ưng vì graphic, màu sắc, phong cách không đẹp. Không phải do mascot hay cách kể.** Sửa phần nhìn trước.
- Bộ nhìn P3 trong style frame: tím đậm làm nền, **xanh lá logo làm màu chính**, hồng làm màu phụ; xanh = một nguồn, bản mới nhất; hồng = bản copy cũ. Font Bricolage Grotesque 800 và JetBrains Mono. Viền kem dày, bóng đổ đặc kiểu sticker.
- Ý câu chuyện video 1: cùng một skill bị copy vào nhiều repo, lệch phiên bản; so với một hub mà agent gọi `skill_resolve` là nhận đúng bản mới nhất.
- `~/projects/skillstation` đang có thay đổi chưa commit trong `video/demos/` và `plans/`. Xem `git status` và `git fetch` trước; không ghi đè.

## Dùng lại kết quả của hai phim cũ

Anh muốn tận dụng một phần những gì đã làm ở skillstation, không làm lại từ đầu. Kiểm kê trước, rồi ghi rõ phần nào giữ, phần nào bỏ, vì sao:

- **Nội dung và kịch bản:** `video/tutorial/script.md`, `narration.vi.json`, `edit.json`; `video/intro/script.json`. Anh không chê cách kể, nên giữ được mạch và lời, chỉ đổi phần nhìn.
- **Bản quay terminal và web:** `video/recordings/tapes/01-install … 06-agent-connect.mp4`, `recordings/web/web-edit.webm`, `web-approve.webm`, transcripts. Các script dựng lại chúng (`video/demos/`: sandbox, happy-path, tapes, `web/record.mjs`) là phần đáng giữ nhất. Đổi theme VHS/CSS sang bảng màu P3 rồi quay lại, đừng dùng mp4 cũ nếu màu lệch bộ nhìn mới. Kiểm tra còn khớp phiên bản skillstation hiện tại không.
- **Giọng đọc:** `video/tutorial/audio/scene-*.wav`, `timeline.json` (edge-tts). Giữ cho video 2 nếu kịch bản không đổi nhiều.
- **Bỏ hoặc cân nhắc:** mascot, prop, watermark trong `video/intro/assets/` và theme HyperFrames của intro: chính là phần nhìn anh chưa ưng.

## Cách làm

1. Đưa link xem 4 style frame (mdview `mdview_view_file`) để anh duyệt; sửa theo góp ý, ghi vào bảng góp ý trong `style-frames/README.md`. Có thể cần thêm khung cho video 2 (cảnh terminal hướng dẫn, chú thích bước).
2. Viết kịch bản và shot list cho từng video, file riêng trong `~/projects/vlog/docs/video-ideas/` (ví dụ `a2-skillstation-gioi-thieu.md`, `a2-skillstation-huong-dan.md`). Chốt với anh: độ dài, tỉ lệ khung (16:9 và/hoặc 9:16), giọng đọc hay chỉ chữ + nhạc, tiếng Anh hay tiếng Việt.
3. Quay tự động hết mức (sandbox, VHS, Playwright), dựng bằng Remotion. Video 2 dùng lại tapes 01–06 nếu còn đúng phiên bản hiện tại.
4. Render bản nháp, gửi anh xem, sửa theo góp ý.

## Quy tắc

- **Ghi khó khăn, nguyên nhân, cách giải và bài học** khi chúng xảy ra, vào `~/projects/vlog/docs/video-ideas/a2-nhat-ky-bai-hoc.md` (cùng dạng bảng với A1).
- Commit/push chỉ khi anh bảo, từng lần.
- Thứ anh tự kể về bản thân: ghi vào `~/projects/vlog/identity-model/chat-lieu-ca-nhan.md`, không đánh giá.
- Nhạc và tài sản phải có license rõ (CC0, CC BY); ghi nguồn trong code. Gu nghe/nhìn để anh chọn bằng bản xem thử cạnh nhau.
- mdview đăng nhập bằng token `demo` nếu cần.
