# Implementation Plan: WAF System Overhaul

## Overview

Đại tu toàn diện hệ thống Kiro WAF bao gồm 15 lĩnh vực: nhận diện thương hiệu, giao diện frontend hiện đại, trực quan hóa dữ liệu, cài đặt client thông minh, cập nhật OTA tự động, tối ưu hiệu năng XDP/eBPF, tối ưu hiệu năng Golang WAF, tái cấu trúc thư mục, tài liệu dự án, tài liệu người dùng cuối, xử lý lỗi/ngăn rò rỉ bộ nhớ, CLI commands đầy đủ với tài liệu website, kiểm tra liên tục XDP/binary với tự phục hồi, gói Community không giới hạn thời gian, và cải thiện trải nghiệm cài đặt (Install UX). Tái cấu trúc thư mục (Yêu Cầu 8) được thực hiện đầu tiên vì nó thiết lập nền tảng cho tất cả công việc khác.

## Tasks

- [x] 1. Directory restructuring to Standard Go Layout
  - [x] 1.1 Create new directory structure and move master-server code
    - Create `cmd/kiro-master/main.go` entry point
    - Move `master-server/` packages to `internal/master/` (handlers, db, models, templates)
    - Update all internal import paths from `kiro_waf/master-server/...` to new paths
    - _Requirements: 8.1, 8.7, 8.10_

  - [x] 1.2 Move client-node code to new structure
    - Create `cmd/kiro-client/main.go` entry point
    - Move `client-node/` packages to `internal/client/` (proxy, ratelimit, challenge, cookie, ban, ua)
    - Move `client-node/xdp_filter.c` to `internal/client/xdp/`
    - Update all internal import paths
    - _Requirements: 8.2, 8.3, 8.10_

  - [x] 1.3 Reorganize shared packages and CLI
    - Move shared code to `pkg/` (version, health)
    - Ensure `cmd/kiro-cli/` entry point is at `cmd/kiro-cli/main.go`
    - Move web assets to `web/templates/` and `web/static/`
    - Verify `deployments/` contains systemd, nginx, nftables, sysctl configs
    - _Requirements: 8.4, 8.5, 8.6_

  - [x] 1.4 Create Makefile and build system
    - Create root `Makefile` with `build` target producing all three binaries in `build/`
    - Add `build-xdp` target for compiling XDP C code with clang -O2
    - Update `go.mod` to reflect new module structure
    - Verify `make build` exits zero with no compilation errors
    - _Requirements: 8.7, 8.8, 8.9_

  - [x] 1.5 Write build verification tests
    - Verify all import paths resolve after restructuring
    - Verify `make build` produces kiro-master, kiro-client, kiro-cli binaries
    - Verify `make build-xdp` compiles XDP object < 32KB
    - _Requirements: 8.8, 8.9_

- [x] 2. Checkpoint - Ensure build passes after restructuring
  - Ensure all tests pass, ask the user if questions arise.

- [x] 3. Brand Identity System
  - [x] 3.1 Create SVG logo and favicon
    - Create `web/static/img/kiro-logo.svg` — vector-only, scalable 16px–512px, cyber/security aesthetic with teal color
    - Create `web/static/img/favicon.svg` derived from logo
    - _Requirements: 1.1, 1.5_

  - [x] 3.2 Create unified CSS with design tokens
    - Create `web/static/css/kiro-brand.css` with CSS custom properties: --kiro-primary (#0d9488), --kiro-accent, --kiro-background, --kiro-surface, --kiro-text-primary, --kiro-text-secondary, --kiro-border, --kiro-success, --kiro-danger, --kiro-warning
    - Create `web/static/css/kiro.css` as single unified stylesheet (<100KB uncompressed)
    - Include dark mode base styles, glassmorphism, neon glow effects
    - _Requirements: 1.2, 1.3, 2.1, 2.6_

  - [x] 3.3 Update HTML templates to use brand system
    - Update all admin templates to load single CSS file and display SVG logo in navbar
    - Add `<link rel="icon">` SVG favicon to all pages
    - Apply color tokens to navigation, page headers, and footers
    - Ensure homepage and challenge pages also use brand tokens
    - _Requirements: 1.3, 1.4, 1.5_

- [x] 4. Modern Frontend UI with Dark Mode
  - [x] 4.1 Implement glassmorphism card and panel components
    - Apply `backdrop-filter: blur(12px)` + `rgba(26,26,46,0.7)` to cards/panels
    - Add fallback solid `rgba(26,26,46,0.9)` for browsers without backdrop-filter support
    - _Requirements: 2.2, 2.7_

  - [x] 4.2 Implement neon glow effects and responsive layout
    - Apply teal neon glow `box-shadow` to primary action buttons and active nav items
    - Implement CSS Grid + Flexbox layout with single-column fallback below 768px
    - Ensure WCAG 2.1 AA contrast ratios (≥4.5:1 for normal text, ≥3:1 for large text)
    - _Requirements: 2.3, 2.4, 2.5_

  - [x] 4.3 Rebuild admin dashboard template
    - Redesign `/admin/` dashboard with dark mode glassmorphism cards for stats
    - Add chart containers for license distribution and heartbeat timeline
    - Style license, release, and heartbeat list pages
    - _Requirements: 2.1, 2.2, 2.3_

- [x] 5. Dynamic Charts and Data Visualization
  - [x] 5.1 Bundle Chart.js and create chart engine
    - Download Chart.js (<50KB gzipped) to `web/static/js/chart.min.js`
    - Create `web/static/js/kiro-charts.js` implementing chart rendering functions
    - Implement `renderLicenseDistribution()` — doughnut/pie chart for active/suspended/revoked/expired
    - _Requirements: 3.1, 3.4_

  - [x] 5.2 Implement heartbeat timeline and release history charts
    - Implement `renderHeartbeatTimeline()` — line chart showing hourly heartbeat counts for 24h
    - Implement `renderReleaseHistory()` — scatter/line chart with version vs. creation date
    - Add tooltip on hover/tap showing exact values
    - _Requirements: 3.2, 3.3, 3.7_

  - [x] 5.3 Add chart API endpoints and empty state handling
    - Add `GET /api/v1/charts/dashboard` endpoint returning `DashboardChartData` JSON
    - Add `GET /api/v1/charts/releases` endpoint returning `ReleaseChartData` JSON
    - Implement empty state placeholder messages when no data available
    - Ensure charts are responsive 320px–2560px with readable labels (≥10px font)
    - _Requirements: 3.5, 3.6, 3.8_

- [x] 6. Checkpoint - Ensure UI and charts render correctly
  - Ensure all tests pass, ask the user if questions arise.

- [x] 7. Smart Client Install Script Enhancement
  - [x] 7.1 Implement OS detection and dependency auto-install
    - Add `detect_os()` function parsing `/etc/os-release` for Ubuntu, Debian, CentOS, Rocky, Fedora, Arch
    - Add `install_dependencies()` using detected package manager (apt, yum, dnf, pacman)
    - Auto-install missing required deps (curl, sha256sum, systemctl)
    - Add `--xdp-mode` flag to install XDP build deps (clang, llvm, libbpf-dev)
    - Exit with error listing supported distros if OS unsupported
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5_

  - [x] 7.2 Implement idempotent installation logic
    - Check existing binary version before downloading; skip if same version
    - Preserve existing config files on re-run
    - Ensure systemd service remains enabled+running after re-run
    - Stop existing service before binary replacement, restart after
    - _Requirements: 4.9, 4.10, 4.11_

  - [x] 7.3 Write property tests for OS detection and install script
    - **Property 13: OS Detection Correctness**
    - **Validates: Requirements 4.1**
    - Test `detect_os()` logic with various `/etc/os-release` content
    - **Property 12: Install Script Idempotency**
    - **Validates: Requirements 4.9**

- [x] 8. OTA Automatic Update System
  - [x] 8.1 Implement OTA updater core with poll interval clamping
    - Create `internal/client/updater/updater.go` implementing `Updater` interface
    - Implement `CheckForUpdate()` polling master at configurable interval (default 300s, min 60s, max 86400s)
    - Implement heartbeat push trigger for immediate update check
    - Create `UpdaterConfig` and `UpdateState` data models
    - _Requirements: 5.1, 5.2_

  - [x] 8.2 Implement download, SHA-256 verification, and atomic replacement
    - Implement `DownloadAndVerify()` with 5-minute timeout and SHA-256 checksum verification
    - Implement `ApplyUpdate()` using atomic `rename(2)` for binary replacement
    - Abort and cleanup partial downloads on checksum mismatch or network failure
    - _Requirements: 5.3, 5.4, 5.5, 5.6_

  - [x] 8.3 Implement rollback and backup management
    - Implement `Rollback()` restoring previous binary from backup path
    - Health check: if new binary not active within 30s, auto-rollback
    - Retain exactly one previous binary version at backup path
    - Log all operations (check, download, verify, replace, rollback) to system journal
    - _Requirements: 5.7, 5.8, 5.9, 5.10_

  - [x] 8.4 Write property tests for OTA updater
    - **Property 1: SHA-256 Verification Round-Trip**
    - **Validates: Requirements 4.6, 5.3**
    - **Property 2: OTA Poll Interval Clamping**
    - **Validates: Requirements 5.1**
    - **Property 3: Atomic Binary Replacement Preserves Content**
    - **Validates: Requirements 5.6**
    - **Property 4: Exactly One Backup Version Retained**
    - **Validates: Requirements 5.8**

- [x] 9. Checkpoint - Ensure OTA and install script work correctly
  - Ensure all tests pass, ask the user if questions arise.

- [x] 10. XDP/eBPF Performance Optimization
  - [x] 10.1 Optimize XDP filter for per-CPU maps and zero allocation
    - Ensure `BPF_MAP_TYPE_PERCPU_ARRAY` for stats counters (eliminate lock contention)
    - Ensure `BPF_MAP_TYPE_LRU_HASH` with 262,144 entries for rate state
    - Ensure `BPF_MAP_TYPE_LPM_TRIE` with 65,536 entries for blocklist
    - Verify zero dynamic allocation in XDP path
    - Ensure non-IPv4 packets (EtherType != 0x0800) return XDP_PASS immediately
    - _Requirements: 6.1, 6.2, 6.5, 6.9_

  - [x] 10.2 Optimize XDP for throughput targets and edge cases
    - Optimize hot path for <100ns per 64-byte packet on x86_64 @ 3.0GHz
    - Target 10M pps single-core throughput in XDP native mode
    - Handle blocklist map full: continue with existing entries (no XDP_ABORTED)
    - Handle LRU rate map full: kernel LRU eviction continues rate limiting
    - Compile with `clang -O2`, verify BPF object < 32KB
    - _Requirements: 6.3, 6.4, 6.6, 6.7, 6.8_

  - [x] 10.3 Write property test for XDP packet classification
    - **Property 5: Non-IPv4 Packets Pass Through XDP**
    - **Validates: Requirements 6.9**
    - Test with simulated packet structures for various EtherType values

- [x] 11. Golang WAF Performance Optimization
  - [x] 11.1 Implement sync.Pool buffer reuse and zero-allocation hot path
    - Create `internal/client/proxy/pool.go` with `sync.Pool` for request/response buffers
    - Implement zero-allocation header inspection and routing (0 allocs/op in benchmarks)
    - Set up `GOGC` and `GOMEMLIMIT` configuration from environment
    - _Requirements: 7.4, 7.6, 7.9_

  - [x] 11.2 Implement connection pool with queue timeout
    - Create `internal/client/proxy/connpool.go` with configurable max idle (256) and max total (1024) connections
    - Implement 90s idle timeout and 1s queue timeout for connection acquisition
    - Return 503 when pool exhausted and queue timeout exceeded
    - Return 502 within 5s when backend unreachable (no goroutine leak)
    - _Requirements: 7.5, 7.7, 7.8_

  - [x] 11.3 Implement goroutine semaphore and panic recovery
    - Create `internal/client/proxy/semaphore.go` with configurable max (default 10,000)
    - Return HTTP 503 when semaphore at capacity
    - Implement `RecoverMiddleware` for panic recovery with stack trace logging
    - Apply read timeout 30s, write timeout 60s on all HTTP connections
    - _Requirements: 7.1, 11.1, 11.2, 11.3, 11.4, 11.5_

  - [x] 11.4 Write property tests for WAF proxy performance
    - **Property 6: Zero-Allocation Proxy Hot Path**
    - **Validates: Requirements 7.4**
    - **Property 7: Unreachable Backend Returns 502 Without Goroutine Leak**
    - **Validates: Requirements 7.7**
    - **Property 8: Pool Exhaustion Returns 503 Within Queue Timeout**
    - **Validates: Requirements 7.8**
    - **Property 9: Goroutine Semaphore Enforces Maximum Concurrency**
    - **Validates: Requirements 11.3, 11.9**
    - **Property 10: Panic Recovery Continues Serving**
    - **Validates: Requirements 11.4**

- [x] 12. Error Handling and Memory Leak Prevention
  - [x] 12.1 Implement resource cleanup and timeout enforcement
    - Ensure all `resp.Body` closed via `defer` in same function scope
    - Apply read timeout 30s, write timeout 60s on master server HTTP connections
    - Implement database query context cancellation after 5s with 503 JSON response
    - _Requirements: 11.1, 11.2, 11.5, 11.6_

  - [x] 12.2 Implement periodic cleanup and startup validation
    - Implement rate-limit entry cleanup every 120s (remove entries older than window)
    - Implement challenge token cleanup every 60s (remove tokens older than 60s TTL)
    - Validate required config at startup: license_key, cookie_secret, backend_url, master_url
    - Exit with descriptive error identifying missing value if any required config absent
    - _Requirements: 11.7, 11.8_

  - [x] 12.3 Write property test for periodic cleanup
    - **Property 11: Periodic Cleanup Removes Exactly Expired Entries**
    - **Validates: Requirements 11.7**
    - Test that cleanup removes exactly entries older than window, retains entries within window

- [x] 13. Checkpoint - Ensure performance and error handling tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 14. README and Project Documentation
  - [x] 14.1 Create comprehensive README
    - Add CI status badge (GitHub Actions), Go version badge, license badge as first elements
    - Add Mermaid architecture diagram showing Master_Server, Client_Node, XDP_Filter, Cloudflare, SQLite relationships
    - Add Mermaid sequence diagram for heartbeat polling and OTA update flow
    - Structure sections: overview, architecture, quick start, configuration, deployment, contributing
    - Ensure quick start allows build + test from clean clone in 10 minutes
    - _Requirements: 9.1, 9.2, 9.3, 9.4, 9.7_

  - [x] 14.2 Create architecture and development documentation
    - Create `docs/architecture.md` describing Master_Server, Client_Node, XDP_Filter, CLI_Tool with data flow
    - Create `docs/development.md` with prerequisites (Go, clang, llvm, libbpf-dev, make), build steps, test commands
    - Validate Mermaid syntax renders correctly in GitHub Markdown
    - _Requirements: 9.5, 9.6, 9.7_

- [x] 15. End-User Public Documentation
  - [x] 15.1 Create documentation site handler and structure
    - Implement `DocsHandler` in `internal/master/handlers/` serving static HTML at `/docs`
    - Create sidebar navigation with table of contents
    - Implement language switcher (Vietnamese + English) visible on every page
    - Return custom error page when docs unavailable (not generic server error)
    - _Requirements: 10.1, 10.6, 10.7_

  - [x] 15.2 Write documentation content
    - Write installation guide (quick-start: install + configure in 15 minutes)
    - Write configuration reference: all YAML options with type, default, range, description, example
    - Write troubleshooting section with ≥10 common error scenarios and resolutions
    - Write FAQ section
    - Display version/date on each page
    - Ensure no internal API endpoints, DB schema, source paths, or security details exposed
    - _Requirements: 10.2, 10.3, 10.4, 10.5, 10.8_

- [x] 16. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 17. CLI Commands Testing & Documentation
  - [x] 17.1 Viết unit tests cho tất cả CLI commands
    - Tạo test files cho mỗi sub-package: `cmd/kiro-cli/status/status_test.go`, `cmd/kiro-cli/update/update_test.go`, v.v.
    - Test lệnh `version` trả về chuỗi semver (X.Y.Z) với mã thoát 0
    - Test lệnh `license fingerprint` trả về chuỗi hex 64 ký tự lowercase, chấp nhận --salt
    - Test lệnh `status` trả về JSON với các trường mode, uptime, license status, version
    - Test lệnh `health` trả về JSON với overall status (healthy/degraded/unhealthy)
    - Test lệnh `preflight` trả về JSON với OS compatibility, root check, command availability
    - Test lệnh `mode set` từ chối giá trị không phải "server" hoặc "full" với mã thoát 1
    - Test lệnh `install apply-lab` từ chối khi --ack sai hoặc UID != 0
    - Đảm bảo coverage tối thiểu 80% cho package cmd/kiro-cli và sub-packages
    - _Requirements: 12.1, 12.2, 12.3, 12.4, 12.5, 12.6, 12.16, 12.18_

  - [x] 17.2 Viết integration tests cho CLI commands
    - Test lệnh `update check` với mock Master_Server (yêu cầu --master-url)
    - Test lệnh `update apply` với xác minh SHA-256 và auto-rollback khi health check thất bại
    - Test lệnh `update rollback` khôi phục file .bak
    - Test lệnh `install plan` trả về JSON kế hoạch cài đặt
    - Test lệnh `install stage-lab` staging vào --install-root
    - Test lệnh `incident report` tạo JSON và Markdown với các tham số --type, --severity, --status, --summary
    - Test lệnh `pilot report` tổng hợp evidence từ --health-file, --benchmark-file, --incident-dir
    - Test lệnh `report` trả về JSON báo cáo tổng hợp
    - Test invalid command hiển thị usage và thoát mã 2
    - Test missing required params hiển thị lỗi chỉ rõ tên tham số và thoát mã 2
    - _Requirements: 12.7, 12.8, 12.9, 12.10, 12.11, 12.12, 12.13, 12.17_

  - [x] 17.3 Tạo trang CLI documentation trên website
    - Tạo nội dung trang tổng quan CLI tại `/docs/vi/cli/` và `/docs/en/cli/` với mục lục tất cả lệnh
    - Viết trang chi tiết cho mỗi lệnh (tối thiểu 11 lệnh): version, license, status, health, preflight, mode, install, update, incident, pilot, report
    - Mỗi trang chứa: cú pháp sử dụng, bảng tham số (bắt buộc/tùy chọn, kiểu dữ liệu, giá trị mặc định), ít nhất 1 ví dụ thực tế, mã thoát và ý nghĩa
    - Thêm mục lục điều hướng nhanh và hỗ trợ tìm kiếm theo tên lệnh
    - Cập nhật `internal/master/handlers/docs_content_vi.go` và `docs_content_en.go` với nội dung CLI docs
    - _Requirements: 12.14, 12.15_

- [x] 18. Health Monitor Implementation
  - [x] 18.1 Tạo cấu trúc Health Monitor cơ bản
    - Tạo thư mục `internal/client/monitor/`
    - Tạo `monitor.go` triển khai interface `HealthMonitor` (Start, Stop, Status)
    - Tạo `config.go` với `MonitorConfig` (CheckInterval 10s, HealthEndpoint, HealthTimeout 5s, MaxConsecFailures 3, MaxRestartFailures 3)
    - Tạo `types.go` với `MonitorStatus`, `OperationMode` (online/offline/emergency), `FailureTracker`
    - Triển khai vòng lặp health check chính chạy mỗi 10 giây trong goroutine riêng
    - _Requirements: 13.1, 13.2_

  - [x] 18.2 Triển khai kiểm tra XDP và binary health check
    - Triển khai kiểm tra XDP_Filter: đọc BPF map statistics và xác minh chương trình XDP gắn vào network interface
    - Triển khai kiểm tra binary: gọi endpoint `/__kiro/health` với timeout 5 giây, xác minh HTTP 200
    - Triển khai auto-reload và reattach XDP khi bị detach (trong vòng 30 giây)
    - Triển khai auto-restart service khi binary không phản hồi 3 lần liên tiếp
    - Triển khai `FailureTracker` với logic escalation: 3 restart thất bại trong 5 phút → alert Master + chờ 60 giây
    - Triển khai XDP reload failure tracking: 3 lần liên tiếp → critical log + alert + retry mỗi 60 giây
    - _Requirements: 13.1, 13.2, 13.3, 13.4, 13.9, 13.12_

  - [x] 18.3 Triển khai chế độ offline và DDoS detection
    - Tạo `offline.go` với logic chuyển sang offline mode khi mất kết nối Master > 60 giây
    - Triển khai exponential backoff reconnect: 30s ban đầu, nhân đôi mỗi lần, tối đa 5 phút
    - Triển khai sync lại cấu hình và gửi offline report khi kết nối khôi phục
    - Tạo `traffic.go` với `TrafficAnalyzer`: sliding window 10 giây, tính CurrentRate
    - Triển khai emergency recovery: khi traffic > 5x threshold + crash → restart với rate-limit giảm 50% trong 5 phút
    - Triển khai DDoS detection: khi traffic > 10x threshold → kích hoạt XDP strict mode + alert Master trong 5 giây
    - Triển khai Package_Plan enforcement: từ chối cấu hình vượt giới hạn plan (domains, XDP, OTA, RPM)
    - _Requirements: 13.5, 13.6, 13.7, 13.10, 13.11_

  - [x] 18.4 Triển khai state snapshot
    - Tạo `snapshot.go` với logic ghi snapshot ra disk mỗi 60 giây
    - Serialize `HealthSnapshot` (rate-limit state, session state, ban list, XDP stats) dạng JSON compressed (gzip)
    - Giới hạn kích thước snapshot tối đa 64MB, truncate entries cũ nhất nếu vượt
    - Xử lý lỗi I/O: ghi log cảnh báo, giữ snapshot cũ, thử lại chu kỳ tiếp theo
    - _Requirements: 13.8, 13.13_

- [x] 19. Community Plan Logic
  - [x] 19.1 Tạo Plan Manager cơ bản
    - Tạo thư mục `internal/master/plan/`
    - Tạo `plan.go` triển khai interface `PlanManager` (CreateCommunityLicense, CheckExpiry, UpgradePlan, DowngradeToCommunity, EnforcePlanLimits)
    - Định nghĩa constants cho plan limits: Community (1 domain, XDP tắt, OTA tắt, 60 rpm), Pro, Enterprise
    - Tạo `types.go` với `DowngradeEvent`, `RequestedConfig`, `PlanChange`
    - _Requirements: 14.1, 14.3, 14.4_

  - [x] 19.2 Triển khai logic hết hạn và auto-downgrade
    - Triển khai `CheckExpiry()`: quét license Pro/Enterprise có ExpiresAt < now, chuyển về Community trong 60 giây
    - Khi downgrade: giữ nguyên license_key, fingerprint_hash, lịch sử — chỉ thay đổi plan, đặt ExpiresAt = zero, vô hiệu hóa tính năng premium
    - Triển khai quy tắc ưu tiên: suspended > expired (nếu license bị suspended VÀ hết hạn → giữ suspended)
    - Triển khai `CreateCommunityLicense()`: tạo license mới với plan=community, status=active, ExpiresAt=zero (vô thời hạn)
    - Ghi `PlanChange` vào lịch sử mỗi khi thay đổi plan
    - _Requirements: 14.1, 14.2, 14.3, 14.7, 14.9_

  - [x] 19.3 Triển khai upgrade flow và heartbeat enforcement
    - Triển khai `UpgradePlan()`: giữ nguyên license_key và fingerprint, cập nhật plan + tính năng + ExpiresAt mới
    - Cập nhật heartbeat response để bao gồm Package_Plan và trạng thái license
    - Triển khai logic Client_Node: khi nhận plan=Community → hoạt động bình thường với tính năng cơ bản (rate-limit 60 rpm/IP, challenge page)
    - Triển khai logic Client_Node: khi nhận status=suspended → ngừng xử lý traffic, trả về trang thông báo tạm ngưng
    - Triển khai offline behavior: license Community cached → tiếp tục hoạt động không tự vô hiệu hóa
    - Cập nhật Admin_UI: hiển thị trạng thái gói, trạng thái license, ngày hết hạn, nút nâng cấp
    - _Requirements: 14.4, 14.5, 14.6, 14.7, 14.8, 14.10_

- [x] 20. Install UX Enhancement
  - [x] 20.1 Triển khai hệ thống UI cơ bản cho install script
    - Tạo các hàm UI trong `scripts/install-client.sh` (hoặc source từ file riêng)
    - Triển khai `detect_color_support()`: kiểm tra --quiet, TERM=dumb, ! -t 1 → set NO_COLOR và NO_ANIMATION
    - Triển khai mã màu nhất quán: xanh lá (✓), đỏ (✗), vàng (⚠), cyan (→), trắng đậm cho tiêu đề
    - Triển khai `print_step()` với format [N/T] cho số thứ tự bước
    - Triển khai `step_complete()` hiển thị thời gian thực hiện (ví dụ: "✓ Tải binary hoàn tất (3.2s)")
    - Triển khai banner ASCII art logo Kiro WAF với màu teal/cyan, phiên bản script, URL master
    - _Requirements: 15.1, 15.4, 15.5, 15.6, 15.9, 15.10_

  - [x] 20.2 Triển khai progress bar và spinner
    - Triển khai `show_progress_bar()`: thanh ngang [████████░░░░░░░░] 50%, chiều rộng tối thiểu 20 ký tự, cập nhật mỗi 1s hoặc 2%
    - Triển khai `start_spinner()` và `stop_spinner()`: ký tự Braille (⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏), tốc độ 80-120ms/frame
    - Triển khai xóa dòng animation trước khi in lỗi (`printf "\r\033[K"`)
    - Tích hợp progress bar vào bước tải binary (dựa trên bytes đã tải / tổng bytes)
    - Tích hợp spinner vào các bước không xác định thời gian (phát hiện OS, cài dependency, tạo service)
    - _Requirements: 15.2, 15.3, 15.11_

  - [x] 20.3 Triển khai summary box và error handling UX
    - Triển khai `print_summary_box()`: box-drawing characters (┌─┐│└─┘), chứa version, binary path, service status, IP, lệnh hữu ích
    - Triển khai `print_error_with_suggestion()`: dừng animation, hiển thị tên bước + nguyên nhân + gợi ý khắc phục
    - Tích hợp error handling vào tất cả bước: lỗi mạng → "Kiểm tra kết nối mạng", lỗi auth → "Xác minh license key"
    - Triển khai --quiet/-q mode: tắt animation + escape code, chỉ hiển thị text thuần
    - Đảm bảo fallback graceful khi terminal không hỗ trợ Unicode (spinner → simple dots)
    - _Requirements: 15.7, 15.8, 15.9, 15.10, 15.11_

- [x] 21. Checkpoint - Đảm bảo tất cả implementation mới hoạt động
  - Đảm bảo tất cả tests pass, hỏi người dùng nếu có thắc mắc.

- [x] 22. Property Tests cho Requirements 12-15
  - [ ]* 22.1 Viết property test cho CLI fingerprint và output validation
    - **Property 14: CLI Fingerprint Produces Valid Hex**
    - **Validates: Requirements 12.2**
    - Test: mọi salt string (rỗng, ASCII, Unicode) → kết quả 64 ký tự hex lowercase, deterministic
    - **Property 15: CLI Command Output Contains Required JSON Fields**
    - **Validates: Requirements 12.3, 12.4, 12.5**
    - Test: status trả về mode/uptime/license/version; health trả về overall status; preflight trả về OS/root/commands

  - [ ]* 22.2 Viết property test cho CLI validation và access control
    - **Property 16: CLI Mode Validation Rejects Invalid Values**
    - **Validates: Requirements 12.6**
    - Test: mọi chuỗi không phải "server"/"full" → mã thoát 1
    - **Property 17: CLI Invalid Command and Missing Params Exit Code 2**
    - **Validates: Requirements 12.12, 12.13**
    - Test: lệnh không hợp lệ → mã 2 + usage; tham số bắt buộc thiếu → mã 2 + tên tham số
    - **Property 18: Install Apply-Lab Access Control**
    - **Validates: Requirements 12.18**
    - Test: --ack sai hoặc UID != 0 → từ chối với mã 1

  - [ ]* 22.3 Viết property test cho Health Monitor state machine
    - **Property 19: Health Monitor Failure Threshold State Machine**
    - **Validates: Requirements 13.4, 13.9, 13.12**
    - Test: restart kích hoạt khi và chỉ khi 3 fail liên tiếp; alert khi 3 restart fail trong 5 phút; critical log khi 3 XDP reload fail
    - **Property 20: DDoS Traffic Threshold Detection**
    - **Validates: Requirements 13.5, 13.11**
    - Test: emergency khi > 5x threshold; XDP strict khi > 10x threshold; trở về bình thường khi dưới ngưỡng
    - **Property 21: Offline Mode with Exponential Backoff**
    - **Validates: Requirements 13.6**
    - Test: offline khi mất kết nối > 60s; backoff = min(30s × 2^(N-1), 300s)

  - [ ]* 22.4 Viết property test cho Health Monitor snapshot và plan enforcement
    - **Property 22: State Snapshot Size Bounded**
    - **Validates: Requirements 13.8**
    - Test: snapshot serialize ≤ 64MB; truncate entries cũ nhất khi vượt giới hạn
    - **Property 23: Package_Plan Enforcement**
    - **Validates: Requirements 13.10**
    - Test: từ chối config vượt giới hạn plan (domains, XDP, OTA, RPM); chấp nhận config trong giới hạn

  - [ ]* 22.5 Viết property test cho Community Plan logic
    - **Property 24: License Expiry Downgrade Preserves Identity**
    - **Validates: Requirements 14.1, 14.2, 14.3**
    - Test: license hết hạn → Community với license_key/fingerprint giữ nguyên, ExpiresAt=zero; đăng ký mới → Community active
    - **Property 25: License Upgrade Preserves Identity**
    - **Validates: Requirements 14.4**
    - Test: upgrade giữ nguyên license_key và fingerprint, chỉ thay đổi plan/features/ExpiresAt
    - **Property 26: Community License Never Self-Disables**
    - **Validates: Requirements 14.5, 14.6**
    - Test: license Community tiếp tục hoạt động bất kể trạng thái kết nối Master
    - **Property 27: Suspended State Priority and Blocking**
    - **Validates: Requirements 14.7, 14.9, 14.10**
    - Test: suspended + expired → giữ suspended; suspended → block traffic; chỉ suspended mới block

  - [ ]* 22.6 Viết property test cho Install UX
    - **Property 28: Progress Bar Monotonic and Bounded**
    - **Validates: Requirements 15.2**
    - Test: phần trăm tăng đơn điệu [0,100]; chiều rộng ≥ 20 ký tự
    - **Property 29: Step Progress Metadata Correctness**
    - **Validates: Requirements 15.5, 15.6**
    - Test: bước đánh số [N/T] tuần tự 1→T; thời gian = (end-start) format 1 chữ số thập phân
    - **Property 30: Install Summary Contains Required Fields**
    - **Validates: Requirements 15.7**
    - Test: summary chứa version, binary path, service status, IP, lệnh hữu ích; sử dụng box-drawing chars
    - **Property 31: Error Message Completeness**
    - **Validates: Requirements 15.8, 15.11**
    - Test: lỗi chứa tên bước + nguyên nhân + gợi ý; animation bị xóa trước lỗi
    - **Property 32: Color Suppression in Non-Color Environments**
    - **Validates: Requirements 15.9, 15.10**
    - Test: --quiet hoặc TERM=dumb hoặc non-TTY → không có ANSI escape codes; nội dung giữ nguyên

- [x] 23. Final Integration Testing cho tính năng mới
  - [x] 23.1 Integration test cho CLI commands end-to-end
    - Test full binary execution với testscript cho mỗi lệnh CLI
    - Test luồng update apply → health check fail → auto-rollback
    - Test luồng install plan → stage-lab → apply-lab với mock filesystem
    - Xác minh tất cả lệnh hoạt động đúng với cấu hình thực tế
    - _Requirements: 12.1–12.18_

  - [x] 23.2 Integration test cho Health Monitor
    - Test XDP detach detection và reattach (yêu cầu BPF environment)
    - Test binary health endpoint check timing và restart logic
    - Test chuyển đổi offline → online với config sync
    - Test DDoS detection → XDP strict mode activation
    - Test snapshot write/read cycle
    - _Requirements: 13.1–13.13_

  - [x] 23.3 Integration test cho Community Plan
    - Test license expiry → auto-downgrade về Community trong 60 giây
    - Test upgrade/downgrade giữ nguyên identity fields (license_key, fingerprint)
    - Test suspended + expired → giữ suspended
    - Test heartbeat response với plan enforcement
    - _Requirements: 14.1–14.10_

  - [x] 23.4 Integration test cho Install UX
    - Test full script execution với --quiet flag
    - Test TERM=dumb fallback behavior
    - Test progress bar rendering với mock download
    - Test error display khi bước thất bại
    - _Requirements: 15.1–15.11_

- [x] 24. Final checkpoint - Đảm bảo tất cả tests mới pass
  - Đảm bảo tất cả tests pass, hỏi người dùng nếu có thắc mắc.

## Notes

- Tasks đánh dấu `*` là tùy chọn và có thể bỏ qua để đạt MVP nhanh hơn
- Mỗi task tham chiếu đến requirements cụ thể để truy vết
- Checkpoints đảm bảo xác nhận tăng dần sau mỗi giai đoạn lớn
- Property tests xác nhận các thuộc tính đúng đắn phổ quát từ design document
- Tái cấu trúc thư mục (Task 1) phải hoàn thành trước vì tất cả tasks khác phụ thuộc vào cấu trúc mới
- Dự án sử dụng Go làm ngôn ngữ triển khai với `pgregory.net/rapid` cho property-based testing
- Mã XDP/eBPF viết bằng C và biên dịch riêng qua `make build-xdp`
- Install UX viết bằng Bash, tích hợp trực tiếp vào `scripts/install-client.sh`
- Tasks 1-16 đã hoàn thành, tasks 17+ là tính năng mới cho Requirements 12-15

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1"] },
    { "id": 1, "tasks": ["1.2"] },
    { "id": 2, "tasks": ["1.3"] },
    { "id": 3, "tasks": ["1.4"] },
    { "id": 4, "tasks": ["1.5"] },
    { "id": 5, "tasks": ["3.1", "7.1", "8.1", "10.1", "14.1"] },
    { "id": 6, "tasks": ["3.2", "7.2", "8.2", "10.2", "14.2"] },
    { "id": 7, "tasks": ["3.3", "7.3", "8.3", "10.3"] },
    { "id": 8, "tasks": ["4.1", "5.1", "8.4", "11.1"] },
    { "id": 9, "tasks": ["4.2", "5.2", "11.2"] },
    { "id": 10, "tasks": ["4.3", "5.3", "11.3"] },
    { "id": 11, "tasks": ["11.4", "12.1", "15.1"] },
    { "id": 12, "tasks": ["12.2", "15.2"] },
    { "id": 13, "tasks": ["12.3"] },
    { "id": 14, "tasks": ["17.1", "18.1", "19.1", "20.1"] },
    { "id": 15, "tasks": ["17.2", "18.2", "19.2", "20.2"] },
    { "id": 16, "tasks": ["17.3", "18.3", "19.3", "20.3"] },
    { "id": 17, "tasks": ["18.4"] },
    { "id": 18, "tasks": ["22.1", "22.2", "22.3", "22.4", "22.5", "22.6"] },
    { "id": 19, "tasks": ["23.1", "23.2", "23.3", "23.4"] }
  ]
}
```
