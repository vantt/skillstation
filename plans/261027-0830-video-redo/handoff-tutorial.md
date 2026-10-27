# Handoff: làm video hướng dẫn (tutorial) của skillstation

Dán phần dưới "Prompt" vào chat mới, mở trong `~/projects/skillstation`. Mọi thứ cần thiết đã nằm trong repo này
(đã chép từ `~/projects/vlog`); không cần thư mục phụ.

Cập nhật: 2026-10-27. Chưa commit gì: toàn bộ thay đổi video đang nằm trong working tree.

## Prompt

Làm tiếp **video hướng dẫn dùng skillstation** (phim 2). Phim giới thiệu (phim 1) đã xong phần dựng, đừng đụng vào
`video/intro-p3/`, trừ việc đọc để học cách làm.

### Đã chốt (anh duyệt 2026-10-27)

- 16:9 (1920×1080), **giọng đọc tiếng Anh** (edge-tts `en-US-AndrewNeural`), **phụ đề EN và VI**, nhạc nền, dựng bằng
  Remotion.
- Phần nhìn theo **P3**: tím đậm `#23175a` làm nền, xanh lá logo `#9ff07f` làm màu chính, hồng `#ff6fc8` làm màu phụ,
  vàng `#ffd166` nhấn, kem `#f0ece2` cho chữ, `#140c38` cho nền terminal. Font Bricolage Grotesque 800 và JetBrains
  Mono. Viền kem dày, bóng đổ đặc kiểu sticker, chuyển động bật vào (pop), mỗi lúc chỉ một thứ chuyển động. Màu mang
  nghĩa: xanh = một nguồn, bản mới nhất; hồng = bản copy cũ. Khung tĩnh anh đã duyệt: `video/style/` (bảng màu `s0`).
  Mã dùng lại: `video/intro-p3/src/theme.tsx` (màu, font, Pop, sticker, Pill, Mark).
- Cấu trúc câu chuyện giữ nguyên: **hub trước, rồi agent**. Storyboard đã duyệt: `video/tutorial/script.md` (10
  cảnh), cut: `video/tutorial/edit.json`, phụ đề VI: `video/tutorial/narration.vi.json`.
- Anh **không chê cách kể**, chỉ chê graphic, màu, phong cách. Sửa phần nhìn trước.

### Đã làm

- Đổi tên Skill Hub → Skillstation trong `video/demos/` (sandbox, happy-path, tapes 01–06, `web/`, fixtures) và trong
  `script.md`, `edit.json`, `narration.vi.json`, README tutorial. `happy-path.sh` **chạy đủ 14/14 trên v0.8.0**
  (`SKILLSTATION_BIN=release`). Biến môi trường mới: `SKILLSTATION_*`, thư mục hub `~/skillstation-hub`.
- Quay thật Claude Code cho phim 1: `video/demos/agent-take.sh` + `video/demos/tapes/07-intro-agent.tape`. Cách này
  (config Claude riêng chỉ liên kết file credentials, `.mcp.json` trỏ hub demo) dùng được cho OBS-1/OBS-2 của phim 2.
- Bộ nhìn P3 cho VHS: theme JSON ở đầu `07-intro-agent.tape` (dòng `Set Theme {...}`).
- Phim 1 là mẫu về cách dựng Remotion P3, kể cả cách cắt/tăng tốc đoạn quay và vẽ khung nhấn đo theo từng take.

### Việc cần làm

1. **Kiểm kê, rồi báo anh** phần nào của tutorial cũ giữ/bỏ: giữ `script.md`, `edit.json`, `narration.vi.json`,
   `scripts/voice.mjs`; bản quay mp4/webm cũ và `video/tutorial/audio/` phải làm lại (tên Skill Hub, bản 0.7.2, màu
   cũ). `video/tutorial/remotion/` giữ cấu trúc, đổi hết phần nhìn sang P3.
2. **Style frame phụ cho tutorial** (anh chưa duyệt): cảnh terminal hướng dẫn, tiêu đề bước, thẻ chú thích/✓, nhãn máy
   ("Hub computer"/"Agent computer"), thanh tiến độ chapter, khung web UI, thẻ lệnh. Thêm vào `video/style/frames.html`,
   render bằng `video/style/render.mjs`, gửi link xem qua mdview, chờ anh duyệt **trước khi dựng**.
3. **Đổi theme VHS** của `video/demos/tapes/settings.tape` (đang Catppuccin Mocha) và tape máy agent (Latte) sang bảng
   màu P3; quay lại tapes 01–05 và 06 (`render.sh`), cảnh web (`video/demos/web/run.sh`). Kiểm lại chữ trên hình khớp
   `skillstation help <lệnh>` của bản đang quay.
4. **Cảnh 6** (curate my Skillstation) cần skill `skill-curator`. v0.8.0 vẫn tên `system-curator`. Chạy
   `gh release list`: nếu có release mới hơn thì quay trên đó; nếu chưa, quay các cảnh khác trước và giữ cảnh 6 cuối.
   Cảnh 1 và 7c cần một lần quay agent thật "use /commit-style…": dùng `agent-take.sh`.
5. Tạo lại giọng đọc (`node video/tutorial/scripts/voice.mjs`; lời thoại đã đổi tên, cảnh 6 và 10 đã sửa), căn lại
   `edit.json` theo tape mới (`from`, `to`, `say`, `callouts`), dựng bản nháp, gửi anh xem, sửa theo góp ý.
6. **Nhạc:** anh chê nhạc đã chọn cho phim 1 là "quê". Đừng chọn một bài rồi báo. Phim 1 đã chốt "You're Somewhere in Between" (Mixkit). Với phim 2, dùng vòng 2 trong
   `plans/261027-0830-video-redo/music.md` (Mixkit, một CC BY, một CC0), ghép 3–5 bài vào cùng bản nháp để anh nghe so sánh;
   tutorial có giọng đọc nên nhạc phải êm hơn.
   Giấy phép phải rõ (CC0, CC BY); ghi nguồn vào `video/README.md`.

### Quy tắc

- **Ghi khó khăn, nguyên nhân, cách giải, bài học** vào `plans/261027-0830-video-redo/lessons.md` khi xảy ra (đã
  có 11 mục; đọc trước khi quay). Đọc cả `plans/261027-0830-video-redo/ref/a1-nhat-ky-bai-hoc.md` (41 bài học của phim
  mdview).
- **Commit/push chỉ khi anh bảo, từng lần.** Hiện không có gì được commit. Đừng ghi đè các thay đổi chưa commit trong
  `video/demos/` và `plans/`; xem `git status` và `git fetch` trước.
- Duyệt rẻ trước, làm đắt sau: khung tĩnh → quay → dựng. Hỏi anh vì sao chưa ưng trước khi đoán.
- Dùng agent thật thì dọn đường cho nó: không hộp thoại trust, không chờ duyệt lệnh, không gợi ý prompt, cấu trúc đầu
  ra cố định; đọc kỹ mọi chữ trên hình (kể cả chữ mờ) và dòng agent nói.
- Không để lộ: đường dẫn, tên máy, email, token, link phiên web (`serve web` in link có token; chỉ quay bằng thẻ lệnh).
  Kiểm cả banner Claude Code.
- Không quay đoạn chờ liên tục quá vài phút (VHS hết bộ nhớ); không dựa vào chữ hiển thị không ổn định để `Wait`.
- Tiến trình nền: theo dõi và dừng những gì bạn bật (server `serve`, ttyd, vhs); cổng 7420 phải trống khi quay cảnh 6.
- Nhạc, font, tài sản: license rõ; ghi nguồn trong code và `video/README.md`. Remotion miễn phí cho cá nhân và công ty
  dưới 3 người: xem remotion.dev/license trước khi đăng.
- mdview: sau khi tạo/sửa file markdown dài, gọi `mdview_view_file` và đưa link (token đăng nhập `demo`).

### Tài liệu cần đọc

`plans/261027-0830-video-redo/plan.md` (kế hoạch, kiểm kê, quyết định), `lessons.md`,
`film-1-intro-script.md` (mẫu kịch bản/shot list), `video/style/README.md`, `video/intro-p3/README.md`,
`video/tutorial/script.md`, `video/tutorial/README.md`, `video/demos/README.md`,
`plans/261016-1000-intro-video/film-2-tutorial.md` (góp ý các vòng cũ), `docs/user-guide.md`.

## Việc chưa chốt (hỏi anh khi tới đó)

- Có cắt thêm bản ngắn (`short.mp4`) và GIF xem trước cho phim 2 không.
- Có render bản 16:9 của phim 1 cho README GitHub không (em đề xuất có; xem trả lời trong chat).
