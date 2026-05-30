# Troubleshooting

## Diagnostic Commands

Trước khi debug, chạy các lệnh sau để thu thập thông tin:

```bash
# Trạng thái tổng quan
kiro-cli status --config /etc/kiro/kiro.yaml

# Health check đầy đủ
kiro-cli health --config /etc/kiro/kiro.yaml

# Preflight checks
kiro-cli preflight --config /etc/kiro/kiro.yaml

# System report
kiro-cli report --config /etc/kiro/kiro.yaml
```

---

## 1. Website Không Truy Cập Được (502/503/504)

### Triệu chứng
- Browser hiển thị 502 Bad Gateway, 503 Service Unavailable, hoặc 504 Gateway Timeout

### Nguyên nhân & Giải pháp

**Backend không chạy:**
```bash
# Kiểm tra backend
curl -I http://127.0.0.1:3000
# Nếu connection refused → start backend app
systemctl status your-app
```

**Nginx config lỗi:**
```bash
nginx -t
# Nếu lỗi → kiểm tra /etc/nginx/sites-enabled/
systemctl reload nginx
```

**Port bị block bởi nftables:**
```bash
nft list ruleset | grep -E "80|443"
# Đảm bảo port 80/443 được allow
```

**Kiro client không chạy:**
```bash
systemctl status kiro-client-waf
journalctl -u kiro-client-waf --since "5 min ago"
```

---

## 2. Không SSH Được Vào Server

### Triệu chứng
- SSH timeout hoặc connection refused sau khi cài Kiro

### Nguyên nhân & Giải pháp

**Admin IP chưa được whitelist:**
```bash
# Kiểm tra config (từ console/VNC)
grep -A5 "admin:" /etc/kiro/kiro.yaml
# Thêm IP của bạn vào allow_ips
```

**SSH port sai trong config:**
```bash
# Kiểm tra SSH port thực tế
grep "^Port" /etc/ssh/sshd_config
# Đảm bảo khớp với server.ssh_port trong kiro.yaml
```

**nftables block SSH:**
```bash
# Emergency: flush rules (từ console)
nft flush ruleset
# Sau đó fix config và re-apply
```

**Safety rollback:**
Kiro có cơ chế tự rollback sau 60 giây nếu mất kết nối admin. Đợi 60s rồi thử lại.

---

## 3. License Errors

### Triệu chứng
- Log hiển thị "license invalid" hoặc "license expired"
- Client không thể đăng ký với master

### Nguyên nhân & Giải pháp

**License key sai:**
```bash
# Kiểm tra key trong config
grep "license_key" /etc/kiro/kiro.yaml
# Đảm bảo format: KIRO-XXXX-XXXX
```

**Machine fingerprint thay đổi:**
```bash
# Xem fingerprint hiện tại
kiro-cli license fingerprint
# Nếu thay đổi (do đổi hardware/MAC), liên hệ provider để rebind
```

**Không kết nối được master:**
```bash
# Test connectivity
curl -v https://firewall.vpsgen.com/api/health
# Kiểm tra DNS, firewall outbound
```

**Community plan - auto-register fail:**
```bash
# Xem log đăng ký
journalctl -u kiro-client-waf | grep -i "register"
# Thử đăng ký lại
systemctl restart kiro-client-waf
```

---

## 4. XDP Filter Không Hoạt Động

### Triệu chứng
- `bpftool prog list` không thấy XDP program
- Traffic không bị filter ở L3/L4

### Nguyên nhân & Giải pháp

**Kernel không hỗ trợ XDP native:**
```bash
# Kiểm tra kernel version
uname -r
# Cần kernel 5.15+ cho XDP native

# Fallback sang generic mode
# Trong kiro.yaml: xdp.mode: generic
```

**XDP object file không tồn tại:**
```bash
ls -la /usr/lib/kiro/xdp/kiro_xdp_drop.o
# Nếu không có → rebuild
make build-xdp
cp build/xdp_filter.o /usr/lib/kiro/xdp/kiro_xdp_drop.o
```

**NIC driver không hỗ trợ native mode:**
```bash
# Xem driver
ethtool -i eth0 | grep driver
# virtio_net, e1000 thường hỗ trợ generic
# Dùng generic mode nếu native fail
```

**Permission denied:**
```bash
# XDP cần CAP_NET_ADMIN
# Đảm bảo service chạy với đủ quyền
grep "CapabilityBoundingSet" /etc/systemd/system/kiro-client-waf.service
```

---

## 5. Rate Limiting Quá Chặt (False Positive)

### Triệu chứng
- User hợp lệ bị 429 Too Many Requests
- API calls bị block

### Nguyên nhân & Giải pháp

**Rate limit quá thấp cho use case:**
```yaml
# Tăng rate limit trong config
protection:
  profile: light  # hoặc tăng custom

# Hoặc per-route
routes:
  - path: /api/
    backend: http://127.0.0.1:4000
    # Không set protection: strict cho API endpoints có traffic cao
```

**Shared IP (NAT/corporate):**
```bash
# Nhiều user cùng 1 IP → tăng limit
# Trong advanced config:
admission:
  default_rpm_per_ip: 300
  default_concurrent_per_ip: 20
```

**Cloudflare IP thay vì real IP:**
```bash
# Kiểm tra Nginx có restore real IP không
grep "real_ip" /etc/nginx/nginx.conf
# Cần: set_real_ip_from (Cloudflare ranges)
# Cần: real_ip_header CF-Connecting-IP
```

---

## 6. Bot Detection False Positive

### Triệu chứng
- User thật bị challenge liên tục
- Legitimate bots (Googlebot) bị block

### Nguyên nhân & Giải pháp

**Giảm bot sensitivity:**
```yaml
# Trong advanced config
bot:
  score_challenge: 70  # Tăng từ 50 lên 70
  score_block: 90      # Tăng từ 80 lên 90
```

**Whitelist legitimate bots:**
```bash
# Thêm vào allowlist
echo "66.249.0.0/16" >> /etc/kiro/allowlist.txt  # Googlebot
echo "40.77.0.0/16" >> /etc/kiro/allowlist.txt   # Bingbot
```

**Cookie/JS challenge loop:**
```bash
# Kiểm tra browser có hỗ trợ JS không
# Nếu target audience dùng text browser → tắt JS challenge
bot:
  js_challenge: false
  cookie_challenge: true
```

---

## 7. Cập Nhật Thất Bại

### Triệu chứng
- `kiro-cli update apply` báo lỗi
- Service không start sau update

### Nguyên nhân & Giải pháp

**Rollback ngay:**
```bash
kiro-cli update rollback \
  --binary-path /usr/local/bin/kiro-client \
  --service kiro-client-waf
```

**Download failed:**
```bash
# Kiểm tra kết nối master
curl -v https://firewall.vpsgen.com/api/updates/check?component=kiro-client-waf&channel=stable

# Kiểm tra disk space
df -h /usr/local/bin/
```

**Binary incompatible:**
```bash
# Kiểm tra architecture
file /usr/local/bin/kiro-client
uname -m
# Phải là x86_64 / amd64
```

---

## 8. High CPU/Memory Usage

### Triệu chứng
- kiro-client-waf chiếm nhiều CPU/RAM
- Server chậm

### Nguyên nhân & Giải pháp

**Đang bị tấn công:**
```bash
# Kiểm tra traffic
kiro-cli status --config /etc/kiro/kiro.yaml | jq '.resource_level'
# Nếu "attack" hoặc "lockdown" → đang bị DDoS

# Xem top IPs
journalctl -u kiro-client-waf | grep "blocked" | sort | uniq -c | sort -rn | head
```

**WAF rules quá nặng:**
```yaml
# Giảm WAF load
waf:
  anomaly_threshold: 8  # Tăng threshold (ít rules trigger)
```

**Rate limit không đủ chặt:**
```yaml
# Giảm concurrent connections
admission:
  default_concurrent_per_ip: 5
  default_rpm_per_ip: 60
```

**Memory leak (bug):**
```bash
# Restart service
systemctl restart kiro-client-waf

# Nếu lặp lại → báo bug với incident report
kiro-cli incident report --type runtime_security --severity medium --summary "Memory leak suspected"
```

---

## 9. Cloudflare Integration Issues

### Triệu chứng
- SSL errors (525, 526)
- Redirect loops
- Origin IP bị lộ

### Nguyên nhân & Giải pháp

**Error 525 (SSL Handshake Failed):**
```bash
# Cloudflare mode = Full/Strict nhưng origin không có SSL
# Fix: đổi Cloudflare SSL mode sang Flexible
# Hoặc: cài cert ở origin
```

**Error 526 (Invalid SSL Certificate):**
```bash
# Cloudflare mode = Full Strict nhưng cert không valid
# Fix: dùng Cloudflare Origin CA
# Hoặc: đổi sang Full (không strict)
```

**Redirect loop (ERR_TOO_MANY_REDIRECTS):**
```yaml
# Kiểm tra tls_mode trong kiro.yaml
website:
  tls_mode: flexible_http  # KHÔNG redirect HTTP→HTTPS ở origin
# Cloudflare đã handle HTTPS ở edge
```

**Origin IP bị lộ:**
```bash
# Kiểm tra DNS records không proxied
# Tất cả A/AAAA records phải có orange cloud (Proxied)

# Kiểm tra nftables block direct access
nft list ruleset | grep -A5 "input"
# Chỉ allow traffic từ Cloudflare IPs
```

---

## 10. Nginx Configuration Conflicts

### Triệu chứng
- `nginx -t` báo lỗi
- Duplicate server_name
- Port conflict

### Nguyên nhân & Giải pháp

**Duplicate server_name:**
```bash
# Tìm conflict
grep -r "server_name" /etc/nginx/sites-enabled/
# Disable config cũ
rm /etc/nginx/sites-enabled/default
nginx -t && systemctl reload nginx
```

**Port 80/443 đã bị dùng:**
```bash
# Tìm process dùng port
ss -tlnp | grep -E ":80|:443"
# Kill hoặc reconfigure process khác
```

**Permission denied trên log/socket:**
```bash
# Fix permissions
chown -R www-data:www-data /var/log/nginx/
chmod 755 /run/nginx/
```

---

## 11. Service Không Start

### Triệu chứng
- `systemctl start kiro-client-waf` fail
- Status: failed/inactive

### Nguyên nhân & Giải pháp

```bash
# Xem lỗi chi tiết
journalctl -u kiro-client-waf -n 50 --no-pager

# Common issues:
# 1. Config file không tồn tại
ls -la /etc/kiro/kiro.yaml

# 2. Binary không có execute permission
chmod +x /usr/local/bin/kiro-client

# 3. Port đã bị dùng
ss -tlnp | grep -E ":80|:443"

# 4. Dependency service chưa start
systemctl status nginx
systemctl status nftables
```

---

## Log Analysis

### Log Locations

| File | Nội dung |
|------|----------|
| `/var/log/kiro/client.log` | Kiro client WAF logs |
| `/var/log/kiro/xdp.log` | XDP filter events |
| `/var/log/nginx/error.log` | Nginx errors |
| `/var/log/nginx/access.log` | HTTP access log |
| `journalctl -u kiro-client-waf` | Systemd service logs |

### Useful Log Queries

```bash
# Xem blocked requests
journalctl -u kiro-client-waf | grep "blocked"

# Xem rate limit events
journalctl -u kiro-client-waf | grep "rate_limit"

# Xem challenge events
journalctl -u kiro-client-waf | grep "challenge"

# Xem errors trong 1 giờ qua
journalctl -u kiro-client-waf --since "1 hour ago" -p err

# Xem XDP drops
cat /var/log/kiro/xdp.log | grep "DROP" | tail -20
```

## Getting Help

Nếu không giải quyết được:

```bash
# Tạo support bundle
kiro-cli report --config /etc/kiro/kiro.yaml > /tmp/kiro-report.json

# Tạo incident report
kiro-cli incident report \
  --config /etc/kiro/kiro.yaml \
  --type other \
  --summary "Mô tả vấn đề"
```

Gửi report cho support team kèm:
1. Output của `kiro-cli health`
2. Output của `kiro-cli report`
3. Relevant logs (30 phút trước và sau sự cố)
