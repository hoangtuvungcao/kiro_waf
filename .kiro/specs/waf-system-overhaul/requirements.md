# Requirements Document

## Introduction

Nâng cấp toàn diện hệ thống kiro_waf để đạt trạng thái sẵn sàng cho production với giao diện chuyên nghiệp, cơ chế bảo vệ mạnh mẽ, trang challenge đáng tin cậy, bảng quản trị an toàn, quy trình cập nhật qua CLI, và hệ thống phát hiện/chặn XDP thông minh. Hệ thống bảo vệ các máy chủ Ubuntu 22.04 LTS đơn lẻ chống lại các cuộc tấn công DDoS, botnet, flood, bypass, và tấn công tầng ứng dụng sử dụng kiến trúc phòng thủ nhiều lớp (XDP/eBPF → nftables → reverse proxy WAF → ứng dụng).

Máy chủ VPS quản lý chính: 103.77.246.198 (Ubuntu 22.04) — chạy các dịch vụ quản lý license key, phân phối bản cập nhật, và trang chủ dự án tại `firewall.vpsgen.com`.

## Glossary

- **Master_Server**: Máy chủ quản lý (`master-server/main.go`) chạy cơ sở dữ liệu license SQLite, bảng điều khiển admin, trang chủ, và các API endpoint tại `firewall.vpsgen.com`
- **Client_WAF**: Reverse proxy WAF phía client (`client-node/client_waf.go`) xử lý lọc lưu lượng, JS challenge, hold captcha, giới hạn tốc độ, và đồng bộ blocklist XDP
- **Admin_Panel**: Giao diện quản trị web chỉ truy cập được qua đường dẫn `/admin` với bảo vệ bằng mật khẩu
- **Challenge_Page**: Các trang HTML phục vụ cho khách truy cập đáng ngờ, yêu cầu xác minh JS Proof-of-Work hoặc Hold-to-Confirm trước khi truy cập backend
- **XDP_Filter**: Bộ lọc gói tin eBPF/XDP (`client-node/xdp_filter.c`) hoạt động tại L3/L4 để drop gói tin tốc độ cao
- **CLI_Tool**: Giao diện dòng lệnh (`kiro-cli`) dùng cho quản trị cục bộ, cập nhật, và chẩn đoán
- **Update_System**: Cơ chế phân phối phiên bản binary mới từ Master_Server đến các node Client_WAF
- **Ban_Engine**: Hệ thống con trong Client_WAF chịu trách nhiệm phát hiện IP độc hại và thêm vào danh sách chặn tạm thời hoặc vĩnh viễn
- **Detection_Engine**: Logic kết hợp của giới hạn tốc độ, phân tích hành vi, chấm điểm User-Agent, và khớp mẫu để nhận diện mối đe dọa
- **Homepage**: Trang đích công khai tại đường dẫn gốc hiển thị thông tin sản phẩm mà không lộ chức năng quản trị

## Requirements

### Requirement 1: Tính Đúng Đắn Chức Năng Trang Challenge

**User Story:** Là một khách truy cập bị đánh dấu đáng ngờ, tôi muốn trang JS challenge và hold captcha hoạt động chính xác trên tất cả trình duyệt hiện đại, để tôi có thể chứng minh mình là người thật và truy cập trang web.

#### Acceptance Criteria

1. WHEN khách truy cập kích hoạt JS Proof-of-Work challenge, THE Challenge_Page SHALL hiển thị trang chức năng tính toán bằng chứng SHA-256 trong vòng 10 giây trên phần cứng tiêu chuẩn và tự động gửi yêu cầu xác minh
2. WHEN khách truy cập kích hoạt Hold-to-Confirm captcha, THE Challenge_Page SHALL yêu cầu thời gian giữ tối thiểu 2 giây trước khi chấp nhận xác minh
3. WHEN xác minh Challenge_Page thành công, THE Client_WAF SHALL đặt cookie truy cập có chữ ký HMAC và chuyển hướng khách truy cập đến URL được yêu cầu ban đầu
4. IF JavaScript của Challenge_Page không thể thực thi, THEN THE Challenge_Page SHALL hiển thị thông báo dự phòng hướng dẫn khách truy cập bật JavaScript
5. WHEN Challenge_Page được hiển thị, THE Challenge_Page SHALL hoạt động chính xác trên Chrome, Firefox, Safari, và Edge mà không phụ thuộc CDN bên ngoài

### Requirement 2: Giao Diện Trang Challenge Chuyên Nghiệp

**User Story:** Là chủ sở hữu trang web, tôi muốn các trang challenge trông chuyên nghiệp và đáng tin cậy, để khách truy cập hợp lệ không bị lo lắng bởi quy trình xác minh.

#### Acceptance Criteria

1. THE Challenge_Page SHALL sử dụng thiết kế tông tối với nhận diện thương hiệu Kiro bao gồm logo, gradient accent, và typography nhất quán
2. THE Challenge_Page SHALL hiển thị chỉ báo tiến trình cho trạng thái xác minh với hiệu ứng CSS mượt mà
3. THE Challenge_Page SHALL responsive và hiển thị chính xác trên viewport từ 320px đến 2560px
4. THE Challenge_Page SHALL tải mà không có yêu cầu mạng bên ngoài cho font, stylesheet, hoặc script ngoài endpoint xác minh
5. WHEN quá trình xác minh đang diễn ra, THE Challenge_Page SHALL hiển thị văn bản trạng thái bằng tiếng Việt với chỉ dẫn rõ ràng về những gì đang xảy ra

### Requirement 3: Kiểm Soát Truy Cập Bảng Quản Trị

**User Story:** Là quản trị viên hệ thống, tôi muốn bảng quản trị chỉ truy cập được qua đường dẫn `/admin` với bảo vệ mật khẩu và ẩn khỏi trang chủ công khai, để kẻ tấn công không thể phát hiện hoặc brute-force giao diện quản lý.

#### Acceptance Criteria

1. THE Homepage SHALL KHÔNG chứa bất kỳ liên kết, tham chiếu, hoặc metadata nào trỏ đến đường dẫn Admin_Panel
2. WHEN yêu cầu đến `/admin/` từ IP không nằm trong danh sách cho phép admin, THE Master_Server SHALL trả về HTTP 404 Not Found
3. WHEN yêu cầu đến `/admin/` từ IP được phép nhưng không có phiên hợp lệ, THE Master_Server SHALL hiển thị form đăng nhập yêu cầu admin key
4. WHEN admin key sai được gửi hơn 5 lần từ cùng một IP trong vòng 10 phút, THE Master_Server SHALL tạm thời chặn IP đó khỏi endpoint đăng nhập trong 30 phút
5. WHEN phiên admin hợp lệ được thiết lập, THE Admin_Panel SHALL đặt cookie HttpOnly, SameSite=Strict với TTL có thể cấu hình mặc định 12 giờ

### Requirement 4: Giao Diện Bảng Điều Khiển Admin Chuyên Nghiệp

**User Story:** Là quản trị viên hệ thống, tôi muốn bảng điều khiển admin chuyên nghiệp, trực quan, và đầy đủ tính năng, để tôi có thể quản lý license, giám sát node, và phát hành bản cập nhật hiệu quả.

#### Acceptance Criteria

1. THE Admin_Panel SHALL hiển thị phần tổng quan gồm tổng số license, node đang hoạt động, heartbeat gần đây, và tình trạng hệ thống ở một cái nhìn tổng quát
2. THE Admin_Panel SHALL cung cấp giao diện quản lý license hỗ trợ các thao tác tạo, xem, sửa, gia hạn, xoay key, thu hồi, kích hoạt, và xóa
3. THE Admin_Panel SHALL cung cấp giao diện quản lý phát hành để đăng và xóa artifact cập nhật với các trường component, channel, version, artifact URL, và SHA256
4. THE Admin_Panel SHALL hiển thị bản ghi heartbeat và báo cáo gần đây trong bảng có thể sắp xếp với các cột timestamp, node ID, IP, và trạng thái
5. THE Admin_Panel SHALL sử dụng thiết kế tông tối nhất quán với phân cấp trực quan rõ ràng, khoảng cách hợp lý, và typography dễ đọc ở mọi kích thước viewport
6. WHEN thao tác admin thành công hoặc thất bại, THE Admin_Panel SHALL hiển thị thông báo flash với nội dung kết quả

### Requirement 5: Cơ Chế Cập Nhật Qua CLI

**User Story:** Là người vận hành máy chủ, tôi muốn nhận thông báo cập nhật trong CLI và thực hiện cập nhật qua CLI với tải xuống binary tự động và xác minh, để tôi có thể giữ các node luôn cập nhật mà không cần chuyển file thủ công.

#### Acceptance Criteria

1. WHEN có bản phát hành mới, THE Client_WAF SHALL ghi thông báo ra stdout bao gồm tên component, phiên bản hiện tại, phiên bản mới, và artifact URL
2. WHEN người vận hành chạy lệnh cập nhật qua CLI_Tool, THE Update_System SHALL tải artifact từ URL được chỉ định trong metadata phát hành
3. WHEN artifact được tải xuống, THE Update_System SHALL xác minh checksum SHA-256 khớp với giá trị được công bố trong metadata phát hành trước khi tiếp tục
4. IF xác minh SHA-256 thất bại, THEN THE Update_System SHALL hủy cập nhật, ghi log lỗi, và giữ nguyên binary hiện tại
5. WHEN checksum được xác minh, THE Update_System SHALL thay thế binary hiện tại một cách nguyên tử, khởi động lại dịch vụ, và xác minh health trong vòng 30 giây
6. IF kiểm tra health thất bại sau khi thay thế binary, THEN THE Update_System SHALL rollback về binary trước đó và khởi động lại dịch vụ

### Requirement 6: Phát Hiện và Chặn XDP/Ban/Block Thông Minh

**User Story:** Là quản trị viên hệ thống, tôi muốn bộ lọc XDP và ban engine phát hiện và chặn mối đe dọa một cách thông minh bao gồm các nỗ lực bypass, botnet, flood, và tấn công tài nguyên, để máy chủ vẫn khả dụng khi bị tấn công.

#### Acceptance Criteria

1. THE XDP_Filter SHALL drop gói tin khớp với LPM trie blocklist IPv4 ở tốc độ line rate mà không tiêu tốn CPU ứng dụng
2. WHEN IP nguồn vượt ngưỡng packets-per-second được cấu hình cho mỗi IP, THE XDP_Filter SHALL drop các gói tin tiếp theo từ IP đó trong phần còn lại của cửa sổ rate
3. WHEN subnet /24 vượt ngưỡng packets-per-second được cấu hình cho subnet, THE XDP_Filter SHALL drop các gói tin tiếp theo từ tất cả IP trong subnet đó trong phần còn lại của cửa sổ rate
4. THE XDP_Filter SHALL phát hiện và drop gói tin bị lỗi bao gồm null TCP flags, SYN+FIN, SYN+RST, Christmas tree flags, IP total length không hợp lệ, và UDP length không khớp
5. WHEN bộ giới hạn tốc độ L7 của Client_WAF phát hiện IP vượt ngưỡng hard-block, THE Ban_Engine SHALL thêm IP vào cả L7 ban store và file blocklist XDP để thực thi ở mức kernel
6. THE Detection_Engine SHALL nhận diện công cụ tự động bằng khớp mẫu User-Agent và chặn ngay lập tức các yêu cầu từ công cụ tấn công đã biết bao gồm sqlmap, python-requests không có custom UA, libwww-perl, và chuỗi User-Agent rỗng
7. WHEN IP nguồn bị ban ở L7, THE Client_WAF SHALL đồng bộ ban đến blocklist XDP trong vòng 1 giây để các gói tin tiếp theo bị drop ở mức kernel

### Requirement 7: Bảo Vệ Chống Bypass

**User Story:** Là quản trị viên hệ thống, tôi muốn WAF chống lại các kỹ thuật bypass phổ biến, để kẻ tấn công không thể vượt qua các lớp bảo vệ.

#### Acceptance Criteria

1. THE Client_WAF SHALL xác thực rằng HMAC của cookie truy cập khớp với IP yêu cầu, ngăn chặn cookie replay từ địa chỉ nguồn khác
2. WHEN chế độ Cloudflare được bật, THE cấu hình Nginx của Master_Server SHALL từ chối kết nối HTTP/HTTPS từ IP không nằm trong dải IP Cloudflare, ngăn chặn bypass origin IP
3. THE XDP_Filter SHALL drop gói tin có IP nguồn giả mạo private (RFC 1918, loopback, link-local) đến trên giao diện public
4. THE Client_WAF SHALL triển khai giới hạn tốc độ per-IP và per-subnet độc lập để các cuộc tấn công phân tán từ một /24 bị throttle ngay cả khi các IP riêng lẻ nằm dưới ngưỡng per-IP
5. WHEN cookie challenge hết hạn, THE Client_WAF SHALL yêu cầu khách truy cập hoàn thành challenge mới thay vì cho phép truy cập thẳng đến backend

### Requirement 8: Trang Chủ và Giao Diện Công Khai

**User Story:** Là khách truy cập domain quản lý WAF, tôi muốn thấy trang chủ sản phẩm chuyên nghiệp không lộ chi tiết hệ thống nội bộ, để hệ thống trông đáng tin cậy và an toàn.

#### Acceptance Criteria

1. THE Homepage SHALL hiển thị thương hiệu sản phẩm, mô tả ngắn gọn về dịch vụ bảo vệ, và thông tin liên hệ mà không tiết lộ đường dẫn admin hoặc API endpoint
2. THE Homepage SHALL sử dụng thiết kế tông tối hiện đại nhất quán với nhận diện trực quan của Challenge_Page và Admin_Panel
3. THE Homepage SHALL tải trong dưới 2 giây trên kết nối 3G mà không có phụ thuộc bên ngoài
4. THE Homepage SHALL KHÔNG bao gồm bất kỳ JavaScript nào có thể bị khai thác để lộ thông tin về hệ thống backend

### Requirement 9: Ổn Định Hệ Thống và Xử Lý Lỗi

**User Story:** Là quản trị viên hệ thống, tôi muốn hệ thống WAF xử lý lỗi một cách graceful và duy trì khả dụng dịch vụ, để bảo vệ không bị gián đoạn bởi các lỗi tạm thời.

#### Acceptance Criteria

1. IF máy chủ backend không khả dụng, THEN THE Client_WAF SHALL trả về HTTP 502 Bad Gateway với trang lỗi có thương hiệu thay vì lộ thông tin nội bộ proxy
2. IF heartbeat license thất bại 3 lần liên tiếp, THEN THE Client_WAF SHALL vào chế độ khóa chặn tất cả lưu lượng ngoại trừ từ IP admin
3. WHEN Client_WAF vào chế độ khóa, THE Client_WAF SHALL ghi log lý do khóa và timestamp cho mục đích chẩn đoán
4. IF file blocklist XDP không thể ghi được, THEN THE Client_WAF SHALL ghi log lỗi và tiếp tục hoạt động chỉ với thực thi L7 mà không crash
5. THE Master_Server SHALL xử lý các yêu cầu API đồng thời mà không hỏng dữ liệu sử dụng chế độ SQLite WAL và cấu hình busy timeout phù hợp

### Requirement 10: Triển Khai Production và Cấu Trúc Dự Án

**User Story:** Là kỹ sư DevOps, tôi muốn cấu trúc dự án sạch sẽ, tổ chức tốt, và tối ưu cho triển khai production, để build có thể tái tạo và triển khai đáng tin cậy.

#### Acceptance Criteria

1. THE Master_Server SHALL có thể triển khai dưới dạng binary đơn với quản lý dịch vụ systemd, tự động khởi động lại khi lỗi, và endpoint kiểm tra health
2. THE Client_WAF SHALL có thể triển khai dưới dạng binary đơn với quản lý dịch vụ systemd và cấu hình qua biến môi trường và cờ dòng lệnh
3. THE script triển khai SHALL cài đặt tất cả thành phần, cấu hình Nginx reverse proxy, build XDP object, và xác minh health trong một lần chạy tự động trên Ubuntu 22.04
4. THE mã nguồn XDP_Filter SHALL biên dịch không có warning sử dụng clang với `-Wall -Werror` nhắm kiến trúc BPF
5. WHEN script triển khai hoàn tất, THE hệ thống SHALL có Master_Server, Client_WAF, Nginx, và XDP đều hoạt động và vượt qua kiểm tra health
