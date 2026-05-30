# Requirements: WAF UI & Experience Overhaul (Đợt 2)

## Introduction

Cải tiến giao diện, trải nghiệm người dùng, và tính năng quản lý cho Kiro WAF. Bao gồm: tiếng Việt có dấu trên homepage, cấu hình gói dịch vụ động từ admin panel, cải tiến challenge pages, CLI output hiện đại có màu, và tăng cường chống crack license.

## Glossary

- **Plan_Config**: Cấu hình gói dịch vụ (giới hạn rate, domain, XDP, OTA) được lưu trong database và đồng bộ xuống client qua heartbeat
- **Hold_Challenge**: Trang xác thực yêu cầu người dùng bấm giữ nút 2-5 giây với thanh loading progress
- **PoW_Challenge**: Trang landing tự động chạy JavaScript Proof-of-Work, người dùng chỉ thấy loading rồi tự redirect
- **CLI_Output**: Kết quả hiển thị trên terminal của kiro-cli với ANSI colors và box drawing

## Requirements

### Yêu Cầu 1: Homepage Tiếng Việt Có Dấu

**Câu Chuyện Người Dùng:** Là người truy cập website, tôi muốn trang chủ hiển thị tiếng Việt có dấu đầy đủ và chuyên nghiệp, để tôi tin tưởng sản phẩm.

#### Tiêu Chí Chấp Nhận

1. Tất cả text trên homepage PHẢI có dấu tiếng Việt đúng chính tả (ví dụ: "Bảo vệ" thay vì "Bao ve", "Giải pháp" thay vì "Giai phap")
2. Navigation bar PHẢI hiển thị: "Trang chủ", "Tài liệu", "Hướng dẫn cài đặt", "Bảng giá", "Admin"
3. Bảng giá PHẢI hiển thị đúng: "Miễn phí", "Không giới hạn", "Hỗ trợ ưu tiên", giá USD và VND
4. Section "Hướng dẫn cài đặt" PHẢI hiển thị lệnh install đầy đủ với nút Copy
5. Footer PHẢI có dấu tiếng Việt đúng

### Yêu Cầu 2: Admin Cấu Hình Gói Dịch Vụ Động

**Câu Chuyện Người Dùng:** Là admin, tôi muốn cấu hình giá, thời hạn, và giới hạn cho mỗi gói dịch vụ từ web admin, để tôi có thể thay đổi chính sách mà không cần sửa code.

#### Tiêu Chí Chấp Nhận

1. Admin panel PHẢI có trang /admin/plans hiển thị danh sách gói dịch vụ hiện tại
2. Mỗi gói PHẢI có các trường cấu hình: tên gói, giá USD, giá VND, rate limit (req/phút/IP), subnet RPM, số domain tối đa, XDP enabled, OTA enabled, thời hạn mặc định (ngày), mô tả
3. Admin PHẢI có thể sửa cấu hình gói và lưu vào database (bảng plan_configs)
4. KHI admin thay đổi cấu hình gói, client node PHẢI nhận config mới qua heartbeat response tiếp theo (không cần restart client)
5. Plan config KHÔNG ĐƯỢC hardcode trong source code — PHẢI đọc từ database
6. KHI tạo license mới, admin chọn gói từ danh sách gói đã cấu hình trong database

### Yêu Cầu 3: Cải Tiến Challenge Pages

**Câu Chuyện Người Dùng:** Là người dùng cuối, tôi muốn trang challenge (Hold và PoW) hoạt động mượt mà và dễ hiểu, để tôi không bị nhầm lẫn khi bị challenge.

#### Tiêu Chí Chấp Nhận

1. Hold_Challenge PHẢI hiển thị nút bấm giữ với thanh loading progress tròn hoặc ngang, thời gian giữ 2-5 giây (cấu hình được)
2. Hold_Challenge PHẢI có text tiếng Việt có dấu: "Vui lòng bấm và giữ nút bên dưới để xác thực"
3. PoW_Challenge PHẢI hiển thị trang landing với animation loading và text: "Đang xác thực truy cập..." rồi tự redirect khi hoàn thành
4. PoW_Challenge PHẢI có thanh progress hiển thị tiến trình tính toán (0% → 100%)
5. Admin PHẢI có thể chọn challenge mode cho mỗi gói: "pow" (mặc định), "hold", hoặc "both" (PoW trước, Hold sau nếu nghi ngờ)
6. Cả hai challenge pages PHẢI responsive trên mobile và có dark theme phù hợp brand

### Yêu Cầu 4: CLI Output Hiện Đại Có Màu

**Câu Chuyện Người Dùng:** Là operator, tôi muốn output của kiro-cli có màu sắc và format đẹp, để tôi dễ đọc trạng thái hệ thống nhanh chóng.

#### Tiêu Chí Chấp Nhận

1. `kiro-cli version` PHẢI hiển thị version với tên sản phẩm có màu (teal/cyan)
2. `kiro-cli status` PHẢI hiển thị bảng có border (box drawing characters), trạng thái xanh = OK, đỏ = lỗi, vàng = cảnh báo
3. `kiro-cli report` PHẢI hiển thị thông tin hệ thống với format dễ đọc: CPU, RAM (có progress bar text), goroutines, uptime
4. `kiro-cli update check` PHẢI hiển thị kết quả có màu: xanh nếu đã mới nhất, vàng nếu có update
5. KHI biến môi trường NO_COLOR được set hoặc terminal không hỗ trợ ANSI, CLI PHẢI fallback về output plain text không màu
6. Tất cả lệnh CLI PHẢI có flag --json để output JSON thay vì text có màu (cho scripting)

### Yêu Cầu 5: Tăng Cường Chống Crack License

**Câu Chuyện Người Dùng:** Là chủ sản phẩm, tôi muốn hệ thống chống crack mạnh hơn, để người dùng không thể bypass license hoặc tự nâng cấp gói.

#### Tiêu Chí Chấp Nhận

1. Plan config KHÔNG BAO GIỜ được cache local trên client — PHẢI lấy từ server mỗi heartbeat
2. NẾU client không gửi heartbeat trong 5 phút, client PHẢI tự động vào lockdown mode (đã có)
3. NẾU binary bị modify (SHA-256 mismatch), server PHẢI lock client với reason "binary_integrity_mismatch" (đã có)
4. Server PHẢI validate request count từ heartbeat stats — nếu client báo cáo request count vượt quá plan limit, server ghi log cảnh báo
5. NẾU license có client_ip được set và heartbeat đến từ IP khác, server PHẢI từ chối với reason "ip_mismatch" (yêu cầu admin re-activate)
6. Client PHẢI gửi binary hash + node_id + stats trong mỗi heartbeat để server có thể audit

### Yêu Cầu 6: Thời Hạn Linh Hoạt

**Câu Chuyện Người Dùng:** Là admin, tôi muốn đặt thời hạn license linh hoạt (ngày/tháng/năm), để tôi có thể bán gói theo nhiều chu kỳ khác nhau.

#### Tiêu Chí Chấp Nhận

1. KHI tạo license, admin PHẢI có thể nhập số ngày hiệu lực (1-3650)
2. Admin panel PHẢI hiển thị "còn X ngày" hoặc "hết hạn Y ngày trước" cho mỗi license
3. Homepage bảng giá PHẢI hiển thị thời hạn theo gói: "30 ngày" (Community), "365 ngày" (Pro), "3650 ngày" (Enterprise) — lấy từ plan_configs trong database
4. KHI license hết hạn, hệ thống PHẢI tự động downgrade về Community plan config (đã implement)
5. Admin PHẢI có thể gia hạn license bất kỳ lúc nào (đã có nút "Gia hạn")
