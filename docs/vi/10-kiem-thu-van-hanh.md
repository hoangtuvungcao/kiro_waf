# Kiểm Thử Và Vận Hành

## Nguyên tắc

Không được coi `kiro_waf` là chống tấn công tốt nếu chưa có kiểm thử lặp lại
được. Sản phẩm bảo mật cần số liệu, log và quy trình phục hồi.

## Nhóm kiểm thử bắt buộc

### Config safety

- Config YAML sai phải bị từ chối.
- License sai chữ ký phải bị từ chối.
- Mode `full` không có site config phải bị từ chối.
- Cloudflare origin lock không được bật nếu admin IP chưa an toàn.
- Firewall apply lỗi phải rollback.

### Network/L3-L4

- Blocklist/allowlist.
- SYN flood giả lập.
- UDP flood giả lập trong lab.
- ICMP rate limit.
- Port scan.
- Bogon/private source drop.
- Conntrack pressure.

### Website/L7

- HTTP flood vào `/`.
- HTTP flood vào `/login`.
- Attack vào route nặng `/api/search`.
- Upload body lớn.
- Slowloris.
- Bot không giữ cookie.
- Bot vào path rác `/.env`, `/.git`, `/wp-login.php`.

### WAF/API

- SQL injection payload phổ biến.
- XSS payload phổ biến.
- Path traversal.
- Command injection pattern.
- JSON schema sai.
- Method không được phép.

### Runtime security

- Web user chạy shell.
- Web process gọi `curl`, `wget`, `nc`.
- Ghi file lạ vào webroot.
- Sửa cron/systemd unit.
- Đọc `.env` bất thường.

### Update/rollback

- Manifest sai chữ ký.
- Artifact sai checksum.
- Update bị ngắt giữa chừng.
- Binary mới crash.
- Config migration lỗi.
- Rollback thành công.

## Benchmark tối thiểu

Cần công bố theo từng loại server:

```text
1 vCPU / 1 GB RAM
2 vCPU / 4 GB RAM
4 vCPU / 8 GB RAM
8 vCPU / 16 GB RAM
```

Chỉ số:

- PPS drop ở XDP.
- Request/giây khi WAF tắt.
- Request/giây khi WAF bật.
- Latency p50/p95/p99.
- CPU/RAM khi attack.
- Conntrack usage.
- Số IP block động tối đa ổn định.
- Thời gian apply rule.
- Thời gian rollback.

## SLO đề xuất cho SMB/trường học

Không nên cam kết tuyệt đối. Có thể cam kết nội bộ:

```text
Agent restart < 5 giây
Config apply rollback < 30 giây
CLI status phản hồi < 2 giây
License verify local < 1 giây
Update rollback tự động nếu health check fail
Không apply firewall nếu có nguy cơ khóa SSH
```

## Quy trình vận hành ngày thường

- Kiểm tra `kiro health`.
- Kiểm tra license hết hạn.
- Kiểm tra update security.
- Kiểm tra disk/log.
- Kiểm tra Cloudflare origin lock nếu full mode.
- Kiểm tra support bundle định kỳ.

## Quy trình khi đang bị tấn công

```text
1. Xác định mode và defense level.
2. Xem top IP/subnet/path/domain.
3. Bật ELEVATED hoặc ATTACK nếu agent chưa tự bật.
4. Với full mode, kiểm tra traffic có đi qua Cloudflare không.
5. Tạm thời giảm quota route nặng.
6. Bảo vệ SSH/admin.
7. Xuất incident report.
8. Sau khi attack dừng, hạ mode theo cooldown.
```

## Điều kiện pass trước khi phát hành

- Tất cả test safety pass.
- Không có test nào khóa SSH trong lab.
- Benchmark có số liệu.
- Update/rollback pass.
- Support bundle không lộ secret.
- Tài liệu giới hạn sản phẩm rõ ràng.

