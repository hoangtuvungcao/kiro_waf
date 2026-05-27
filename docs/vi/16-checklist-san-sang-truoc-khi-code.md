# Checklist Sẵn Sàng Trước Khi Code

## Kết luận hiện tại

Sau khi bổ sung tài liệu này, hệ thống tài liệu đã đủ để bắt đầu khởi tạo code
theo từng phase. Chưa đủ để bán sản phẩm, nhưng đủ để dev không bị lạc hướng.

## Checklist tài liệu

- [x] Tổng quan sản phẩm.
- [x] Kiến trúc `server` và `full`.
- [x] Domain/backend mapping.
- [x] License/key list.
- [x] Cloudflare Free và origin lock.
- [x] Update/bảo hành.
- [x] Bảo mật hệ thống.
- [x] Đánh giá khả thi doanh nghiệp.
- [x] Roadmap.
- [x] Test/vận hành.
- [x] Gói triển khai/community.
- [x] Căn cứ công nghệ.
- [x] Threat model.
- [x] Cấu trúc code/module.
- [x] Kế hoạch khởi tạo và test từng bước.
- [x] SSL/TLS domain, key và pem.
- [x] Phân tách provider server và server khách hàng.
- [x] Cấu hình tối giản cho mô hình cho thuê.
- [x] Production/commercial readiness gate.
- [x] PRD/SRS sản phẩm.

## Checklist trước khi viết code

- [x] Chọn ngôn ngữ chính: Go.
- [x] Chốt Go module name.
- [x] Chốt binary names: `kiro-agent`, `kiro-cli`, `kiro-provider`.
- [x] Chốt ranh giới import: agent không import provider, provider không import agent firewall/ebpf.
- [x] Chốt format config v1.
- [x] Chốt cơ chế expand simple config -> runtime advanced config.
- [x] Tạo test fixtures cho `kiro.example.yaml`, `tenant.*.example.yaml`, `kiro.advanced.example.yaml`.
- [x] Chốt public/private key format cho license.
- [x] Tạo test fixtures cho config hợp lệ/lỗi.
- [x] Tạo fake license dùng trong unit test.
- [ ] Tạo lab notes cho Ubuntu 22.04 LTS.

## Checklist trước khi apply firewall thật

- [ ] Có admin IP allowlist.
- [ ] Có console recovery path.
- [x] Có dry-run.
- [ ] Có rollback timer.
- [ ] Có last known good config.
- [ ] Đã test trong lab, không test trực tiếp trên máy production.

## Checklist trước khi bật full mode

- [ ] Domain trỏ đúng.
- [ ] Backend health check pass.
- [ ] Nginx config validate pass.
- [ ] Nếu dùng Cloudflare, chọn `flexible_http` hoặc `full_strict`.
- [ ] Nếu `flexible_http`, không yêu cầu cert/key nhưng cảnh báo khi có login hoặc dữ liệu nhạy cảm.
- [ ] Nếu `full_strict`, cert/key phải tồn tại và đúng domain.
- [ ] Nếu dùng Cloudflare, cả IPv4 và IPv6 origin lock đã bật.
- [ ] `CF-Connecting-IP` chỉ được tin từ Cloudflare IP.
- [ ] Direct origin test bị chặn.

## Checklist trước khi phát hành bản MVP

- [ ] `go test ./...` pass.
- [ ] Config validation pass.
- [ ] Simple config expansion pass.
- [ ] License validation pass.
- [ ] Agent binary không chứa code issue/sign license.
- [ ] Provider binary không chứa code apply firewall/XDP.
- [x] Firewall dry-run pass.
- [ ] Proxy generator pass.
- [ ] Update rollback pass.
- [ ] Support bundle redact secret.
- [ ] Tài liệu cài đặt có thể làm theo từ đầu đến cuối.

## Checklist trước khi gọi là production-ready

- [ ] Installer có preflight check.
- [ ] Firewall dry-run/rollback pass trên Ubuntu lab thật.
- [ ] Không khóa SSH trong lab.
- [ ] Agent systemd restart/crash behavior pass.
- [ ] Provider/agent import boundary pass.
- [ ] Cloudflare origin lock IPv4/IPv6 pass.
- [ ] Flexible HTTP và Full Strict pass.
- [ ] WAF/bot test pass.
- [ ] Governor hysteresis/cooldown pass.
- [ ] Update rollback pass.
- [ ] Support bundle redact pass.
- [ ] Benchmark có số liệu.

## Checklist trước khi gọi là commercial-ready

- [ ] Production gate pass.
- [ ] Pilot 30 ngày ổn định.
- [ ] Gói dịch vụ và giới hạn rõ.
- [ ] SLA/SLO thực tế.
- [ ] Privacy policy.
- [ ] Security vulnerability reporting policy.
- [ ] Terms of service.
- [ ] Runbook support/bảo hành.
- [ ] Changelog/release/signature process.

## Quy tắc không lạc hướng

- Không thêm dashboard trước khi agent/config/license/firewall/proxy ổn.
- Không thêm database provider trước khi file-based chạm giới hạn thật.
- Không thêm ML trước khi rule/baseline/hysteresis hoạt động.
- Không quảng cáo chống DDoS mạnh nếu chưa có benchmark.
- Không apply thay đổi hệ thống nếu chưa có rollback.
- Không thu thập dữ liệu khách hàng nếu chưa có privacy switch.
