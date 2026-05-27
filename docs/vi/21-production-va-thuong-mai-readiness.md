# Production Và Thương Mại Readiness

## Mục tiêu

Tài liệu này định nghĩa điều kiện để `kiro_waf` chuyển từ lab/dev sang:

```text
production-ready
  Có thể triển khai cho khách hàng pilot hoặc khách hàng nhỏ với rủi ro kiểm soát được.

commercial-ready
  Có thể bán dịch vụ có thu phí, có support, update, bảo hành và trách nhiệm rõ ràng.
```

Không được đánh dấu production/commercial chỉ vì tài liệu đầy đủ. Phải có code,
test, benchmark, release và vận hành thực tế.

## Trạng thái mục tiêu

```text
Documentation-ready: Có.
Implementation-ready: Sau Phase 0-3.
Lab-ready: Sau Phase 4-5.
Pilot-ready: Sau Phase 6-8.
Production-ready: Sau khi pass production gate.
Commercial-ready: Sau khi pass production gate + business/support/legal gate.
```

## Production Gate

Một bản được gọi là production-ready phải đạt tất cả mục này.

### 1. Cài đặt và rollback

- Installer chạy được trên Ubuntu 22.04 LTS.
- Có preflight check.
- Có dry-run.
- Có rollback timer.
- Không khóa SSH/admin.
- Có uninstall sạch.
- Có restore last known good config.
- Có lab guide cho VPS trắng.

### 2. Agent ổn định

- `kiro-agent` chạy bằng systemd.
- Restart agent không mất state bảo vệ.
- Agent crash không làm mất firewall rule đang hoạt động.
- Config lỗi không apply.
- Event log có rate limit.
- Disk log không phình vô hạn.

### 3. Provider/agent tách biệt

- Agent không import provider package.
- Provider không import agent firewall/eBPF package.
- Provider private key không xuất hiện trên protected server.
- License verify local hoạt động khi provider tạm offline.
- Activation/rebind/revoke có audit log.

### 4. Bảo vệ server

- nftables apply/rollback pass trong lab.
- XDP blocklist/allowlist hoạt động.
- Dynamic temp block có TTL.
- Admin allowlist không bị block.
- Conntrack pressure test pass.
- UDP/ICMP/SYN protection pass trong lab.

### 5. Bảo vệ website

- Nginx/HAProxy config generator pass.
- 1 domain nhiều backend pass.
- Nhiều domain một backend pass.
- Route-based backend pass.
- Flexible HTTP pass.
- Full Strict pass.
- Cloudflare origin lock IPv4/IPv6 pass.
- Direct origin HTTP/HTTPS bị chặn khi bật Cloudflare.

### 6. WAF/bot

- OWASP CRS integration pass.
- SQLi/XSS/path traversal payload phổ biến bị detect.
- False positive có cơ chế exception theo route.
- Bot không giữ cookie bị challenge/block.
- Allowlist không bị challenge nhầm.
- Challenge không tạo vòng lặp vô hạn.

### 7. Overload/resource governor

- Baseline learning hoạt động.
- Hysteresis/cooldown hoạt động.
- Không nhảy mode liên tục.
- CPU/RAM/conntrack/backend latency trigger đúng.
- Khi attack dừng, hệ thống tự hạ mode.
- Backend chậm thì proxy trả 429/503 sớm.

### 8. Update

- Manifest ký số.
- Checksum artifact.
- Không cho downgrade ngoài policy.
- Update fail rollback.
- Binary crash rollback.
- Policy update sai schema bị từ chối.

### 9. Runtime security

- Web user chạy shell tạo alert.
- Web process gọi `curl/wget/nc` tạo alert.
- File lạ trong webroot tạo alert.
- Support bundle redact secret.
- Không gửi request body/cookie/token mặc định.

### 10. Benchmark

Phải có benchmark tối thiểu cho:

```text
1 vCPU / 1 GB RAM
2 vCPU / 4 GB RAM
4 vCPU / 8 GB RAM
8 vCPU / 16 GB RAM
```

Chỉ số bắt buộc:

- XDP PPS drop.
- nftables dynamic block scale.
- HTTP RPS WAF off/on.
- Latency p50/p95/p99.
- CPU/RAM khi attack lab.
- Conntrack usage.
- Thời gian apply rule.
- Thời gian rollback.

## Commercial Gate

Một bản được gọi là commercial-ready phải đạt production gate và các mục sau.

### 1. Gói dịch vụ rõ ràng

Phải có mô tả:

- Community.
- School/SMB.
- Professional.
- Enterprise-lite.

Mỗi gói phải ghi rõ:

- Feature có/không.
- Giới hạn support.
- Giới hạn server/domain.
- Update channel.
- Chính sách rebind.

### 2. SLA/SLO thực tế

Không cam kết chống mọi DDoS. Chỉ cam kết phần mình kiểm soát:

```text
Support response time
Update security delivery
Rollback behavior
Agent health check
License activation
Incident report
```

Ví dụ SLO nội bộ:

- License activation < 5 phút nếu provider online.
- Security update phát hành trong 24-72 giờ tùy severity.
- Rollback config < 30 giây trong lab.
- Support bundle tạo < 60 giây.

### 3. Chính sách giới hạn sản phẩm

Tài liệu bán hàng phải ghi rõ:

- Không thay thế upstream bandwidth DDoS protection.
- Cloudflare Free chỉ bảo vệ traffic đi qua Cloudflare.
- Flexible HTTP không mã hóa Cloudflare -> origin.
- WAF không sửa lỗi logic ứng dụng.
- Hiệu quả phụ thuộc cấu hình, tài nguyên server và loại tấn công.

### 4. Privacy và dữ liệu

Phải có privacy statement:

- Mặc định telemetry tắt.
- Không gửi request body.
- Không gửi cookie/token/auth header.
- IP client hash hoặc rút gọn khi không cần incident detail.
- Support bundle redact secret.
- Health report retention rõ.
- Khách hàng có quyền xóa dữ liệu support.

### 5. Security policy

Phải có:

- Cách report lỗ hổng.
- PGP/email/security contact.
- Severity classification.
- Thời gian phản hồi.
- Quy trình phát hành security update.
- Không public exploit detail trước khi có patch.

### 6. Release management

Mỗi release phải có:

- Version.
- Changelog.
- Signed artifact.
- Signed manifest.
- Checksum.
- Migration note.
- Rollback note.
- Compatibility matrix.

### 7. Support và bảo hành

Phải có:

- Support bundle command.
- Incident report template.
- Runbook khi bị attack.
- Runbook khi mất SSH.
- Runbook khi update lỗi.
- Runbook khi lộ origin IP.
- Runbook khi license/rebind lỗi.

### 8. Pháp lý và hợp đồng

Tối thiểu cần:

- Terms of service.
- Acceptable use policy.
- Privacy policy.
- Data processing note nếu thu thập telemetry.
- Disclaimer giới hạn DDoS.
- Chính sách hoàn tiền/bảo hành.

## Pilot Gate

Trước khi bán rộng, cần pilot:

```text
3-5 server lab nội bộ
3-5 khách hàng thân thiết
ít nhất 30 ngày chạy ổn định
ít nhất 1 lần update/rollback thành công
ít nhất 1 incident report giả lập
không có lỗi khóa SSH hoặc làm chết website do agent
```

## Ma trận quyết định go/no-go

```text
Code build pass                      bắt buộc
Unit/integration test pass            bắt buộc
Firewall rollback pass                bắt buộc
Proxy reload rollback pass            bắt buộc
License/update signature pass         bắt buộc
Support bundle redact pass            bắt buộc
Benchmark có số liệu                  bắt buộc
Pilot 30 ngày ổn định                 bắt buộc trước commercial
Dashboard đẹp                         không bắt buộc cho MVP
Database provider                     không bắt buộc cho MVP
ML detection                          không bắt buộc cho MVP
```

## Kết luận

Sau khi có tài liệu này:

```text
Tài liệu định hướng production: Đủ.
Tài liệu định hướng thương mại: Đủ.
Sản phẩm production thật: Chỉ đạt sau khi implementation + test gate pass.
Sản phẩm bán thương mại: Chỉ đạt sau production gate + pilot + support/legal.
```

