# Requirements Document

## Introduction

Đại tu toàn diện hệ thống Kiro WAF bao gồm nhận diện thương hiệu, giao diện frontend hiện đại, trực quan hóa dữ liệu động, cài đặt client thông minh, cập nhật OTA, tối ưu hiệu năng XDP/eBPF, tối ưu hiệu năng Golang WAF, tái cấu trúc thư mục, tài liệu dự án, tài liệu người dùng cuối, và xử lý lỗi/ngăn rò rỉ bộ nhớ. Hệ thống bao gồm Master Server (mặt phẳng quản lý/điều khiển tại firewall.vpsgen.com), Client Node (reverse proxy + lọc XDP/eBPF), Admin UI (HTML templates phục vụ bởi Go), và công cụ CLI.

## Glossary

- **Master_Server**: Máy chủ quản lý trung tâm tại firewall.vpsgen.com xử lý quản lý license, phân phối bản phát hành, giám sát heartbeat, và admin UI
- **Client_Node**: WAF reverse proxy biên được triển khai trên các máy chủ được bảo vệ, kết hợp Golang HTTP proxy với lọc gói tin XDP/eBPF
- **Admin_UI**: Giao diện quản trị dựa trên HTML được phục vụ bởi Master_Server để quản lý license, bản phát hành, và giám sát heartbeat
- **Install_Script**: Script bash (scripts/install-client.sh) tự động hóa triển khai Client_Node trên máy chủ đích
- **OTA_Updater**: Hệ thống con cập nhật qua mạng tự động giữ cho binary Client_Node luôn mới nhất mà không cần can thiệp thủ công
- **XDP_Filter**: Chương trình eBPF/XDP C (xdp_filter.c) thực hiện lọc gói tin L3/L4 ở tốc độ dây trong kernel
- **WAF_Proxy**: Thành phần reverse proxy Golang trong Client_Node xử lý kiểm tra lưu lượng L7, giới hạn tốc độ, thử thách, và chuyển tiếp yêu cầu
- **Brand_System**: Hệ thống nhận diện thị giác bao gồm logo, bảng màu, typography, và design token được sử dụng nhất quán trên tất cả giao diện UI
- **Chart_Engine**: Hệ thống con trực quan hóa dữ liệu phía client hiển thị thống kê, log, và metrics dưới dạng biểu đồ tương tác
- **Documentation_Site**: Tài liệu công khai phục vụ tại /docs chứa hướng dẫn sử dụng cho người dùng cuối
- **CLI_Tool**: Công cụ dòng lệnh kiro-cli cung cấp các lệnh quản trị và chẩn đoán hệ thống (version, license, status, health, preflight, mode, install, update, incident, pilot, report)
- **Health_Monitor**: Hệ thống con kiểm tra sức khỏe liên tục cho XDP_Filter và binary Client_Node, phát hiện sự cố và kích hoạt tự phục hồi
- **Package_Plan**: Gói dịch vụ do admin cấp cho người dùng, xác định mức độ bảo vệ và tính năng khả dụng (Community, Pro, Enterprise)
- **Install_UX**: Hệ thống giao diện người dùng trong Install_Script bao gồm progress bar, màu sắc, animation, và thông báo trạng thái trực quan

## Requirements

### Yêu Cầu 1: Hệ Thống Nhận Diện Thương Hiệu

**Câu Chuyện Người Dùng:** Là quản trị viên hệ thống, tôi muốn có nhận diện thương hiệu nhất quán theo phong cách cyber/bảo mật trên tất cả giao diện Kiro WAF, để sản phẩm trông chuyên nghiệp và đáng tin cậy.

#### Tiêu Chí Chấp Nhận

1. Brand_System PHẢI cung cấp một file logo SVG duy nhất hiển thị không bị pixel hóa hoặc cắt xén ở kích thước từ 16x16 pixel đến 512x512 pixel, chỉ sử dụng phần tử vector (không nhúng ảnh raster)
2. Brand_System PHẢI định nghĩa bảng màu dựa trên teal với tối thiểu các token CSS custom property sau: primary (#0d9488), accent, background, surface, text-primary, text-secondary, border, success, danger, và warning — mỗi token được gán giá trị hex hoặc rgba cụ thể
3. Brand_System PHẢI bao gồm tất cả CSS custom property design token trong một stylesheet hoặc style block duy nhất được tải trên mọi trang UI (trang chủ, trang admin, và trang challenge), có thể xác minh bằng sự hiện diện của custom property trên phần tử root của document
4. KHI Admin_UI tải, Brand_System PHẢI hiển thị logo SVG trong thanh điều hướng và áp dụng color token đã định nghĩa cho navigation, header trang, và footer trang sao cho không phần tử nào trong các vùng đó sử dụng giá trị màu ngoài tập token đã định nghĩa
5. Brand_System PHẢI bao gồm favicon được khai báo qua phần tử `<link rel="icon">` ở định dạng SVG, được tạo từ logo chính và có mặt trong HTML head của mọi trang UI

### Yêu Cầu 2: Giao Diện Frontend Hiện Đại với Dark Mode

**Câu Chuyện Người Dùng:** Là quản trị viên, tôi muốn giao diện UI dark-mode hiện đại với hiệu ứng glassmorphism và neon glow, để giao diện admin cảm thấy đương đại và giảm mỏi mắt khi sử dụng lâu.

#### Tiêu Chí Chấp Nhận

1. Admin_UI PHẢI hiển thị tất cả trang sử dụng nền tối với giá trị luminance bằng hoặc thấp hơn #1a1a2e cho body và phần tử surface, và text sáng với giá trị màu bằng hoặc cao hơn độ sáng #e0e0e0 cho body text
2. Admin_UI PHẢI áp dụng hiệu ứng glassmorphism cho các thành phần card và panel sử dụng backdrop-filter blur từ 8px đến 16px và background opacity từ 0.6 đến 0.9
3. Admin_UI PHẢI áp dụng hiệu ứng neon glow teal cho các nút hành động chính và mục navigation đang active sử dụng box-shadow với giá trị rgba teal, blur radius tối thiểu 4px, và spread tối thiểu 1px
4. Admin_UI PHẢI duy trì tỷ lệ tương phản WCAG 2.1 AA (tối thiểu 4.5:1 cho text thường, tối thiểu 3:1 cho text 18px trở lên) cho tất cả nội dung text so với nền trực tiếp của nó
5. KHI viewport width dưới 768 pixel, Admin_UI PHẢI chuyển nội dung sang layout một cột mà không cần cuộn ngang ở viewport width đó
6. Admin_UI PHẢI tải tất cả style từ một file CSS hợp nhất duy nhất, không phụ thuộc CDN bên ngoài, với tổng kích thước file không vượt quá 100KB chưa nén
7. NẾU trình duyệt không hỗ trợ thuộc tính CSS backdrop-filter, THÌ Admin_UI PHẢI fallback sang nền bán trong suốt đặc (opacity từ 0.85 đến 0.95) trên các thành phần card và panel để nội dung vẫn đọc được

### Yêu Cầu 3: Biểu Đồ Động và Trực Quan Hóa Dữ Liệu

**Câu Chuyện Người Dùng:** Là quản trị viên, tôi muốn biểu đồ tương tác hiển thị thống kê license, lịch sử cập nhật, xu hướng heartbeat, và log sự kiện DDoS, để tôi có thể giám sát sức khỏe hệ thống trong nháy mắt.

#### Tiêu Chí Chấp Nhận

1. KHI dashboard admin tải, Chart_Engine PHẢI hiển thị biểu đồ phân bố trạng thái license hiển thị số lượng license active, suspended, revoked, và expired dưới dạng các phân đoạn có nhãn riêng biệt
2. KHI dashboard admin tải, Chart_Engine PHẢI hiển thị biểu đồ timeline heartbeat cho thấy số heartbeat nhận được mỗi khoảng 1 giờ trong 24 giờ qua
3. KHI trang releases admin tải, Chart_Engine PHẢI hiển thị biểu đồ lịch sử cập nhật vẽ phiên bản release trên trục Y so với ngày tạo trên trục X
4. Chart_Engine PHẢI sử dụng thư viện biểu đồ JavaScript nhẹ (dưới 50KB gzipped) được bundle cục bộ mà không có yêu cầu CDN bên ngoài
5. KHI có dữ liệu mới qua refresh trang, Chart_Engine PHẢI cập nhật trực quan hóa biểu đồ để phản ánh dữ liệu hiện tại
6. Chart_Engine PHẢI hiển thị biểu đồ ở định dạng SVG hoặc Canvas co giãn tỷ lệ ở viewport width từ 320 pixel đến 2560 pixel với nhãn trục và giá trị dữ liệu hiển thị ở cỡ chữ không nhỏ hơn 10 pixel
7. KHI người dùng hover hoặc tap vào điểm dữ liệu biểu đồ, Chart_Engine PHẢI hiển thị tooltip cho thấy giá trị chính xác và nhãn cho điểm dữ liệu đó
8. NẾU không có dữ liệu cho biểu đồ, THÌ Chart_Engine PHẢI hiển thị thông báo placeholder cho biết không có dữ liệu thay vì hiển thị biểu đồ trống hoặc bị lỗi

### Yêu Cầu 4: Script Cài Đặt Client Thông Minh

**Câu Chuyện Người Dùng:** Là người vận hành máy chủ, tôi muốn script cài đặt tự động phát hiện OS, cài đặt dependency cần thiết, và triển khai binary Kiro WAF client, để tôi có thể thiết lập bảo vệ mà không cần quản lý dependency thủ công.

#### Tiêu Chí Chấp Nhận

1. KHI Install_Script thực thi, Install_Script PHẢI phát hiện bản phân phối hệ điều hành (Ubuntu, Debian, CentOS, Rocky, Fedora, Arch) và phiên bản
2. KHI OS được phát hiện không được hỗ trợ, Install_Script PHẢI thoát với mã thoát khác không và thông báo lỗi mô tả liệt kê các bản phân phối được hỗ trợ
3. NẾU Install_Script không được thực thi với quyền root, THÌ Install_Script PHẢI thoát với mã thoát khác không và hiển thị thông báo cho biết cần root hoặc sudo
4. KHI các dependency bắt buộc (curl, sha256sum, systemctl) bị thiếu, Install_Script PHẢI cài đặt chúng sử dụng trình quản lý gói của OS đã phát hiện
5. KHI chế độ XDP được yêu cầu, Install_Script PHẢI cài đặt dependency build XDP (clang, llvm, libbpf-dev) sử dụng trình quản lý gói của OS đã phát hiện
6. KHI tất cả dependency được thỏa mãn, Install_Script PHẢI tải binary client từ firewall.vpsgen.com sử dụng license key được cung cấp để xác thực, với timeout kết nối 30 giây, và xác minh checksum SHA-256 so với giá trị lấy từ endpoint thông tin release của server
7. NẾU xác minh checksum SHA-256 thất bại, THÌ Install_Script PHẢI hủy cài đặt, xóa file đã tải, và hiển thị giá trị checksum mong đợi so với thực tế
8. NẾU tải từ firewall.vpsgen.com thất bại do lỗi mạng hoặc license key không hợp lệ, THÌ Install_Script PHẢI thoát với mã thoát khác không và hiển thị thông báo cho biết lý do thất bại
9. Install_Script PHẢI là idempotent: khi thực thi nhiều lần trên cùng hệ thống, nó PHẢI thay thế binary hiện có chỉ khi phiên bản khác nhau, giữ nguyên file cấu hình hiện có, và để systemd service ở trạng thái enabled và running
10. KHI cài đặt hoàn tất, Install_Script PHẢI tạo và enable systemd service unit cho Client_Node, reload systemd daemon, và khởi động service
11. KHI Install_Script được thực thi trên hệ thống mà Client_Node service đang chạy, Install_Script PHẢI dừng service hiện có trước khi thay thế binary và khởi động lại sau khi cài đặt hoàn tất

### Yêu Cầu 5: Hệ Thống Cập Nhật OTA Tự Động

**Câu Chuyện Người Dùng:** Là người vận hành hệ thống, tôi muốn các client node tự động kiểm tra và áp dụng cập nhật từ master server, để các bản vá bảo mật được triển khai mà không cần can thiệp thủ công.

#### Tiêu Chí Chấp Nhận

1. OTA_Updater PHẢI kiểm tra cập nhật có sẵn bằng cách polling Master_Server ở khoảng thời gian có thể cấu hình (mặc định 300 giây, tối thiểu 60 giây, tối đa 86400 giây)
2. KHI Master_Server gửi thông báo push qua phản hồi heartbeat, OTA_Updater PHẢI khởi tạo kiểm tra cập nhật ngay lập tức
3. KHI có cập nhật, OTA_Updater PHẢI tải binary mới trực tiếp từ firewall.vpsgen.com trong timeout 5 phút và xác minh checksum SHA-256
4. NẾU checksum binary đã tải không khớp giá trị mong đợi, THÌ OTA_Updater PHẢI hủy bản tải và ghi log lỗi mà không sửa đổi binary đang chạy
5. NẾU tải thất bại do lỗi mạng hoặc timeout, THÌ OTA_Updater PHẢI hủy mọi bản tải một phần, ghi log thất bại, và thử lại ở khoảng polling tiếp theo mà không sửa đổi binary đang chạy
6. KHI checksum hợp lệ, OTA_Updater PHẢI thực hiện thay thế binary nguyên tử sử dụng rename(2) để hoán đổi binary mới vào vị trí
7. KHI binary mới không đạt trạng thái systemd active trong 30 giây sau khi restart service, OTA_Updater PHẢI tự động rollback về phiên bản binary trước đó và restart service với binary đã khôi phục
8. OTA_Updater PHẢI giữ lại chính xác một phiên bản binary trước đó cho mục đích rollback
9. OTA_Updater PHẢI ghi log tất cả thao tác cập nhật (kiểm tra, tải, xác minh, thay thế, rollback) vào system journal
10. NẾU Master_Server không thể truy cập trong lần poll theo lịch, THÌ OTA_Updater PHẢI ghi log lỗi kết nối và thử lại ở khoảng polling tiếp theo mà không thay đổi binary đang chạy

### Yêu Cầu 6: Tối Ưu Hiệu Năng XDP/eBPF

**Câu Chuyện Người Dùng:** Là kỹ sư mạng, tôi muốn bộ lọc gói tin XDP xử lý 10 triệu gói tin mỗi giây với độ trễ dưới 100 nano giây mỗi gói, để giảm thiểu DDoS không trở thành nút thắt cổ chai.

#### Tiêu Chí Chấp Nhận

1. XDP_Filter PHẢI sử dụng BPF_MAP_TYPE_PERCPU_ARRAY cho bộ đếm thống kê để loại bỏ tranh chấp lock giữa các CPU
2. XDP_Filter PHẢI sử dụng BPF_MAP_TYPE_LRU_HASH với dung lượng tối đa 262,144 entry cho trạng thái rate per-IP và per-subnet để tránh logic eviction thủ công
3. XDP_Filter PHẢI xử lý một gói tin 64-byte kích thước tối thiểu trong dưới 100 nano giây trung bình trên CPU x86_64 chạy ở 3.0 GHz trở lên (đo qua delta bpf_ktime_get_ns trên 1,000,000 gói tin)
4. XDP_Filter PHẢI duy trì throughput 10 triệu gói tin 64-byte mỗi giây trên một CPU core đơn ở 3.0 GHz trở lên khi gắn ở chế độ XDP native
5. XDP_Filter PHẢI tránh cấp phát bộ nhớ động trong đường dẫn chương trình XDP
6. XDP_Filter PHẢI biên dịch với clang -O2 optimization và tạo ra BPF object hợp lệ dưới 32,768 byte kích thước
7. KHI blocklist map (dung lượng tối đa 65,536 entry) đạt dung lượng tối đa, XDP_Filter PHẢI tiếp tục xử lý gói tin sử dụng các entry hiện có mà không crash hoặc trả về XDP_ABORTED
8. KHI LRU rate state map đạt dung lượng tối đa, XDP_Filter PHẢI evict entry ít được sử dụng gần đây nhất và tiếp tục rate limiting cho IP nguồn mới mà không trả về XDP_ABORTED
9. KHI nhận gói tin không phải IPv4, XDP_Filter PHẢI trả về XDP_PASS mà không thực hiện bất kỳ kiểm tra lọc hoặc rate limiting nào

### Yêu Cầu 7: Tối Ưu Hiệu Năng Golang WAF

**Câu Chuyện Người Dùng:** Là kỹ sư nền tảng, tôi muốn WAF reverse proxy xử lý 100,000 request mỗi giây với mức sử dụng bộ nhớ tối thiểu và GC pause thấp, để lưu lượng hợp lệ không bị suy giảm trong các cuộc tấn công.

#### Tiêu Chí Chấp Nhận

1. WAF_Proxy PHẢI xử lý 100,000 HTTP request mỗi giây trên server 4-core với độ trễ phản hồi dưới 5 mili giây ở p99, đo sử dụng request body 1KB qua kết nối HTTP/1.1 persistent với backend phản hồi trong 1 mili giây
2. WAF_Proxy PHẢI tiêu thụ dưới 512 megabyte bộ nhớ RSS dưới tải 100,000 request mỗi giây duy trì ít nhất 60 giây
3. WAF_Proxy PHẢI duy trì thời gian pause garbage collection Go dưới 1 mili giây (đo qua runtime/metrics) dưới tải 100,000 request mỗi giây
4. WAF_Proxy PHẢI sử dụng pattern zero-allocation trong đường dẫn request nóng (không cấp phát heap mỗi request cho kiểm tra header và routing, xác minh qua Go benchmark báo cáo 0 allocs/op)
5. WAF_Proxy PHẢI sử dụng connection pooling cho kết nối backend với số kết nối idle tối đa có thể cấu hình (mặc định 256) và tổng số kết nối tối đa có thể cấu hình (mặc định 1024)
6. WAF_Proxy PHẢI sử dụng sync.Pool cho buffer tái sử dụng trong đường dẫn chuyển tiếp proxy
7. KHI backend không thể truy cập, WAF_Proxy PHẢI trả về phản hồi 502 trong 5 giây mà không rò rỉ goroutine hoặc kết nối
8. NẾU tất cả kết nối backend trong pool đang được sử dụng và giới hạn tổng kết nối tối đa đã đạt, THÌ WAF_Proxy PHẢI xếp hàng request tối đa 1 giây và trả về phản hồi 503 nếu không có kết nối nào khả dụng trong khoảng thời gian đó
9. KHI WAF_Proxy khởi động, WAF_Proxy PHẢI thiết lập GOGC và GOMEMLIMIT dựa trên environment để duy trì mục tiêu GC pause dưới tải

### Yêu Cầu 8: Tái Cấu Trúc Thư Mục theo Standard Go Layout

**Câu Chuyện Người Dùng:** Là developer, tôi muốn dự án được tổ chức theo standard Go project layout, để codebase dễ điều hướng và tuân theo quy ước cộng đồng.

#### Tiêu Chí Chấp Nhận

1. Mã nguồn Master_Server PHẢI nằm dưới cmd/kiro-master/ cho entry point (main.go) và internal/master/ cho các package private
2. Mã nguồn Client_Node PHẢI nằm dưới cmd/kiro-client/ cho entry point (main.go) và internal/client/ cho các package private
3. Mã nguồn C và build script của XDP_Filter PHẢI nằm dưới internal/client/xdp/
4. Các package dùng chung bởi cả Master_Server và Client_Node PHẢI nằm dưới pkg/ với API exported tuân theo quy ước Go module versioning và không import từ internal/
5. Web asset (CSS, JavaScript, HTML template) PHẢI nằm dưới web/templates/ và web/static/
6. Cấu hình triển khai (systemd, nginx, nftables, sysctl) PHẢI nằm dưới deployments/
7. Dự án PHẢI duy trì cấu hình go.work hoặc go.mod cho phép build cmd/kiro-master/, cmd/kiro-client/, và cmd/kiro-cli/ từ thư mục gốc repository sử dụng lệnh go build tiêu chuẩn
8. KHI tái cấu trúc hoàn tất, dự án PHẢI build tất cả Go binary (kiro-master, kiro-client, kiro-cli) thành công sử dụng một lệnh make build duy nhất, tạo ra file thực thi trong thư mục build output với mã thoát bằng không và không có lỗi biên dịch
9. NẾU mã nguồn C XDP_Filter cần biên dịch, THÌ dự án PHẢI cung cấp make target riêng (make build-xdp) biên dịch mã nguồn XDP C, để build Go binary không phụ thuộc vào toolchain clang/llvm
10. KHI file mã nguồn được di chuyển sang cấu trúc thư mục mới, dự án PHẢI cập nhật tất cả import path nội bộ để không còn tham chiếu import bị hỏng

### Yêu Cầu 9: README và Tài Liệu Dự Án

**Câu Chuyện Người Dùng:** Là contributor, tôi muốn README toàn diện và tài liệu kiến trúc với badge và diagram, để tôi có thể hiểu hệ thống nhanh chóng.

#### Tiêu Chí Chấp Nhận

1. README PHẢI hiển thị badge CI status (GitHub Actions workflow badge), Go version, và license là các phần tử nội dung đầu tiên của tài liệu
2. README PHẢI bao gồm Mermaid architecture diagram cho thấy mối quan hệ giữa Master_Server, Client_Node, XDP_Filter, Cloudflare, và SQLite
3. README PHẢI bao gồm Mermaid sequence diagram cho thấy luồng heartbeat polling và luồng OTA update (kiểm tra, tải, xác minh, thay thế, rollback)
4. README PHẢI chứa các phần theo thứ tự: overview, architecture, quick start, configuration, deployment, và contributing, trong đó phần quick start cho phép người đọc build tất cả binary và chạy test từ bản clone sạch trong 10 phút
5. Documentation_Site PHẢI bao gồm tài liệu kiến trúc (docs/architecture.md) mô tả mỗi trong bốn thành phần cốt lõi (Master_Server, Client_Node, XDP_Filter, CLI_Tool) với tóm tắt trách nhiệm và giải thích luồng dữ liệu bao gồm đường đi request từ ingress đến backend
6. Documentation_Site PHẢI bao gồm hướng dẫn thiết lập phát triển (docs/development.md) liệt kê tất cả prerequisite build (Go version, clang, llvm, libbpf-dev, make), các bước biên dịch tất cả binary, và lệnh chạy toàn bộ test suite
7. KHI Mermaid diagram được bao gồm trong README hoặc docs/architecture.md, diagram PHẢI sử dụng cú pháp Mermaid hợp lệ hiển thị không lỗi trong GitHub Markdown renderer

### Yêu Cầu 10: Tài Liệu Công Khai Người Dùng Cuối

**Câu Chuyện Người Dùng:** Là người dùng cuối, tôi muốn tài liệu công khai tại /docs giải thích cách sử dụng Kiro WAF mà không lộ chi tiết triển khai nội bộ, để tôi có thể cấu hình và vận hành hệ thống độc lập.

#### Tiêu Chí Chấp Nhận

1. Documentation_Site PHẢI được phục vụ tại đường dẫn URL /docs trên Master_Server dưới dạng trang HTML tĩnh với sidebar điều hướng cố định hoặc mục lục liệt kê tất cả phần
2. Documentation_Site PHẢI chứa tối thiểu: hướng dẫn cài đặt, tham chiếu cấu hình bao gồm tất cả tùy chọn YAML người dùng trong kiro.example.yaml, phần xử lý sự cố với ít nhất 10 kịch bản lỗi phổ biến và cách giải quyết, và phần FAQ
3. Documentation_Site KHÔNG ĐƯỢC lộ endpoint API nội bộ, schema database, đường dẫn mã nguồn, hoặc chi tiết triển khai bảo mật
4. Documentation_Site PHẢI bao gồm hướng dẫn quick-start cho phép người dùng có kinh nghiệm dòng lệnh Linux cơ bản cài đặt và cấu hình Client_Node trên OS được hỗ trợ trong 15 phút, đo từ tải script đến systemd service đã xác minh đang chạy
5. KHI tài liệu tham chiếu tùy chọn cấu hình, Documentation_Site PHẢI cung cấp giá trị ví dụ hợp lệ, kiểu dữ liệu của tùy chọn, giá trị mặc định, phạm vi chấp nhận hoặc giá trị cho phép, và mô tả một câu về tác dụng của tùy chọn cho mọi tùy chọn cấu hình người dùng
6. Documentation_Site PHẢI có sẵn bằng cả tiếng Việt và tiếng Anh với nút chuyển ngôn ngữ hiển thị trên mọi trang
7. NẾU người dùng yêu cầu đường dẫn /docs và nội dung tài liệu không khả dụng, THÌ Master_Server PHẢI trả về trang lỗi cho biết tài liệu tạm thời không khả dụng thay vì lỗi server chung
8. KHI bản phát hành Client_Node mới được xuất bản, Documentation_Site PHẢI hiển thị phiên bản tài liệu hoặc ngày cập nhật cuối trên mỗi trang để người dùng có thể xác minh độ mới của nội dung

### Yêu Cầu 11: Xử Lý Lỗi và Ngăn Rò Rỉ Bộ Nhớ

**Câu Chuyện Người Dùng:** Là kỹ sư độ tin cậy, tôi muốn xử lý lỗi mạnh mẽ và ngăn rò rỉ bộ nhớ trên tất cả thành phần, để hệ thống chạy ổn định hàng tháng mà không suy giảm.

#### Tiêu Chí Chấp Nhận

1. KHI nhận HTTP response body, WAF_Proxy PHẢI đóng response body trong cùng phạm vi hàm sử dụng defer
2. WAF_Proxy PHẢI áp dụng read timeout 30 giây và write timeout 60 giây trên tất cả kết nối HTTP client
3. WAF_Proxy PHẢI giới hạn goroutine đồng thời ở mức tối đa có thể cấu hình (mặc định 10,000) sử dụng pattern semaphore
4. KHI goroutine panic, WAF_Proxy PHẢI recover panic, ghi log stack trace, và tiếp tục phục vụ các request khác
5. Master_Server PHẢI áp dụng read timeout 30 giây và write timeout 60 giây trên tất cả kết nối HTTP server
6. KHI database query vượt quá 5 giây, Master_Server PHẢI hủy query context và trả về phản hồi HTTP 503 với body JSON chứa trường error cho biết query đã timeout
7. Client_Node PHẢI chạy cleanup các rate-limit entry hết hạn mỗi 120 giây và challenge token hết hạn mỗi 60 giây để ngăn tăng trưởng bộ nhớ không giới hạn
8. NẾU giá trị cấu hình bắt buộc (license key, cookie secret, backend URL, hoặc master URL) bị thiếu hoặc rỗng khi khởi động, THÌ Client_Node PHẢI ghi log thông báo lỗi xác định giá trị bị thiếu và thoát với mã trạng thái khác không
9. NẾU giới hạn goroutine đồng thời của WAF_Proxy đã đạt, THÌ WAF_Proxy PHẢI từ chối request đến với phản hồi HTTP 503 cho đến khi có slot goroutine khả dụng


### Yêu Cầu 12: CLI Commands Hoạt Động Đầy Đủ và Tài Liệu Website

**Câu Chuyện Người Dùng:** Là quản trị viên hệ thống, tôi muốn tất cả lệnh CLI hoạt động 100% với test đầy đủ và website hiển thị hướng dẫn chi tiết cho từng lệnh, để tôi có thể quản lý hệ thống hiệu quả và tra cứu cách sử dụng nhanh chóng.

#### Tiêu Chí Chấp Nhận

1. CLI_Tool PHẢI cung cấp lệnh `version` trả về chuỗi phiên bản build hợp lệ theo định dạng semver (X.Y.Z) với mã thoát bằng không
2. CLI_Tool PHẢI cung cấp lệnh `license fingerprint` tạo machine fingerprint hash duy nhất cho mỗi máy chủ, chấp nhận tham số tùy chọn --salt và trả về chuỗi hash hex hợp lệ (64 ký tự hex lowercase)
3. CLI_Tool PHẢI cung cấp lệnh `status` chấp nhận tham số --config và trả về JSON chứa trạng thái runtime hiện tại bao gồm các trường: mode (server hoặc full), uptime, license status, và phiên bản hiện tại
4. CLI_Tool PHẢI cung cấp lệnh `health` chấp nhận tham số --config, --os-release, --preflight-writable-root, --skip-command-checks và trả về JSON chứa kết quả kiểm tra sức khỏe tổng hợp bao gồm trạng thái service (active/inactive), kết quả preflight, và overall status (healthy, degraded, hoặc unhealthy)
5. CLI_Tool PHẢI cung cấp lệnh `preflight` chấp nhận tham số --config, --os-release, --preflight-writable-root, --skip-command-checks và trả về JSON chứa kết quả kiểm tra điều kiện tiên quyết bao gồm OS compatibility (Ubuntu 22.04/24.04), quyền root (UID 0), và command availability (nft, nginx, systemctl)
6. CLI_Tool PHẢI cung cấp lệnh `mode show` hiển thị chế độ hoạt động hiện tại và lệnh `mode set --mode <value>` thay đổi chế độ hoạt động, NẾU giá trị --mode không phải "server" hoặc "full", THÌ CLI_Tool PHẢI hiển thị thông báo lỗi cho biết giá trị không hợp lệ và thoát với mã thoát 1
7. CLI_Tool PHẢI cung cấp lệnh `install plan` trả về JSON mô tả kế hoạch cài đặt, lệnh `install stage-lab` thực hiện staging vào thư mục --install-root, và lệnh `install apply-lab` áp dụng cài đặt yêu cầu tham số --ack với giá trị chính xác "KIRO_LAB_INSTALL_APPLY" để xác nhận thực thi
8. CLI_Tool PHẢI cung cấp lệnh `update check` kiểm tra bản cập nhật từ Master_Server (yêu cầu --master-url), lệnh `update apply` tải và áp dụng bản cập nhật với xác minh SHA-256 (yêu cầu --master-url, --binary-path, --service), và lệnh `update rollback` khôi phục phiên bản trước từ file .bak (yêu cầu --binary-path, --service)
9. CLI_Tool PHẢI cung cấp lệnh `incident report` tạo báo cáo sự cố với các tham số --type (attack, lost_ssh, update_failed, origin_ip_leaked, license_rebind, runtime_security, other), --severity, --status, --summary và lưu kết quả dưới dạng JSON và Markdown vào thư mục --output-dir
10. CLI_Tool PHẢI cung cấp lệnh `pilot report` tạo báo cáo pilot với các tham số --server-count, --started-at (RFC3339), --ended-at (RFC3339) và tổng hợp evidence từ --health-file, --benchmark-file, --incident-dir, trả về kết quả go/no-go dưới dạng JSON và Markdown
11. CLI_Tool PHẢI cung cấp lệnh `report` chấp nhận tham số --config và trả về JSON chứa báo cáo tổng hợp hệ thống bao gồm thông tin phiên bản, cấu hình runtime, và trạng thái các thành phần
12. KHI CLI_Tool nhận lệnh không hợp lệ hoặc không có tham số, CLI_Tool PHẢI hiển thị thông báo usage liệt kê tất cả lệnh khả dụng và thoát với mã thoát 2
13. NẾU tham số bắt buộc bị thiếu cho bất kỳ lệnh nào (ví dụ: --master-url cho update check, --binary-path cho update apply, --service cho update apply/rollback), THÌ CLI_Tool PHẢI hiển thị thông báo lỗi chỉ rõ tên tham số bị thiếu và thoát với mã thoát 2
14. Documentation_Site PHẢI hiển thị trang danh sách tất cả lệnh CLI (tối thiểu 11 lệnh chính) với cú pháp sử dụng, danh sách tham số bắt buộc và tùy chọn kèm giá trị mặc định, và ít nhất 1 ví dụ sử dụng thực tế cho mỗi lệnh
15. KHI người dùng truy cập trang CLI documentation trên website, Documentation_Site PHẢI hiển thị mục lục có thể điều hướng nhanh đến từng lệnh và hỗ trợ tìm kiếm theo tên lệnh
16. CLI_Tool PHẢI có unit test và integration test cho mỗi lệnh, đảm bảo coverage tối thiểu 80% cho package cmd/kiro-cli và các sub-package liên quan
17. NẾU lệnh `update apply` hoàn tất thay thế binary nhưng health check thất bại trong vòng 30 giây, THÌ CLI_Tool PHẢI tự động rollback về phiên bản trước (khôi phục file .bak) và restart service, sau đó thoát với mã thoát 1 kèm thông báo lỗi cho biết đã rollback
18. NẾU lệnh `install apply-lab` được gọi mà giá trị --ack không khớp "KIRO_LAB_INSTALL_APPLY" hoặc không có quyền root (UID != 0), THÌ CLI_Tool PHẢI từ chối thực thi và thoát với mã thoát 1 kèm thông báo lỗi cụ thể

### Yêu Cầu 13: Kiểm Tra Liên Tục XDP và Binary với Tự Phục Hồi

**Câu Chuyện Người Dùng:** Là kỹ sư vận hành, tôi muốn hệ thống liên tục kiểm tra sức khỏe XDP filter và binary client, tự động phục hồi khi bị DDoS sập hoặc mất mạng, để server được bảo vệ liên tục mà không cần can thiệp thủ công.

#### Tiêu Chí Chấp Nhận

1. Health_Monitor PHẢI kiểm tra trạng thái XDP_Filter mỗi 10 giây bằng cách đọc BPF map statistics và xác minh chương trình XDP vẫn được gắn vào network interface
2. Health_Monitor PHẢI kiểm tra trạng thái binary Client_Node mỗi 10 giây bằng cách gọi endpoint health check nội bộ (/__kiro/health) và xác minh phản hồi HTTP 200 trong timeout 5 giây
3. KHI XDP_Filter bị detach khỏi network interface hoặc BPF program không còn hoạt động, Health_Monitor PHẢI tự động reload và reattach chương trình XDP trong vòng 30 giây
4. KHI binary Client_Node không phản hồi health check trong 3 lần liên tiếp (30 giây), Health_Monitor PHẢI tự động restart service qua systemctl và ghi log sự kiện recovery
5. NẾU Client_Node crash trong khi traffic đến vượt 5x ngưỡng rate-limit cấu hình hiện tại (đo trong cửa sổ 10 giây trước crash), THÌ Health_Monitor PHẢI kích hoạt chế độ emergency recovery: restart service với cấu hình rate-limit nghiêm ngặt hơn (giảm 50% ngưỡng cho phép) trong 5 phút đầu sau recovery, sau đó tự động khôi phục ngưỡng ban đầu
6. NẾU Client_Node mất kết nối đến Master_Server quá 60 giây, THÌ Health_Monitor PHẢI chuyển sang chế độ offline operation: tiếp tục bảo vệ với cấu hình cached cuối cùng và thử reconnect mỗi 30 giây với exponential backoff (tối đa 5 phút giữa các lần thử)
7. KHI kết nối đến Master_Server được khôi phục sau giai đoạn offline, Health_Monitor PHẢI đồng bộ lại cấu hình mới nhất từ Master_Server, gửi báo cáo về thời gian offline (thời điểm bắt đầu, thời điểm kết thúc, số lần thử reconnect), và chuyển từ chế độ offline operation về chế độ online với cấu hình mới đồng bộ
8. Health_Monitor PHẢI bảo vệ dữ liệu rate-limit và session state bằng cách ghi snapshot ra disk mỗi 60 giây với kích thước tối đa 64MB mỗi file snapshot
9. NẾU restart service thất bại 3 lần liên tiếp trong vòng 5 phút, THÌ Health_Monitor PHẢI gửi alert đến Master_Server (nếu có kết nối) và ghi log critical error, sau đó chờ 60 giây trước khi thử lại
10. Package_Plan PHẢI xác định mức độ bảo vệ cho mỗi gói: số lượng rule tối đa, ngưỡng rate-limit, và tính năng XDP khả dụng — Health_Monitor PHẢI enforce giới hạn theo Package_Plan được cấp từ admin và từ chối áp dụng cấu hình vượt quá giới hạn của Package_Plan hiện tại
11. KHI Health_Monitor phát hiện tấn công DDoS (traffic đến vượt 10x ngưỡng rate-limit cấu hình hiện tại của Package_Plan, đo trung bình trong cửa sổ trượt 10 giây), Health_Monitor PHẢI tự động kích hoạt XDP_Filter ở chế độ strict mode và thông báo Master_Server về sự kiện tấn công trong vòng 5 giây
12. NẾU reload hoặc reattach XDP_Filter thất bại 3 lần liên tiếp, THÌ Health_Monitor PHẢI ghi log critical error, gửi alert đến Master_Server (nếu có kết nối), và tiếp tục thử reload mỗi 60 giây cho đến khi thành công
13. NẾU ghi snapshot ra disk thất bại (do lỗi I/O hoặc hết dung lượng), THÌ Health_Monitor PHẢI ghi log cảnh báo, giữ lại snapshot thành công gần nhất, và thử ghi lại ở chu kỳ tiếp theo (60 giây sau) mà không dừng hoạt động bảo vệ

### Yêu Cầu 14: Gói Community Không Giới Hạn Thời Gian

**Câu Chuyện Người Dùng:** Là người dùng miễn phí, tôi muốn gói Community luôn hoạt động vĩnh viễn mà không bị khóa khi hết hạn, và có thể nâng cấp lên gói cao hơn bất kỳ lúc nào, để tôi yên tâm sử dụng dịch vụ bảo vệ cơ bản lâu dài.

#### Tiêu Chí Chấp Nhận

1. KHI người dùng đăng ký mới, Master_Server PHẢI cấp license gói Community với thời hạn vô thời hạn (expiry_date = null hoặc giá trị đặc biệt "never") và trạng thái active
2. KHI license gói Pro hoặc Enterprise đạt thời điểm hết hạn (ExpiresAt), Master_Server PHẢI tự động chuyển license về gói Community với trạng thái downgraded trong vòng 60 giây kể từ thời điểm hết hạn, thay vì khóa hoặc vô hiệu hóa license
3. KHI license được chuyển từ gói cao hơn về Community, Master_Server PHẢI giữ nguyên machine fingerprint, license key, và lịch sử sử dụng — chỉ thay đổi mức Package_Plan, đặt expiry_date thành vô thời hạn, và vô hiệu hóa các tính năng vượt giới hạn Community (domain bảo vệ giảm về tối đa 1, XDP bị tắt, OTA bị tắt)
4. Master_Server PHẢI cho phép nâng cấp từ gói Community lên Pro hoặc Enterprise bất kỳ lúc nào mà không yêu cầu tạo license mới hoặc cài đặt lại Client_Node — license key và machine fingerprint được giữ nguyên, chỉ cập nhật Package_Plan, tính năng khả dụng, và expiry_date mới
5. KHI Client_Node nhận phản hồi heartbeat với Package_Plan = Community, Client_Node PHẢI tiếp tục hoạt động bình thường với tập tính năng Community (bảo vệ DDoS cơ bản, rate-limiting mặc định 60 request/phút/IP, challenge page) mà không hiển thị cảnh báo hết hạn hoặc giảm chức năng
6. NẾU Master_Server không thể truy cập và license cached là gói Community, THÌ Client_Node PHẢI tiếp tục hoạt động với cấu hình Community cached mà không tự vô hiệu hóa, cho đến khi Master_Server trở lại khả dụng hoặc service bị dừng thủ công
7. Master_Server PHẢI phân biệt rõ ràng giữa trạng thái license: active (đang hoạt động), suspended (bị tạm ngưng do vi phạm), và downgraded (đã hạ cấp từ gói cao hơn) — trạng thái suspended là trạng thái duy nhất ngăn Client_Node hoạt động
8. Admin_UI PHẢI hiển thị trên trang chi tiết license mà không cần cuộn: trạng thái gói hiện tại (Community/Pro/Enterprise), trạng thái license (active/suspended/downgraded), ngày hết hạn gói cao cấp (nếu có), và nút nâng cấp cho license đang ở gói Community
9. NẾU license có trạng thái suspended và đồng thời đạt thời điểm hết hạn gói Pro hoặc Enterprise, THÌ Master_Server PHẢI giữ nguyên trạng thái suspended mà không chuyển về Community — chỉ admin mới có thể gỡ trạng thái suspended
10. KHI Client_Node nhận phản hồi heartbeat cho biết license có trạng thái suspended, THÌ Client_Node PHẢI ngừng xử lý traffic và trả về trang thông báo cho biết dịch vụ bị tạm ngưng cho tất cả request đến

### Yêu Cầu 15: Cải Thiện Trải Nghiệm Cài Đặt (Install UX)

**Câu Chuyện Người Dùng:** Là người vận hành máy chủ, tôi muốn quá trình cài đặt có giao diện đẹp với progress bar, màu sắc rõ ràng, và thông báo trạng thái chi tiết, để tôi biết chính xác tiến trình cài đặt và cảm thấy tự tin khi triển khai.

#### Tiêu Chí Chấp Nhận

1. Install_Script PHẢI hiển thị banner ASCII art logo Kiro WAF với màu teal/cyan khi bắt đầu cài đặt, bao gồm phiên bản script và URL master server
2. Install_Script PHẢI hiển thị progress bar dạng thanh ngang (ví dụ: [████████░░░░░░░░] 50%) có chiều rộng tối thiểu 20 ký tự cho mỗi bước tải file, cập nhật ít nhất mỗi 1 giây hoặc mỗi 2% tiến trình (tùy điều kiện nào đến trước) dựa trên kích thước đã tải so với tổng kích thước
3. Install_Script PHẢI hiển thị spinner animation (ký tự xoay ⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏) với tốc độ 80-120ms mỗi frame cho các bước xử lý không xác định thời gian (phát hiện OS, cài đặt dependency, tạo service)
4. Install_Script PHẢI sử dụng mã màu nhất quán: xanh lá (✓) cho thành công, đỏ (✗) cho lỗi, vàng (⚠) cho cảnh báo, cyan (→) cho thông tin tiến trình, và trắng đậm cho tiêu đề bước
5. Install_Script PHẢI hiển thị số thứ tự bước dạng [N/T] (trong đó T là tổng số bước thực tế của luồng cài đặt hiện tại) trước mỗi giai đoạn cài đặt để người dùng biết tiến trình tổng thể
6. KHI mỗi bước hoàn tất, Install_Script PHẢI hiển thị thời gian thực hiện bước đó tính bằng giây với 1 chữ số thập phân (ví dụ: "✓ Tải binary hoàn tất (3.2s)") và tổng thời gian đã trôi qua kể từ khi script bắt đầu
7. KHI cài đặt hoàn tất thành công, Install_Script PHẢI hiển thị bảng tóm tắt có viền (box-drawing characters ┌─┐│└─┘) chứa: phiên bản đã cài, đường dẫn binary, trạng thái service (đang chạy/dừng), IP server, và danh sách lệnh hữu ích (status, log, restart, stop)
8. NẾU bất kỳ bước nào thất bại, THÌ Install_Script PHẢI dừng spinner hoặc progress bar hiện tại, hiển thị thông báo lỗi với màu đỏ bao gồm tên bước thất bại, mô tả nguyên nhân có thể, và gợi ý hành động khắc phục liên quan đến bước đó (ví dụ: "Kiểm tra kết nối mạng" cho lỗi tải, "Xác minh license key" cho lỗi xác thực)
9. Install_Script PHẢI hỗ trợ tham số --quiet hoặc -q để tắt tất cả animation (spinner, progress bar) và escape code màu, chỉ hiển thị mỗi dòng kết quả bước (thành công/thất bại) dạng text thuần cho môi trường CI/CD hoặc pipe
10. NẾU terminal không hỗ trợ màu sắc (biến TERM=dumb hoặc output không phải TTY), THÌ Install_Script PHẢI tự động fallback sang output text thuần không có escape code màu và không có animation, giữ nguyên nội dung thông báo trạng thái của mỗi bước
11. NẾU progress bar hoặc spinner đang hiển thị khi xảy ra lỗi, THÌ Install_Script PHẢI xóa dòng animation hiện tại trước khi in thông báo lỗi để tránh output bị chồng chéo
