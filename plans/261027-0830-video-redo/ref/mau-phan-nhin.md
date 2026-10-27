# Mẫu phần nhìn cho video sản phẩm dev tool

Ngày: 2026-10-26. Dùng cho: [A1 mdview hero demo](a1-mdview-hero-demo.md) và A2 làm lại phim skillstation ([đề xuất](2026-10-de-xuat-video.md)).

Mục đích: anh chọn phần nhìn mình thích **trước khi** chốt màu, font, kiểu chuyển động và làm style frame. Lý do: video skillstation không đạt vì graphic, màu sắc, phong cách.

## Cách em chọn và giới hạn

- Tất cả lấy từ **kênh YouTube chính thức** hoặc **trang chủ** của các hãng dev tool nổi tiếng về chất lượng hình ảnh. Link, độ dài, ngày đăng đã kiểm trực tiếp trên YouTube ngày 2026-10-26.
- Ưu tiên video **ngắn (dưới 2 phút), giới thiệu một tính năng**, gần với định dạng A1.
- **Em chưa xem các video này.** Em chỉ xem ảnh thumbnail và định dạng. Mô tả phong cách dưới đây là **[Suy luận]** từ thumbnail và từ phong cách quen thuộc của mỗi hãng; anh xem rồi tự đánh giá.

## 1. Năm phong cách

### P1. Tối giản, nền tối, chuyển động chính xác (Linear)

Nền đen hoặc xám than, chữ trắng mảnh, đường nét mảnh như bản vẽ kỹ thuật, ít màu. Chuyển động chậm, mượt, camera lướt vào giao diện.

| Video | Độ dài | Đăng |
|---|---|---|
| [Loops in Linear](https://www.youtube.com/watch?v=_gBF27M7NHI) | 0:40 | 07/2026 |
| [Automate Triage in Linear](https://www.youtube.com/watch?v=QfRULgmspxs) | 0:54 | 06/2026 |
| [Coding Sessions on mobile](https://www.youtube.com/watch?v=QGeI2cI-S-g) | 0:21 | 07/2026 |

**Hợp với mdview:** mdview bán cảm giác "đọc dễ chịu". Phong cách yên tĩnh này nói đúng điều đó. Rủi ro: làm tới mức của Linear cần tay nghề motion cao; bản tự làm dễ thành "tối và trống".

### P2. Sản phẩm như vật thể, kính và màu nhấn (Raycast)

Nền đen, icon hoặc giao diện được dựng như vật thể 3D bằng kính, có một màu nhấn mạnh (cam, đỏ). Giao diện được phóng to, rõ từng chi tiết.

| Video | Độ dài | Đăng |
|---|---|---|
| [Clipboard History in Raycast](https://www.youtube.com/watch?v=LIJLTKVXv38) | 2:00 | 10/2026 |
| [Shortcuts in Raycast](https://www.youtube.com/watch?v=C0oOsjmXduY) | 3:01 | 10/2026 |
| [New in Raycast AI: Automations, Projects and a new Harness](https://www.youtube.com/watch?v=ucVSAI2gnro) | 4:41 | 09/2026 |

**Hợp với:** cả mdview và skillstation, nếu mỗi sản phẩm có một màu nhấn riêng. Cần logo hoặc icon đẹp làm "vật thể" ở cảnh mở và cảnh cuối. mdview hiện chưa có logo.

### P3. Terminal sặc sỡ, vui (Charm)

Màu tím, xanh ngọc, hồng đậm, chữ to kiểu retro, terminal được trang trí như sân khấu. Năng lượng cao, có cá tính.

| Video | Độ dài | Đăng |
|---|---|---|
| [Your CLI, but Smarter: Crush, Your AI Bestie for the Terminal](https://www.youtube.com/watch?v=Cuhi7y6sRaA) | 0:35 | 07/2025 |
| [You WILL glamourize your CLI](https://www.youtube.com/watch?v=IMd05f0Msyg) | 7:35 | 03/2024 |

**Hợp với:** skillstation (sống trong terminal) và các cảnh terminal của A1. Charm cũng làm ra **VHS**, công cụ skillstation đang dùng để quay terminal, nên phong cách này đi được bằng công cụ sẵn có. Rủi ro: lệch với cảm giác "yên tĩnh, dễ đọc" của mdview.

### P4. Người thật, ánh sáng ấm, xen giao diện (Cursor)

Quay người thật trong không gian ấm, chữ tiêu đề trắng lớn đè lên hình, xen cảnh giao diện.

| Video | Độ dài | Đăng |
|---|---|---|
| [Introducing Cursor for iOS](https://www.youtube.com/watch?v=ovxL42LkKNg) | 1:14 | 06/2026 |

**Hợp với:** sau này, khi anh đã quen lên hình (bậc thang trong identity model). Chưa hợp cho A1 vì cần quay thật và có ánh sáng.

### P5. Đoạn lặp không tiếng trên trang chủ

Không phải video YouTube, mà là đoạn video ngắn tự chạy trên trang chủ. Đây đúng là định dạng của bản 30 giây cho README mdview.

| Trang | Ghi chú |
|---|---|
| [linear.app](https://linear.app) | Có video `.webm` trên trang chủ |
| [warp.dev](https://www.warp.dev) | Có video `.webm`, chủ đề terminal và agent |
| [resend.com](https://resend.com) | Có 4 video `.mp4`, dev tool nhỏ có phần nhìn nổi tiếng là đẹp |
| [vercel.com](https://vercel.com) | Có video `.webm` |
| [cursor.com](https://cursor.com) | Có video `.webm` |

## 2. Gợi ý của em [Suy luận]

- **mdview:** P1 làm nền (yên tĩnh, dễ đọc) cộng một màu nhấn kiểu P2 cho khoảnh khắc click link.
- **skillstation:** P3, vì sản phẩm sống trong terminal và công cụ quay (VHS) đã cùng hệ.
- Hai sản phẩm nên có phần nhìn khác nhau, vì người xem và cảm giác cần truyền khác nhau.

## 3. Phần anh điền

Xem 3–5 video, ghi ngắn vào bảng. Chỉ cần vài chữ mỗi ô.

| Video | Thích / không | Thích gì (màu, font, chuyển động, nhịp, độ tối giản) | Không thích gì |
|---|---|---|---|
| | | | |
| | | | |
| | | | |

Có video ngoài danh sách mà anh thích thì thêm vào bảng. Từ bảng này em chốt bộ nhìn (bảng màu, font, theme terminal và trình duyệt, kiểu chữ trên màn hình) rồi làm 3 style frame tĩnh để anh duyệt.
