# Chiến lược launch Skill Hub: beta có kiểm soát, rồi public

Ngày: 2026-10-18. Bổ sung cho plan [Promote skillhub-mcp](../261014-1700-promote-skillhub-mcp/plan.md), không thay thế nó.
Plan gốc giữ nguyên các ràng buộc: không đăng gì khi chủ project chưa duyệt từng bài, không nói quá, không lộ dữ liệu riêng.

## Kết luận

Skill Hub chưa sẵn sàng cho public launch: cổng trước launch còn mở năm mục, và chưa ai chạy README trên hai máy thật.
Vì vậy launch đi theo bốn chặng: chuẩn bị cổng, beta kín với 5 đến 10 người quen, mở danh bạ và cộng đồng Việt Nam,
rồi mới đăng public trên Reddit và X/LinkedIn. Không dùng Hacker News và Product Hunt ở vòng này. Ngày launch không
cố định: chặng sau chỉ bắt đầu khi tiêu chí thoát của chặng trước có bằng chứng.

## Quyết định của chủ project (2026-10-18)

| Câu hỏi | Quyết định |
|---|---|
| Hướng launch | Beta có kiểm soát trước, public sau |
| Kênh đăng dưới tên chủ project | Reddit, X, LinkedIn, nhóm Facebook/Discord tiếng Việt. Không Show HN |
| Thời gian | Launch khi qua cổng, không đặt ngày cứng |
| Kiểm tra tên `skillhub` | Kiểm tra trước launch (báo cáo: `researcher-261018-0611-name-check.md`) |

## Hiện trạng (đo ngày 2026-10-18 bằng `gh`)

| Chỉ số | Giá trị |
|---|---|
| Sao | 0 |
| Lượt xem repo 14 ngày | 51 lượt, 5 người |
| Lượt clone 14 ngày | 4433 lượt, 527 nguồn (phần lớn là CI và bot, không dùng làm chỉ số người dùng) |
| Lượt tải asset v0.7.2 / v0.7.1 / v0.7.0 | 33 / 18 / 42 |
| Topics, homepage, Discussions | Chưa có, chưa có, đang tắt |
| FAQ, issue template, demo trong README | Chưa có |
| Video giới thiệu | Có bản nháp `video/output/draft.mp4`, phụ đề EN và VI, mô tả YouTube; chưa upload |
| Host đã kiểm chứng | Chỉ Claude Code. Codex và Gemini CLI chưa kiểm chứng end to end |

## Kênh theo khung ORB

Mọi lượt chú ý từ kênh thuê và kênh mượn phải dẫn về kênh của mình, nơi người dùng ở lại được.

- **Kênh của mình (owned).** Repo GitHub là trang đích duy nhất: README, Releases (người dùng bấm Watch, chọn
  Releases để nhận tin), Discussions để hỏi đáp và góp ý. Kênh YouTube của chủ project giữ video giới thiệu. Không làm
  email list hay Discord riêng ở vòng này (plan gốc đã loại).
- **Kênh thuê (rented).** Reddit (r/ClaudeAI, r/mcp; đọc luật từng sub trước), X và LinkedIn của chủ project, nhóm
  Facebook/Discord dev và AI tiếng Việt. Mỗi bài có một link duy nhất về repo.
- **Kênh mượn (borrowed).** Danh bạ MCP (official MCP registry, Smithery, Glama, mcp.so, PulseMCP) và các awesome list
  về MCP, Claude Code, agent skills (chi tiết ở [phase 2 của plan gốc](../261014-1700-promote-skillhub-mcp/phase-02-directories-and-lists.md)).
  Về sau: newsletter về MCP hoặc Claude Code, người làm nội dung trong cộng đồng Việt Nam đã dùng thử ở beta.

## Bốn chặng

### Chặng 0: Qua cổng chuẩn bị

Việc cần xong, ai làm, bằng chứng:

| Việc | Người làm | Bằng chứng xong |
|---|---|---|
| FAQ `docs/faq.md`, có đoạn "vì sao không symlink `~/.claude/skills`" | Agent viết, chủ project duyệt | File có link từ README và README.vi |
| Issue template (bug, feature) và Discussions bật với Q&A, Ideas, Show and tell, Beta feedback | Agent soạn, chủ project bật | Trang repo khi chưa đăng nhập thấy đủ |
| Description, topics, homepage, ảnh social preview 1280x640 | Agent soạn, chủ project duyệt rồi `gh repo edit` | Trang repo xem trên điện thoại đọc được |
| Demo 30 đến 60 giây đầu README (cắt từ video) | Agent | GIF hoặc MP4 trong `docs/images/demo/`, đã soát từng khung không lộ đường dẫn, token |
| Video giới thiệu qua CHECKPOINT 2 và 3 của [plan video](../261016-1000-intro-video/plan.md) | Chủ project duyệt, tự upload | Link YouTube (có thể để unlisted cho beta) |
| Kiểm tra tên | Agent research, chủ project quyết | Quyết định ghi vào plan gốc |
| Lệnh cài trên Linux, macOS, Windows chạy với release mới nhất | CI release | Smoke install của workflow release xanh |

Tiêu chí thoát: mọi dòng trên có bằng chứng. Ước lượng một đến hai tuần làm việc bán thời gian.

### Chặng 1: Beta kín (5 đến 10 người, khoảng hai tuần)

Mục tiêu là bằng chứng rằng người lạ làm theo README được, trên một máy và trên hai máy, chứ không phải lượt tiếp cận.

- **Chọn người.** Người quen đang dùng Claude Code hằng ngày, có skill riêng trong `~/.claude/skills`; ưu tiên người
  dùng hai máy trở lên. Ít nhất hai người dùng macOS và một người dùng Windows. Mời từng người một qua tin nhắn riêng.
- **Việc nhờ làm.** (1) Làm Quickstart trên một máy, bấm giờ. (2) Thêm một skill của chính họ, dùng nó từ agent.
  (3) Nối máy thứ hai bằng invite theo `docs/remote-hub-guide.md`. (4) Trả lời ba câu: Skill Hub là gì (một câu),
  chỗ nào kẹt, có dùng tiếp không và vì sao.
- **Nơi góp ý.** Discussions, category Beta feedback, hoặc issue. Lỗi chặn người dùng được sửa và ra release ngay trong tuần.
- **Theo dõi.** Bảng trong `plans/reports/launch-beta-<ngày>.md`: người, hệ điều hành, bước nào xong, thời gian, lỗi, câu mô tả.

Tiêu chí thoát (tất cả):

- Ít nhất 5 người làm xong Quickstart một máy; ít nhất 2 người nối được máy thứ hai trên mạng thật (LAN, Tailscale hoặc SSH).
- Thời gian trung vị cho Quickstart một máy không quá 5 phút.
- Không còn lỗi chặn nào mở.
- Ít nhất 3 trong 5 câu mô tả khớp định vị: thư viện skill của riêng mình, trong Git, agent lấy qua MCP.

Điểm dừng: nếu sau hai tuần ít hơn 3 người làm xong, dừng lại, sửa README và onboarding theo chỗ kẹt, chạy lại beta
với người mới. Không sang chặng 2.

### Chặng 2: Danh bạ và cộng đồng Việt Nam (tuần đầu sau beta)

- Gửi listing lên danh bạ và awesome list theo bảng mục tiêu của phase 2 plan gốc. Chủ project gửi từ tài khoản mình.
- Một bài trong một đến hai nhóm Facebook hoặc Discord dev/AI tiếng Việt, kèm video có phụ đề tiếng Việt. Góc kể:
  câu chuyện cá nhân (skill bị lệch giữa các máy và dự án), cách giải, lời mời dùng thử và góp ý trên Discussions.
  Đọc luật nhóm trước; ghi rõ mình là tác giả, dự án còn sớm, chỉ Claude Code đã kiểm chứng.
- Trích dẫn tốt từ người beta (khi họ đồng ý cho dùng) đưa vào bài.

Tiêu chí thoát: ít nhất 3 listing đã live hoặc pull request đang mở; câu hỏi trong nhóm Việt Nam đã trả lời hết;
câu hỏi lặp lại đã vào FAQ.

### Chặng 3: Public launch (một nền tảng mỗi ngày)

- **Ngày L:** bài Reddit ở sub đúng chủ đề nhất (r/ClaudeAI hoặc r/mcp, chọn sau khi đọc luật). Góc: vấn đề và demo,
  không liệt kê tính năng.
- **Ngày L+1:** thread X kèm clip demo; **ngày L+2:** bài LinkedIn, góc thiết kế (Git làm cơ sở dữ liệu, duyệt skill
  trước khi agent nhận). Mỗi bài dẫn về repo.
- **Tùy chọn trong tuần:** bài dài trên dev.to hoặc blog về một lựa chọn thiết kế đáng đọc tự thân.
- Chủ project trực 6 đến 8 giờ sau mỗi bài để trả lời. Ngày trong tuần, giờ sáng ở Mỹ (khoảng 20:00 đến 22:00 giờ Việt Nam) cho Reddit và X.
- Bản nháp các bài và mười câu trả lời sẵn nằm trong `plans/reports/promote-launch-drafts-<ngày>.md` (phase 3 plan gốc),
  viết lại bỏ Show HN, thêm bài tiếng Việt.

Điểm dừng: nếu Quickstart hỏng trong lúc launch, ngưng đăng bài kế tiếp, nói rõ trong thread, sửa, ra release, trả lời bằng link bản sửa.

### Chặng 4: Giữ đà

Giữ nguyên [phase 4 plan gốc](../261014-1700-promote-skillhub-mcp/phase-04-momentum-and-measurement.md): trả lời trong
hai ngày, một release sửa lỗi trong tuần sau launch, đo ở ngày 7 và ngày 30. Thêm phân loại thông báo cho các lần ra sau:

- **Lớn** (ví dụ Codex hoặc Gemini CLI được kiểm chứng end to end, tính năng mới cho người dùng): release note, bài X
  và LinkedIn, bài trong nhóm Việt Nam, cập nhật listing. Đây là lý do chính đáng để "launch lại".
- **Vừa** (cải tiến web UI, lệnh mới): release note và một bài X.
- **Nhỏ** (sửa lỗi): chỉ release note.

## Chỉ số và mục tiêu

Mục tiêu ngày 30 sau public launch của plan gốc giữ nguyên (100 sao, 10 người đã cài và kết nối, 5 góp ý dùng được,
3 listing). Bổ sung chỉ số theo chặng:

| Chặng | Chỉ số chính | Nguồn |
|---|---|---|
| Beta | Số người xong Quickstart, xong hai máy, thời gian trung vị, lỗi chặn | Bảng beta |
| Danh bạ và VN | Listing live, lượt xem repo, người xem duy nhất | GitHub traffic (chủ project xem) |
| Public | Sao, lượt tải asset theo release, issue và Discussions mới | `gh api`, trang release |
| Giữ đà | Thời gian trả lời trung vị, issue đóng trên mở | GitHub |

Lượt clone không dùng làm chỉ số vì phần lớn đến từ CI.

## Rủi ro chính

| Rủi ro | Cách giảm |
|---|---|
| Beta toàn người quen nên khen dễ dãi | Hỏi bằng việc cụ thể và bấm giờ, không hỏi "thấy sao" |
| Nhóm Việt Nam coi là quảng cáo | Đọc luật, kể chuyện cá nhân, ghi tác giả, trả lời mọi bình luận |
| Trùng tên với dự án skillhub khác | Kết quả kiểm tra tên; luôn viết "Skill Hub (`skillhub-mcp`)" kèm link repo |
| Không có email list nên mất người quan tâm | Lời kêu gọi trong mọi bài: Watch, chọn Releases, và Discussions |

## Việc tiếp theo

1. Agent làm phần chuẩn bị của chặng 0 không cần quyền đăng: FAQ, issue template, nháp description và topics, cắt demo.
2. Chủ project xem bản nháp video (CHECKPOINT 2) và đọc báo cáo kiểm tra tên.
3. Chủ project lập danh sách 5 đến 10 người beta.

## Kết quả kiểm tra tên (2026-10-18)

Chi tiết: [researcher-261018-0611-name-check.md](researcher-261018-0611-name-check.md). Rủi ro giữ tên "Skill Hub
(`skillhub-mcp`)" ở mức trung bình đến cao: rủi ro pháp lý thấp (USPTO không có mark SKILLHUB ở class 9 hoặc 42),
nhưng tên đã đông dự án cùng lĩnh vực: iflytek/skillhub (khoảng 5.200 sao), Tencent SkillHub, skillhub.club. Gói
`skillhub-mcp` trên npm và PyPI đã có người khác dùng, và ít nhất sáu gói npm cài một lệnh tên `skillhub`. Chưa kiểm
tra được trademark ở EU, WIPO và Trung Quốc.

Đây là quyết định của chủ project và nằm trong cổng của chặng 0: đổi tên trước launch (rẻ nhất lúc còn 0 sao, nhưng
cần một plan riêng để đổi lệnh, tên MCP server và biến `SKILLHUB_*`), hoặc giữ tên kèm các biện pháp giảm rủi ro trong báo cáo.

**Quyết định 2026-10-19:** đổi tên thành Skillstation, cắt hẳn với tên cũ; xem
[plan đổi tên](../261019-0425-rename-to-skillstation/plan.md). Chặng 0 chờ plan này xong.

## Câu hỏi còn mở

- Chủ project có kênh YouTube để đăng video không, hay để unlisted chỉ dùng link?
- Nhóm Facebook/Discord tiếng Việt cụ thể nào chủ project đã là thành viên và được phép đăng giới thiệu dự án?
- Người beta có được nhận gì không (ví dụ ghi tên trong release note), hay chỉ mời giúp?
