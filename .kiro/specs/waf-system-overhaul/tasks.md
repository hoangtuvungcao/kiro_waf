# Implementation Plan: WAF System Overhaul

## Overview

Triển khai toàn diện hệ thống kiro_waf bao gồm Master_Server, Client_WAF, XDP_Filter, CLI_Tool, cấu hình Nginx, và script triển khai. Mỗi task xây dựng trên các task trước đó, kết thúc bằng việc kết nối tất cả thành phần và kiểm tra tích hợp. Ngôn ngữ triển khai: Go (với C cho XDP_Filter).

## Tasks

- [x] 1. Thiết lập cấu trúc dự án và interfaces cốt lõi
  - [x] 1.1 Tạo cấu trúc thư mục dự án và Go modules
    - Tạo `master-server/` với `main.go`, `handlers/`, `models/`, `db/`, `templates/`
    - Tạo `client-node/` với `client_waf.go`, `challenge/`, `ratelimit/`, `ban/`, `cookie/`, `ua/`
    - Tạo `cmd/kiro-cli/` với `main.go`, `update/`, `status/`
    - Tạo `tests/property/`, `tests/unit/`, `tests/integration/`, `tests/smoke/`
    - Khởi tạo Go module với `go mod init` và thêm dependencies: `rapid`, `mattn/go-sqlite3`
    - _Requirements: 10.1, 10.2_

  - [x] 1.2 Định nghĩa data models và interfaces Go
    - Tạo `master-server/models/models.go` với structs: License, Release, Heartbeat, AdminSession, AdminLoginAttempt
    - Tạo `client-node/models.go` với structs: Challenge, BanEntry, XDPConfig
    - Tạo `client-node/cookie/cookie.go` với interface cho HMAC cookie operations
    - Tạo `client-node/ratelimit/ratelimit.go` với interface cho rate limiter
    - Tạo `client-node/ban/ban.go` với interface cho ban engine
    - _Requirements: 1.3, 6.5, 7.1_

- [x] 2. Triển khai Master_Server - Database và API cốt lõi
  - [x] 2.1 Triển khai lớp database SQLite với WAL mode
    - Tạo `master-server/db/sqlite.go` với khởi tạo database, migration schema
    - Cấu hình PRAGMA: journal_mode=WAL, busy_timeout=5000, synchronous=NORMAL, foreign_keys=ON, cache_size=-8000
    - Tạo tất cả bảng: licenses, releases, heartbeats, admin_login_attempts, admin_sessions
    - Tạo indexes cho performance
    - Implement CRUD operations cho License: Create, Read, Update, Delete, Renew, Rotate, Revoke
    - Implement CRUD operations cho Release: Create, Read, Delete
    - Implement Heartbeat logging
    - _Requirements: 9.5, 4.2, 4.3_

  - [x] 2.2 Viết unit tests cho database layer
    - Test concurrent writes với WAL mode
    - Test CRUD operations cho licenses
    - Test CRUD operations cho releases
    - Test heartbeat logging
    - _Requirements: 9.5_

  - [x] 2.3 Triển khai API endpoints: heartbeat và update check
    - Tạo `master-server/handlers/api.go`
    - Implement `POST /api/v1/heartbeat`: validate license key, log heartbeat, trả về trạng thái license
    - Implement `POST /api/v1/update/check`: lookup latest release theo component+channel, trả về metadata
    - Implement `GET /healthz`: health check endpoint
    - Xử lý lỗi: invalid payload → 400, invalid license → 200 với `valid: false, lock: true`
    - _Requirements: 5.1, 9.2, 9.5_

  - [x] 2.4 Viết unit tests cho API handlers
    - Test heartbeat với valid/invalid license key
    - Test update check với/không có bản phát hành mới
    - Test health check endpoint
    - _Requirements: 5.1, 9.2_

- [x] 3. Triển khai Master_Server - Admin Panel
  - [x] 3.1 Triển khai admin authentication và brute-force protection
    - Tạo `master-server/handlers/admin_auth.go`
    - Implement `POST /admin/login`: validate admin key, tạo session cookie HttpOnly + SameSite=Strict + TTL 12h
    - Implement IP allowlist check: trả về 404 cho IP không được phép
    - Implement brute-force protection: block IP 30 phút sau 5 lần sai trong 10 phút
    - Implement session validation middleware
    - _Requirements: 3.2, 3.3, 3.4, 3.5_

  - [x] 3.2 Viết property test cho Admin Brute-Force Protection
    - **Property 6: Admin Brute-Force Protection**
    - **Validates: Requirements 3.4**

  - [x] 3.3 Viết property test cho Admin IP Access Control
    - **Property 7: Admin IP Access Control**
    - **Validates: Requirements 3.2**

  - [x] 3.4 Triển khai admin dashboard HTML templates
    - Tạo `master-server/templates/admin/` với embedded HTML templates (no CDN)
    - Implement trang overview: tổng license, node hoạt động, heartbeat gần đây, tình trạng hệ thống
    - Implement giao diện quản lý license: tạo, xem, sửa, gia hạn, xoay key, thu hồi, kích hoạt, xóa
    - Implement giao diện quản lý release: đăng và xóa artifact với fields component, channel, version, artifact_url, sha256
    - Implement bảng heartbeat log: sortable với timestamp, node_id, IP, trạng thái
    - Implement flash messages cho thao tác thành công/thất bại
    - Thiết kế tông tối, responsive, typography dễ đọc
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6_

  - [x] 3.5 Triển khai admin API handlers
    - Tạo `master-server/handlers/admin.go`
    - Implement tất cả admin routes: GET/POST/PUT/DELETE cho licenses, releases, heartbeats
    - Áp dụng session middleware cho tất cả admin routes
    - _Requirements: 4.2, 4.3, 4.4_

- [x] 4. Triển khai Master_Server - Homepage và trang công khai
  - [x] 4.1 Triển khai Homepage
    - Tạo `master-server/templates/homepage.html` embedded template
    - Hiển thị thương hiệu Kiro, mô tả dịch vụ bảo vệ, thông tin liên hệ
    - Thiết kế tông tối hiện đại, nhất quán với Challenge_Page và Admin_Panel
    - KHÔNG chứa liên kết/tham chiếu đến `/admin` hoặc `/api/`
    - KHÔNG bao gồm JavaScript có thể lộ thông tin backend
    - Tải trong dưới 2 giây trên 3G, không phụ thuộc bên ngoài
    - _Requirements: 8.1, 8.2, 8.3, 8.4, 3.1_

  - [x] 4.2 Viết property test cho Homepage No Admin/API Exposure
    - **Property 5: Homepage No Admin/API Exposure**
    - **Validates: Requirements 3.1, 8.1**

- [x] 5. Checkpoint - Master_Server hoàn chỉnh
  - Ensure all tests pass, ask the user if questions arise.

- [x] 6. Triển khai Client_WAF - Challenge Pages
  - [x] 6.1 Triển khai JS Proof-of-Work Challenge Page
    - Tạo `client-node/challenge/pow.go` với handler `GET /__kiro/challenge`
    - Sinh challenge HTML embedded: tính toán SHA-256 nonce với difficulty prefix
    - Implement `POST /__kiro/challenge/verify`: validate PoW solution (SHA-256(token:salt:nonce) bắt đầu bằng N ký tự "0")
    - Trang hoạt động trên Chrome, Firefox, Safari, Edge không phụ thuộc CDN
    - Hiển thị chỉ báo tiến trình với hiệu ứng CSS mượt mà
    - Văn bản trạng thái bằng tiếng Việt
    - Fallback noscript hướng dẫn bật JavaScript
    - Thiết kế tông tối với branding Kiro, responsive 320px-2560px
    - _Requirements: 1.1, 1.4, 1.5, 2.1, 2.2, 2.3, 2.4, 2.5_

  - [x] 6.2 Viết property test cho PoW Verification Correctness
    - **Property 1: PoW Verification Correctness**
    - **Validates: Requirements 1.1**

  - [x] 6.3 Triển khai Hold-to-Confirm Captcha Page
    - Tạo `client-node/challenge/hold.go` với handler `GET /__kiro/hold`
    - Sinh hold captcha HTML embedded: nút giữ tối thiểu 2 giây
    - Implement `POST /__kiro/hold/verify`: validate hold duration >= 2 giây
    - Thiết kế nhất quán với PoW page, responsive, tiếng Việt
    - _Requirements: 1.2, 2.1, 2.2, 2.3, 2.5_

  - [x] 6.4 Viết property test cho Hold Time Validation
    - **Property 2: Hold Time Validation**
    - **Validates: Requirements 1.2**

  - [x] 6.5 Viết property test cho Challenge Page No External Dependencies
    - **Property 4: Challenge Page No External Dependencies**
    - **Validates: Requirements 2.4**

- [x] 7. Triển khai Client_WAF - Cookie, Rate Limiting, Ban Engine
  - [x] 7.1 Triển khai HMAC-SHA256 Access Cookie
    - Tạo `client-node/cookie/hmac.go`
    - Implement tạo cookie: HMAC-SHA256 binding IP, expiration timestamp
    - Implement xác thực cookie: verify HMAC, check IP match, check expiration
    - Cookie hết hạn → yêu cầu challenge mới
    - _Requirements: 1.3, 7.1, 7.5_

  - [x] 7.2 Viết property test cho Cookie HMAC Round-Trip và IP Binding
    - **Property 3: Cookie HMAC Round-Trip và IP Binding**
    - **Validates: Requirements 1.3, 7.1, 7.5**

  - [x] 7.3 Triển khai Rate Limiter per-IP và per-Subnet
    - Tạo `client-node/ratelimit/limiter.go`
    - Implement per-IP rate limiting với sliding window
    - Implement per-subnet /24 rate limiting độc lập
    - Cấu hình ngưỡng: soft (challenge), hard (ban)
    - _Requirements: 6.2, 6.3, 7.4_

  - [x] 7.4 Triển khai Ban Engine với XDP Sync
    - Tạo `client-node/ban/engine.go`
    - Implement in-memory BanStore (map IP → BanEntry)
    - Khi IP vượt ngưỡng hard_block_after: thêm vào BanStore + append vào blocklist file
    - Gọi xdp-sync-maps để đồng bộ blocklist đến XDP kernel map
    - Đồng bộ trong vòng 1 giây
    - _Requirements: 6.5, 6.7_

  - [x] 7.5 Viết property test cho L7 Ban to XDP Sync
    - **Property 13: L7 Ban to XDP Sync**
    - **Validates: Requirements 6.5**

  - [x] 7.6 Triển khai User-Agent Detection Engine
    - Tạo `client-node/ua/detector.go`
    - Implement `automationUserAgent()`: phát hiện sqlmap, python-requests, libwww-perl, curl, httpclient, UA rỗng
    - Chặn ngay lập tức yêu cầu từ công cụ tấn công đã biết
    - _Requirements: 6.6_

  - [x] 7.7 Viết property test cho Automation User-Agent Detection
    - **Property 14: Automation User-Agent Detection**
    - **Validates: Requirements 6.6**

- [x] 8. Triển khai Client_WAF - Reverse Proxy chính và xử lý lỗi
  - [x] 8.1 Triển khai reverse proxy handler chính
    - Tạo `client-node/proxy.go` với logic routing chính
    - Luồng: check cookie → rate limit → challenge/ban/proxy
    - Tích hợp tất cả components: cookie, ratelimit, ban, ua, challenge
    - Proxy verified requests đến backend
    - _Requirements: 1.3, 6.2, 6.5, 6.6, 7.1, 7.4, 7.5_

  - [x] 8.2 Triển khai xử lý lỗi và trang lỗi có thương hiệu
    - Implement trang 502 Bad Gateway có thương hiệu khi backend không khả dụng
    - Implement graceful degradation khi blocklist file không ghi được (log lỗi, tiếp tục L7)
    - Implement lockdown mode khi heartbeat thất bại 3 lần liên tiếp
    - Log lý do khóa và timestamp
    - _Requirements: 9.1, 9.2, 9.3, 9.4_

  - [x] 8.3 Viết property test cho Heartbeat Failure Lockdown
    - **Property 16: Heartbeat Failure Lockdown**
    - **Validates: Requirements 9.2**

  - [x] 8.4 Triển khai heartbeat loop và update check loop
    - Implement goroutine heartbeat: gửi trạng thái đến Master_Server theo interval
    - Implement goroutine update check: kiểm tra phiên bản mới, ghi thông báo ra stdout
    - Xử lý heartbeat failure counter cho lockdown logic
    - _Requirements: 5.1, 9.2_

- [x] 9. Checkpoint - Client_WAF hoàn chỉnh
  - Ensure all tests pass, ask the user if questions arise.

- [x] 10. Triển khai XDP_Filter
  - [x] 10.1 Triển khai XDP_Filter kernel program
    - Tạo `client-node/xdp_filter.c` với chương trình XDP hoàn chỉnh
    - Implement LPM trie blocklist lookup (ipv4_blocklist map)
    - Implement LPM trie allowlist lookup (ipv4_allowlist map) — allowlist ưu tiên hơn blocklist
    - Implement per-IP rate limiting (packets-per-second threshold)
    - Implement per-subnet /24 rate limiting
    - Implement malformed packet detection: null TCP flags, SYN+FIN, SYN+RST, Christmas tree, IP total_length invalid, UDP length mismatch
    - Implement private source IP drop (RFC 1918, loopback, link-local)
    - Implement UDP blocked source port
    - Implement IP fragment drop (configurable)
    - Implement per-CPU statistics counters
    - Biên dịch không warning với `clang -Wall -Werror -target bpf`
    - _Requirements: 6.1, 6.2, 6.3, 6.4, 7.3, 10.4_

  - [x] 10.2 Viết property test cho XDP Blocklist Drop
    - **Property 9: XDP Blocklist Drop**
    - Test logic lookup LPM trie bằng Go simulation
    - **Validates: Requirements 6.1**

  - [x] 10.3 Viết property test cho XDP Per-IP Rate Limiting
    - **Property 10: XDP Per-IP Rate Limiting**
    - **Validates: Requirements 6.2**

  - [x] 10.4 Viết property test cho XDP Per-Subnet Rate Limiting Independence
    - **Property 11: XDP Per-Subnet Rate Limiting Independence**
    - **Validates: Requirements 6.3, 7.4**

  - [x] 10.5 Viết property test cho XDP Malformed Packet Detection
    - **Property 12: XDP Malformed Packet Detection**
    - **Validates: Requirements 6.4**

  - [x] 10.6 Viết property test cho Private Source IP Detection
    - **Property 15: Private Source IP Detection**
    - **Validates: Requirements 7.3**

- [x] 11. Triển khai CLI_Tool và Update System
  - [x] 11.1 Triển khai CLI_Tool commands cơ bản
    - Tạo `cmd/kiro-cli/main.go` với command routing
    - Implement `kiro-cli version`: in phiên bản hiện tại
    - Implement `kiro-cli status --config <path>`: hiển thị trạng thái hệ thống
    - Implement `kiro-cli health --config <path>`: health report
    - Implement `kiro-cli preflight --config <path>`: pre-deployment check
    - _Requirements: 5.1, 10.2_

  - [x] 11.2 Triển khai Update System qua CLI
    - Tạo `cmd/kiro-cli/update/update.go`
    - Implement `kiro-cli update check`: gọi API `/api/v1/update/check`, hiển thị thông tin bản mới
    - Implement `kiro-cli update apply`: tải artifact, xác minh SHA-256, atomic binary replace, restart service, health check 30s
    - Implement `kiro-cli update rollback`: khôi phục binary .bak, restart service
    - Xử lý lỗi: SHA-256 mismatch → abort + log; health fail → auto rollback
    - _Requirements: 5.2, 5.3, 5.4, 5.5, 5.6_

  - [x] 11.3 Viết property test cho SHA-256 Update Verification
    - **Property 8: SHA-256 Update Verification**
    - **Validates: Requirements 5.3, 5.4**

- [x] 12. Checkpoint - Tất cả components đã triển khai
  - Ensure all tests pass, ask the user if questions arise.

- [x] 13. Cấu hình Nginx và triển khai
  - [x] 13.1 Tạo cấu hình Nginx reverse proxy
    - Tạo `deployments/nginx/kiro-waf.conf`
    - Cấu hình TLS termination
    - Cấu hình Cloudflare origin lock: chỉ accept traffic từ CF IP ranges, deny all khác
    - Cấu hình admin path IP restriction: `/admin/` chỉ cho phép IP trong allowlist
    - Proxy pass `/` → Client_WAF (:8090)
    - Proxy pass `/admin/` → Client_WAF (:8090) → Master_Server (:8080)
    - Tạo `kiro-admin-allow.conf` với danh sách IP admin
    - _Requirements: 3.2, 7.2, 10.1_

  - [x] 13.2 Tạo script triển khai tự động cho Ubuntu 22.04
    - Tạo `deploy_master.sh` hoàn chỉnh
    - Cài đặt dependencies: Go, clang, libbpf, nginx, certbot
    - Build Master_Server binary → `/usr/local/bin/kiro-master`
    - Build Client_WAF binary → `/usr/local/bin/kiro-client-waf`
    - Build XDP object: `clang -Wall -Werror -O2 -target bpf` → `/usr/lib/kiro/xdp/xdp_filter.o`
    - Tạo systemd service files: kiro-master.service, kiro-client-waf.service
    - Cấu hình Nginx reverse proxy
    - Tạo thư mục data: `/var/lib/kiro-master/`, `/var/lib/kiro/`, `/etc/kiro-master/`
    - Xác minh health: tất cả services active, healthz endpoints respond
    - _Requirements: 10.1, 10.2, 10.3, 10.4, 10.5_

  - [x] 13.3 Tạo systemd service files
    - Tạo `deployments/systemd/kiro-master.service`: auto-restart on failure, health check
    - Tạo `deployments/systemd/kiro-client-waf.service`: auto-restart, env config
    - Cấu hình `Restart=on-failure`, `RestartSec=5`, `WantedBy=multi-user.target`
    - _Requirements: 10.1, 10.2_

- [x] 14. Kết nối toàn bộ hệ thống và integration tests
  - [x] 14.1 Kết nối Master_Server main.go
    - Tạo `master-server/main.go` hoàn chỉnh
    - Khởi tạo database, load config từ env/flags
    - Register tất cả routes: homepage, admin, API, healthz
    - Áp dụng middleware: logging, recovery, admin auth
    - Graceful shutdown
    - _Requirements: 10.1, 9.5_

  - [x] 14.2 Kết nối Client_WAF main entry point
    - Hoàn thiện `client-node/client_waf.go`
    - Khởi tạo tất cả components: cookie, ratelimit, ban, ua, challenge, proxy
    - Start heartbeat loop và update check loop
    - Register routes: challenge, hold, healthz, proxy catch-all
    - Graceful shutdown, fatal on missing config (cookie_secret, license_key)
    - _Requirements: 10.2, 9.4_

  - [x] 14.3 Viết integration tests
    - Test luồng License CRUD end-to-end
    - Test luồng Heartbeat: Client → Master → DB → Response
    - Test luồng Update: Check → Download → Verify → Replace → Health
    - Test concurrent DB access với multiple goroutines
    - _Requirements: 4.2, 5.5, 9.5_

- [x] 15. Final checkpoint - Toàn bộ hệ thống hoạt động
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks đánh dấu `*` là optional và có thể bỏ qua cho MVP nhanh hơn
- Mỗi task tham chiếu requirements cụ thể để đảm bảo traceability
- Checkpoints đảm bảo validation tăng dần
- Property tests sử dụng thư viện `rapid` (github.com/flyingmutant/rapid) với tối thiểu 100 iterations
- Unit tests validate ví dụ cụ thể và edge cases
- XDP property tests sử dụng Go simulation của logic XDP (không chạy kernel code trực tiếp trong test)
- Tất cả HTML templates được embedded (không CDN), thiết kế tông tối nhất quán

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1"] },
    { "id": 1, "tasks": ["1.2"] },
    { "id": 2, "tasks": ["2.1", "6.1"] },
    { "id": 3, "tasks": ["2.2", "2.3", "6.2", "6.3"] },
    { "id": 4, "tasks": ["2.4", "3.1", "6.4", "6.5"] },
    { "id": 5, "tasks": ["3.2", "3.3", "3.4", "7.1"] },
    { "id": 6, "tasks": ["3.5", "4.1", "7.2", "7.3"] },
    { "id": 7, "tasks": ["4.2", "7.4", "7.6"] },
    { "id": 8, "tasks": ["7.5", "7.7", "8.1"] },
    { "id": 9, "tasks": ["8.2", "8.4", "10.1"] },
    { "id": 10, "tasks": ["8.3", "10.2", "10.3", "10.4", "10.5", "10.6"] },
    { "id": 11, "tasks": ["11.1", "11.2"] },
    { "id": 12, "tasks": ["11.3", "13.1"] },
    { "id": 13, "tasks": ["13.2", "13.3"] },
    { "id": 14, "tasks": ["14.1", "14.2"] },
    { "id": 15, "tasks": ["14.3"] }
  ]
}
```
