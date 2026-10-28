---
title: "Skillstation intro film (9:16 and 16:9)"
description: "The 41-second intro film: one skill copied into nine projects against one hub every agent pulls from. Redone in the P3 look with Remotion, a real Claude Code take, and a silent 16:9 cut for the README."
status: completed # approved by the owner 2026-10-27; split out of 261016-1000-intro-video on 2026-10-28
priority: P3
branch: main
tags: [promotion, video]
created: 2026-10-16
completed: 2026-10-27
---

# Phim 1: Giới thiệu (9:16 và 16:9, chữ + nhạc)

Phim ngắn cho người chưa biết Skillstation. Mục đích: hiểu vấn đề và lý do dùng sản phẩm trong khoảng 41 giây, không
hướng dẫn thao tác. **Làm lại ngày 2026-10-27** theo bộ nhìn P3, dựng bằng Remotion; bản HyperFrames cũ (skill
explainer, mascot) bị chủ project bỏ vì graphic, màu và phong cách chưa đẹp. Brief chung (nay là plan của phim hướng dẫn): [261016-1000-intro-video](../../../plans/261016-1000-intro-video/plan.md). Phim
hướng dẫn: [film-2-tutorial.md](../../../plans/261016-1000-intro-video/film-2-tutorial.md). Kế hoạch và bài học của lần làm lại:
[plans/261027-0830-video-redo](../../../plans/261027-0830-video-redo/plan.md).

## Trạng thái

| Mục | Giá trị |
|---|---|
| Bước hiện tại | **Xong.** Chủ project duyệt hai bản cuối ngày 2026-10-27 ("hai bản intro ổn"). Chưa đăng; chỉ đăng khi chủ project nói "đăng". |
| Bản cuối | `video/intro-p3/out/intro-9x16.mp4` (1080×1920) và `intro-16x9.mp4` (1920×1080), 41,4 giây, có nhạc, -16 LUFS; `intro-readme.mp4` (16:9, không tiếng, 2,8 MB) cho README. Không vào Git. |
| Gate A, B, C | Không còn áp dụng: chúng thuộc quy trình skill explainer cũ. Thay bằng khung tĩnh duyệt trước khi dựng, rồi duyệt bản nháp. |
| Ngôn ngữ | Tiếng Anh |
| Nhạc | "You're Somewhere in Between" (Michael Ramir C., Mixkit Stock Music Free License, không phải CC). Không commit file nhạc. |

## Nằm ở đâu

| File | Nội dung |
|---|---|
| `video/intro-p3/` | Dự án Remotion của phim (`src/Intro.tsx` bản dọc, `src/IntroWide.tsx` bản ngang), cách dựng lại: [README](../../../video/intro-p3/README.md) |
| `plans/261027-0830-video-redo/film-1-intro-script.md` | Kịch bản và shot list đã duyệt |
| `video/style/` | Khung tĩnh P3 đã duyệt |
| `video/demos/agent-take.sh`, `tapes/07-*`, `tapes/08-*` | Bản quay thật Claude Code cho cảnh agent |
| `video/intro/` | Bản HyperFrames cũ, không dùng nữa; chưa vào Git, giữ tới khi chủ project cho xóa |

## Cách góp ý

Ghi vào bảng bên dưới, mỗi vòng một dòng, nói theo cảnh (1 đến 6 của [kịch bản](../../../plans/261027-0830-video-redo/film-1-intro-script.md)) hoặc theo mốc thời gian. Agent trả lời trong cột "Xử lý".

| Vòng | Ngày | Góp ý | Xử lý | Trạng thái |
|---|---|---|---|---|
| 1 | 2026-10-18 | Bản Remotion "hình ảnh xấu, thiếu điểm nhấn"; thử skill explainer; tách thành phim giới thiệu và phim hướng dẫn | Làm phim này bằng skill explainer: 7 cảnh, 1:25, ba gate do subagent độc lập duyệt | Thay bằng vòng 2 |
| 2 | 2026-10-26 | Vẫn chưa ưng: graphic, màu sắc, phong cách không đẹp (không phải mascot hay cách kể) | Chọn phong cách P3, làm khung tĩnh; làm lại bằng Remotion, 9:16, khoảng 60 giây, chữ + nhạc | Xong |
| 3 | 2026-10-27 | Duyệt 4 khung P3; duyệt kịch bản. "36 copies, 9 projects" khó hiểu; nhịp cảnh 2 và 3 chậm; cảnh agent chưa rõ, cần quay thật | Hook thành "One skill. Nine copies." kèm tên skill; rút 60 → 41 giây; cảnh 5 quay thật Claude Code, chú thích `skill_resolve → skill_get`; khung nhấn `Checked-by` đặt lại đúng lúc, đúng chỗ | Xong |
| 4 | 2026-10-27 | Chọn nhạc: nhóm đầu "quê"; chọn "You're Somewhere in Between"; làm bản 16:9 (README không tiếng, bản có nhạc) | Tìm lại nhạc, so sánh 7 bài; thêm composition 16:9; chuẩn hóa -16 LUFS | Xong |
| 5 | 2026-10-27 | "Hai bản intro ổn"; commit phần video của phim 1 | Commit `feat(video): add the intro film in the new look…` | Xong |

## Việc tiếp theo

Phim đã xong và được duyệt. Còn lại, chỉ khi chủ project quyết:

1. ~~Đưa `intro-readme.mp4` vào README~~ Xong 2026-10-28: poster liên kết tới `docs/images/demo/intro.mp4` trong
   `README.md` và `README.vi.md`; xem [video/intro-p3/README.md](../../../video/intro-p3/README.md) nếu muốn phát ngay trong README.
2. Đăng bản 16:9 / 9:16 lên kênh. Chưa đăng.
3. Xóa `video/intro/` (bản HyperFrames cũ, 185 MB, chưa vào Git) khi chủ project cho phép.
