# Threat Model Và Yêu Cầu Hệ Thống

## Mục tiêu tài liệu

Tài liệu này chốt phạm vi trước khi bắt đầu code để đội phát triển không bị lạc
hướng. Mọi module được tạo sau này phải trả lời được:

- Nó bảo vệ tài sản nào?
- Nó chống loại tấn công nào?
- Nó có giới hạn gì?
- Nó được test bằng cách nào?
- Khi lỗi thì rollback hoặc fail-safe ra sao?

## Tài sản cần bảo vệ

```text
server availability
  CPU, RAM, disk, network, conntrack, file descriptor, process table.

website availability
  Nginx/HAProxy, app workers, backend pool, route quota, cache.

application data
  Database ứng dụng, file upload, backup, secret, token, session.

origin identity
  IP gốc của website khi dùng Cloudflare Free.

license integrity
  License, key list, machine fingerprint, update entitlement.

update integrity
  Binary, policy bundle, WAF rules, manifest, rollback artifact.

provider operations
  Customer list, server list, incident history, support bundle.
```

## Đối tượng tấn công

```text
network attacker
  SYN flood, UDP flood, ICMP abuse, port scan, malformed packet.

botnet attacker
  Nhiều IP gửi request nhỏ nhưng tổng lớn, đánh route nặng, giữ connection lâu.

web attacker
  SQLi, XSS, path traversal, command injection, upload attack, SSRF pattern.

origin bypass attacker
  Bỏ qua Cloudflare và đánh trực tiếp vào IP gốc.

post-exploitation attacker
  Webshell, reverse shell, sửa cron/systemd, đọc secret, outbound lạ.

license abuse attacker
  Dùng một license cho nhiều máy, sửa license file, downgrade update.

operator mistake
  Cấu hình sai firewall, khóa mất SSH, apply nhầm domain/backend.
```

## Ngoài phạm vi của MVP

MVP không cam kết:

- Chống DDoS vượt băng thông đường truyền.
- Thay thế CDN/scrubbing chuyên dụng.
- Chống zero-day ứng dụng chưa có rule hoặc hành vi nhận diện được.
- Sửa lỗi logic của ứng dụng khách hàng.
- Bảo vệ dịch vụ không đi qua server đang cài `kiro_waf`.
- Quản trị hàng nghìn server bằng file storage mà không nâng cấp provider backend.

## Yêu cầu bắt buộc

### An toàn vận hành

- Không được apply firewall nếu chưa có admin allowlist hợp lệ.
- Mọi thay đổi firewall/proxy phải có dry-run.
- Mọi thay đổi nguy hiểm phải có rollback timer.
- Luôn giữ last known good config.
- Agent restart không được làm mất rule đang bảo vệ.
- Config lỗi phải bị từ chối trước khi apply.

### Bảo vệ server

- Chặn được IP/blocklist ở tầng sớm nhất có thể.
- Có allowlist không bị block nhầm.
- Có profile `NORMAL`, `ELEVATED`, `ATTACK`, `LOCKDOWN`.
- Có cooldown/hysteresis để không nhảy mode liên tục.
- Có giới hạn dynamic block để tránh tự làm cạn RAM.
- Có log rate limit.

### Bảo vệ website

- Hỗ trợ 1 domain nhiều backend.
- Hỗ trợ nhiều domain một backend.
- Hỗ trợ route-based backend.
- Hỗ trợ route quota.
- Hỗ trợ cache static asset.
- Hỗ trợ WAF bằng Coraza hoặc ModSecurity.
- Hỗ trợ Cloudflare origin lock cả IPv4 và IPv6.
- Chỉ tin `CF-Connecting-IP` khi source thuộc Cloudflare ranges.
- Hỗ trợ `flexible_http` không cần cert/key cho setup đơn giản.
- Hỗ trợ `full_strict` với cert/key cho production cần bảo mật cao hơn.
- Cảnh báo khi dùng `flexible_http` cho route login/admin/dữ liệu nhạy cảm.

### Bản quyền và update

- License phải được ký số.
- Agent chỉ giữ provider public key.
- Machine binding không chỉ dựa vào MAC.
- Có grace period rõ ràng.
- Có rebind workflow.
- Update manifest phải ký số.
- Artifact phải check checksum.
- Update fail phải rollback.

### Privacy

- Không gửi request body mặc định.
- Không gửi cookie, authorization header, password, token.
- IP client nên hash nếu chỉ dùng thống kê dài hạn.
- Support bundle phải redact secret.
- Telemetry phải có công tắc bật/tắt.

## Yêu cầu nên có sau MVP

- Dashboard local.
- Wizard cài đặt.
- Policy template theo trường học/SMB/hosting nhỏ.
- Staged update.
- Local RBAC.
- Signed audit log.
- Export report.
- Provider database tùy chọn khi vượt giới hạn file-based.

## Tiêu chí thiết kế

Ưu tiên theo thứ tự:

1. Không khóa mất server.
2. Drop traffic xấu sớm.
3. Giữ app/backend sống.
4. Cấu hình dễ hiểu.
5. Có rollback.
6. Có số liệu kiểm chứng.
7. Có thể hỗ trợ khách hàng lâu dài.
