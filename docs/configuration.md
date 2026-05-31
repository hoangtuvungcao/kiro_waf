# Configuration

## Overview

Kiro WAF sử dụng **YAML** làm phương thức cấu hình chính cho cả `kiro-client-waf` và `kiro-cli`. Cả hai tool đều đọc từ cùng một file: `/etc/kiro/kiro.yaml`.

**Thứ tự ưu tiên (cao → thấp):**

1. **Environment variables** — override YAML values khi được set (non-empty)
2. **YAML config file** (`/etc/kiro/kiro.yaml`) — nguồn cấu hình chính
3. **Built-in defaults** — giá trị mặc định trong code

**Quy tắc:**
- Cả `kiro-client-waf` và `kiro-cli` đều đọc `/etc/kiro/kiro.yaml`
- Environment variables vẫn hoạt động và override YAML values (backward compatibility)
- Install script tự động tạo YAML config cho cài đặt mới
- File `.env` legacy vẫn được hỗ trợ nhưng khuyến nghị migrate sang YAML

## Unified YAML Configuration

File: `/etc/kiro/kiro.yaml`

Đây là file cấu hình duy nhất cho toàn bộ hệ thống Kiro trên mỗi node. Binary `kiro-client-waf` đọc file này qua flag `--config` (mặc định: `/etc/kiro/kiro.yaml`).

```yaml
# /etc/kiro/kiro.yaml — Cấu hình thống nhất cho kiro-client-waf và kiro-cli
mode: full
plan: school_smb
license_key: KIRO-XXXX-XXXX

admin:
  allow_ips:
    - 203.0.113.10/32

website:
  enabled: true
  cloudflare: true
  tls_mode: flexible_http
  sites:
    - domains: [example.com, www.example.com]
      backend: http://127.0.0.1:3000

protection:
  profile: balanced

# WAF-specific runtime settings (dùng bởi kiro-client-waf)
client:
  cookie_secret: "random-40-char-secret"
  master_url: https://firewall.vpsgen.com
  listen_addr: ":8090"
  node_id: my-server
  heartbeat_seconds: 60
  update_seconds: 300
```


## Client YAML Section (`client`)

Section `client` chứa các cấu hình runtime dành riêng cho `kiro-client-waf`. Đây là phần mở rộng so với schema tenant YAML hiện có.

### Required Fields

| YAML Path | Env Var | Default | Mô tả |
|-----------|---------|---------|--------|
| `license_key` | `KIRO_LICENSE_KEY` | — (required) | License key (fatal if empty) |
| `website.sites[0].backend` | `KIRO_BACKEND_URL` | — (required) | Backend URL to proxy to |
| `client.master_url` | `KIRO_MASTER_URL` | — (required) | Master server URL |
| `client.cookie_secret` | `KIRO_CLIENT_COOKIE_SECRET` | — (required) | HMAC cookie secret (40 chars) |

### Network & Identity

| YAML Path | Env Var | Default | Mô tả |
|-----------|---------|---------|--------|
| `client.listen_addr` | `KIRO_CLIENT_LISTEN` | `:8090` | Listen address cho WAF proxy |
| `client.node_id` | `KIRO_NODE_ID` | hostname | Node identifier cho heartbeat |
| `admin.allow_ips` | `KIRO_ADMIN_IPS` | `[]` | IP/CIDR list bypass lockdown |

### Rate Limiting & Protection

| YAML Path | Env Var | Default | Mô tả |
|-----------|---------|---------|--------|
| `client.rpm_per_ip` | `KIRO_RPM_PER_IP` | from profile | Requests/minute per IP |
| `client.subnet_rpm` | `KIRO_SUBNET_RPM` | from profile | Requests/minute per /24 subnet |
| `client.hard_block_after` | `KIRO_HARD_BLOCK_AFTER` | from profile | RPM threshold for hard block |
| `client.block_ttl_seconds` | `KIRO_BLOCK_TTL_SECONDS` | `900` | Ban duration (seconds) |

> **Note:** Nếu `client.rpm_per_ip`, `client.subnet_rpm`, hoặc `client.hard_block_after` không được set, giá trị mặc định lấy từ `protection.profile`. Xem bảng [Protection Profiles](#protection-profiles).

### Challenge & PoW

| YAML Path | Env Var | Default | Mô tả |
|-----------|---------|---------|--------|
| `client.pow_difficulty` | `KIRO_POW_DIFFICULTY` | `4` | Proof-of-Work difficulty (leading zeros) |
| `client.hold_seconds` | `KIRO_HOLD_SECONDS` | `2` | Hold page duration (seconds) |
| `client.challenge_all_new` | `KIRO_CHALLENGE_ALL_NEW` | `false` | Challenge all new visitors |
| `client.transparent_ttl` | `KIRO_TRANSPARENT_TTL` | `30` | Transparent challenge token TTL (seconds) |
| `client.cookie_short_ttl` | `KIRO_COOKIE_SHORT_TTL` | `300` | Short-lived cookie TTL (seconds) |
| `client.escalation_threshold` | `KIRO_ESCALATION_THRESHOLD` | `3` | Failures before escalating challenge |
| `client.escalation_cooldown` | `KIRO_ESCALATION_COOLDOWN` | `600` | Cooldown before de-escalating (seconds) |
| `client.cookie_rate_limit` | `KIRO_COOKIE_RATE_LIMIT` | `300` | Max cookie validations per IP per window |

### XDP & Geo Blocking

| YAML Path | Env Var | Default | Mô tả |
|-----------|---------|---------|--------|
| `client.blocklist_file` | `KIRO_XDP_BLOCKLIST_FILE` | `/var/lib/kiro/xdp-blocklist.txt` | XDP blocklist file path |
| `client.xdp_sync_command` | `KIRO_XDP_SYNC_COMMAND` | `""` | Command to sync XDP blocklist |
| `client.xdp_blocked_countries` | `KIRO_XDP_BLOCKED_COUNTRIES` | `""` | Comma-separated country codes |
| `client.geoip_csv_path` | `KIRO_GEOIP_CSV_PATH` | `""` | Path to GeoIP CSV file |

### Heartbeat & Updates

| YAML Path | Env Var | Default | Mô tả |
|-----------|---------|---------|--------|
| `client.heartbeat_seconds` | `KIRO_HEARTBEAT_SECONDS` | `60` | Heartbeat interval to master |
| `client.update_seconds` | `KIRO_UPDATE_SECONDS` | `300` | Update check interval |

### Cloudflare

| YAML Path | Env Var | Default | Mô tả |
|-----------|---------|---------|--------|
| `client.cf_trust_mode` | `KIRO_CF_TRUST_MODE` | `strict` | Cloudflare trust mode: `strict`, `permissive`, `off` |


## Environment Variable Override

Environment variables vẫn hoạt động và **override** giá trị YAML tương ứng. Điều này cho phép:
- Migration dần dần từ `.env` sang YAML mà không cần downtime
- Override tạm thời trong testing/debugging
- Sử dụng systemd `Environment=` directive cho secrets

**Quy tắc override:**
- Env var được set và **non-empty** → override YAML value
- Env var set thành empty string (`""`) → coi như unset, dùng YAML value
- Env var không tồn tại → dùng YAML value
- Cả YAML và env var đều không có → dùng built-in default

**Ví dụ:** Override listen address tạm thời:
```bash
KIRO_CLIENT_LISTEN=":9090" /usr/local/bin/kiro-client-waf --config /etc/kiro/kiro.yaml
```

### Legacy Mode (Env-Only)

Nếu file YAML không tồn tại nhưng environment variables có sẵn, `kiro-client-waf` sẽ load cấu hình từ env vars (chế độ legacy). Một warning sẽ được log khuyến nghị migrate sang YAML.

## Migration Guide: `.env` → YAML

### Bước 1: Backup file `.env` hiện tại

```bash
cp /etc/kiro/kiro-client.env /etc/kiro/kiro-client.env.bak
```

### Bước 2: Tạo file YAML từ giá trị `.env`

Chuyển đổi từng biến môi trường sang YAML tương ứng:

**Trước (`.env`):**
```bash
KIRO_LICENSE_KEY=KIRO-XXXX-XXXX
KIRO_CLIENT_COOKIE_SECRET=my-secret-cookie-key-40chars
KIRO_MASTER_URL=https://firewall.vpsgen.com
KIRO_BACKEND_URL=http://127.0.0.1:3000
KIRO_CLIENT_LISTEN=:8090
KIRO_RPM_PER_IP=120
KIRO_SUBNET_RPM=1800
KIRO_HARD_BLOCK_AFTER=360
KIRO_CHALLENGE_ALL_NEW=false
KIRO_ADMIN_IPS=203.0.113.10/32,198.51.100.5/32
```

**Sau (YAML):**
```yaml
mode: full
license_key: KIRO-XXXX-XXXX

admin:
  allow_ips:
    - 203.0.113.10/32
    - 198.51.100.5/32

website:
  enabled: true
  cloudflare: true
  tls_mode: flexible_http
  sites:
    - domains: [example.com]
      backend: http://127.0.0.1:3000

protection:
  profile: balanced

client:
  cookie_secret: "my-secret-cookie-key-40chars"
  master_url: https://firewall.vpsgen.com
  listen_addr: ":8090"
  rpm_per_ip: 120
  subnet_rpm: 1800
  hard_block_after: 360
  challenge_all_new: false
```

### Bước 3: Bảng chuyển đổi đầy đủ

| Env Var | YAML Path |
|---------|-----------|
| `KIRO_LICENSE_KEY` | `license_key` |
| `KIRO_BACKEND_URL` | `website.sites[0].backend` |
| `KIRO_MASTER_URL` | `client.master_url` |
| `KIRO_CLIENT_COOKIE_SECRET` | `client.cookie_secret` |
| `KIRO_CLIENT_LISTEN` | `client.listen_addr` |
| `KIRO_NODE_ID` | `client.node_id` |
| `KIRO_ADMIN_IPS` | `admin.allow_ips` (array) |
| `KIRO_RPM_PER_IP` | `client.rpm_per_ip` |
| `KIRO_SUBNET_RPM` | `client.subnet_rpm` |
| `KIRO_HARD_BLOCK_AFTER` | `client.hard_block_after` |
| `KIRO_POW_DIFFICULTY` | `client.pow_difficulty` |
| `KIRO_HOLD_SECONDS` | `client.hold_seconds` |
| `KIRO_BLOCK_TTL_SECONDS` | `client.block_ttl_seconds` |
| `KIRO_XDP_BLOCKLIST_FILE` | `client.blocklist_file` |
| `KIRO_XDP_SYNC_COMMAND` | `client.xdp_sync_command` |
| `KIRO_HEARTBEAT_SECONDS` | `client.heartbeat_seconds` |
| `KIRO_UPDATE_SECONDS` | `client.update_seconds` |
| `KIRO_CHALLENGE_ALL_NEW` | `client.challenge_all_new` |
| `KIRO_TRANSPARENT_TTL` | `client.transparent_ttl` |
| `KIRO_COOKIE_SHORT_TTL` | `client.cookie_short_ttl` |
| `KIRO_ESCALATION_THRESHOLD` | `client.escalation_threshold` |
| `KIRO_ESCALATION_COOLDOWN` | `client.escalation_cooldown` |
| `KIRO_COOKIE_RATE_LIMIT` | `client.cookie_rate_limit` |
| `KIRO_CF_TRUST_MODE` | `client.cf_trust_mode` |
| `KIRO_XDP_BLOCKED_COUNTRIES` | `client.xdp_blocked_countries` |
| `KIRO_GEOIP_CSV_PATH` | `client.geoip_csv_path` |

### Bước 4: Kiểm tra cấu hình

```bash
# Validate YAML syntax
kiro-cli status

# Restart service
sudo systemctl restart kiro-client-waf

# Kiểm tra logs
sudo journalctl -u kiro-client-waf -f
```

### Bước 5: Xóa file `.env` (tùy chọn)

Sau khi xác nhận YAML hoạt động đúng, có thể xóa file `.env` legacy:

```bash
sudo rm /etc/kiro/kiro-client.env
```

> **Lưu ý:** Không cần xóa ngay. Env vars vẫn override YAML values nếu cả hai tồn tại.


## Master Server Environment Variables

File: `/etc/kiro-master/master.env`

| Variable | Default | Required | Mô tả |
|----------|---------|----------|--------|
| `KIRO_MASTER_ADDR` | `:8080` | No | Listen address (host:port) |
| `KIRO_MASTER_DB` | `/var/lib/kiro-master/master.db` | No | SQLite database path |
| `KIRO_MASTER_ADMIN_KEY` | — | **Yes** | Admin API key (fatal if empty) |
| `KIRO_MASTER_ADMIN_IPS` | `""` | No | Comma-separated admin IP allowlist |
| `KIRO_MASTER_SESSION_TTL` | `12h` | No | Admin session TTL (Go duration) |

**Example:**
```bash
KIRO_MASTER_ADDR=127.0.0.1:8080
KIRO_MASTER_DB=/var/lib/kiro-master/master.db
KIRO_MASTER_ADMIN_KEY=your-secret-admin-key-here
KIRO_MASTER_ADMIN_IPS=203.0.113.10,198.51.100.5
KIRO_MASTER_SESSION_TTL=12h
```

## Top-level YAML Options

| Key | Type | Default | Mô tả |
|-----|------|---------|--------|
| `mode` | string | `full` | Chế độ: `server` (chỉ firewall) hoặc `full` (firewall + reverse proxy) |
| `plan` | string | `community` | Gói: `community`, `school_smb`, `professional`, `enterprise_lite` |
| `license_key` | string | `""` | License key (để trống cho community) |

### admin

| Key | Type | Default | Mô tả |
|-----|------|---------|--------|
| `admin.allow_ips` | []string | `[]` | Danh sách IP/CIDR được phép quản trị |

### server

| Key | Type | Default | Mô tả |
|-----|------|---------|--------|
| `server.interface` | string | `eth0` | Network interface chính |
| `server.ssh_port` | int | `22` | SSH port (luôn được allow) |

### website

| Key | Type | Default | Mô tả |
|-----|------|---------|--------|
| `website.enabled` | bool | `true` | Bật reverse proxy |
| `website.cloudflare` | bool | `true` | Tích hợp Cloudflare |
| `website.tls_mode` | string | `flexible_http` | TLS mode: `flexible_http`, `full_tls`, `full_strict` |
| `website.sites` | []Site | `[]` | Danh sách website |

#### Site Object

| Key | Type | Mô tả |
|-----|------|--------|
| `domains` | []string | Danh sách domain |
| `backend` | string | Backend URL (e.g. `http://127.0.0.1:3000`) |
| `routes` | []Route | Route-specific config (optional) |

#### Route Object

| Key | Type | Mô tả |
|-----|------|--------|
| `path` | string | URL path prefix |
| `backend` | string | Backend URL cho route này |
| `protection` | string | Protection level: `light`, `balanced`, `strict` |

### protection

| Key | Type | Default | Mô tả |
|-----|------|---------|--------|
| `protection.profile` | string | `balanced` | Profile: `light`, `balanced`, `strict`, `lockdown` |
| `protection.waf` | bool | `true` | Bật WAF engine |
| `protection.bot` | bool | `true` | Bật bot detection |
| `protection.auto_attack_mode` | bool | `true` | Tự động chuyển mode khi bị tấn công |

### updates

| Key | Type | Default | Mô tả |
|-----|------|---------|--------|
| `updates.auto_security_updates` | bool | `true` | Tự động cập nhật bảo mật |

### telemetry

| Key | Type | Default | Mô tả |
|-----|------|---------|--------|
| `telemetry.enabled` | bool | `false` | Gửi telemetry về provider |


## Advanced Configuration Reference

### paths

| Key | Type | Default | Mô tả |
|-----|------|---------|--------|
| `paths.state_dir` | string | `/var/lib/kiro` | Thư mục state |
| `paths.log_dir` | string | `/var/log/kiro` | Thư mục log |
| `paths.run_dir` | string | `/run/kiro` | Thư mục runtime |
| `paths.last_good_config_dir` | string | `/var/lib/kiro/last-good-config` | Backup config |

### safety

| Key | Type | Default | Mô tả |
|-----|------|---------|--------|
| `safety.dry_run_before_apply` | bool | `true` | Dry-run trước khi áp dụng firewall rules |
| `safety.require_admin_ip_before_firewall_apply` | bool | `true` | Bắt buộc có admin IP |
| `safety.rollback_timer_seconds` | int | `60` | Tự rollback nếu mất kết nối |
| `safety.keep_last_good_configs` | int | `5` | Số config backup giữ lại |
| `safety.never_block_admin_ips` | bool | `true` | Không bao giờ block admin IP |

### server_protection.xdp

| Key | Type | Default | Mô tả |
|-----|------|---------|--------|
| `xdp.enabled` | bool | `true` | Bật XDP filter |
| `xdp.mode` | string | `native` | Mode: `native`, `generic`, `offload` |
| `xdp.program_path` | string | `/usr/lib/kiro/xdp/kiro_xdp_drop.o` | Path đến XDP object |
| `xdp.drop_private_source_ip` | bool | `true` | Drop spoofed private IPs |
| `xdp.drop_malformed` | bool | `true` | Drop malformed packets |
| `xdp.drop_fragments` | bool | `true` | Drop fragmented packets |
| `xdp.allowlist_file` | string | `/etc/kiro/allowlist.txt` | File allowlist IP |
| `xdp.blocklist_file` | string | `/etc/kiro/blocklist.txt` | File blocklist IP |

### server_protection.ddos

| Key | Type | Default | Mô tả |
|-----|------|---------|--------|
| `ddos.per_ip_pps` | int | `3000` | Max packets/sec per IP |
| `ddos.per_subnet24_pps` | int | `30000` | Max packets/sec per /24 subnet |
| `ddos.syn_per_ip_per_second` | int | `200` | Max SYN packets/sec per IP |
| `ddos.udp_per_ip_per_second` | int | `500` | Max UDP packets/sec per IP |
| `ddos.icmp_per_ip_per_second` | int | `30` | Max ICMP packets/sec per IP |
| `ddos.temporary_block_seconds` | int | `900` | Thời gian block tạm (15 phút) |
| `ddos.greylist_seconds` | int | `300` | Thời gian greylist (5 phút) |

### website_protection.waf

| Key | Type | Default | Mô tả |
|-----|------|---------|--------|
| `waf.enabled` | bool | `true` | Bật WAF |
| `waf.engine` | string | `coraza` | Engine: `coraza`, `modsecurity` |
| `waf.owasp_crs` | bool | `true` | Dùng OWASP Core Rule Set |
| `waf.anomaly_threshold` | int | `5` | Ngưỡng anomaly score |

### website_protection.bot

| Key | Type | Default | Mô tả |
|-----|------|---------|--------|
| `bot.enabled` | bool | `true` | Bật bot detection |
| `bot.cookie_challenge` | bool | `true` | Cookie challenge |
| `bot.js_challenge` | bool | `true` | JavaScript challenge |
| `bot.proof_of_work` | bool | `false` | Proof of Work challenge |
| `bot.score_challenge` | int | `50` | Score threshold cho challenge |
| `bot.score_block` | int | `80` | Score threshold cho block |

### website_protection.admission

| Key | Type | Default | Mô tả |
|-----|------|---------|--------|
| `admission.enabled` | bool | `true` | Bật admission control |
| `admission.overload_returns` | int | `503` | HTTP code khi overload |
| `admission.default_rpm_per_ip` | int | `120` | Default requests/minute per IP |
| `admission.default_concurrent_per_ip` | int | `10` | Default concurrent requests per IP |

## TLS Modes

| Mode | Mô tả | Khi nào dùng |
|------|--------|--------------|
| `flexible_http` | Cloudflare HTTPS → Origin HTTP:80 | Website nhỏ, không cần cert |
| `full_tls` | Cloudflare HTTPS → Origin HTTPS:443 | Cần encryption end-to-end |
| `full_strict` | Cloudflare HTTPS → Origin HTTPS:443 (verified cert) | Dữ liệu nhạy cảm, compliance |

## Protection Profiles

| Profile | RPMPerIP | SubnetRPM | HardBlockAfter | Use case |
|---------|----------|-----------|----------------|----------|
| `light` | 200 | 3000 | 600 | Blog, static site |
| `balanced` | 120 | 1800 | 360 | Web app thông thường |
| `strict` | 60 | 900 | 180 | E-commerce, banking |
| `lockdown` | 30 | 450 | 90 | Đang bị tấn công |

Khi `protection.profile` được set trong YAML, các giá trị `rpm_per_ip`, `subnet_rpm`, và `hard_block_after` sẽ lấy từ bảng trên (trừ khi bị override bởi `client.*` hoặc env var).

## File Paths Summary

| Path | Purpose | Status |
|------|---------|--------|
| `/usr/local/bin/kiro-master` | Master server binary | Active |
| `/usr/local/bin/kiro-client-waf` | Client WAF binary | Active |
| `/usr/local/bin/kiro-cli` | CLI tool binary | Active |
| `/etc/kiro-master/master.env` | Master environment config | Active |
| `/etc/kiro/kiro.yaml` | Unified YAML config (kiro-client-waf + kiro-cli) | **Primary** |
| `/etc/kiro/kiro-client.env` | Client environment config | **Legacy/Deprecated** |
| `/var/lib/kiro-master/master.db` | Master SQLite database | Active |
| `/var/lib/kiro/` | Client state data | Active |
| `/var/log/kiro/` | Client logs | Active |
| `/usr/lib/kiro/xdp/xdp_filter.o` | XDP eBPF object | Active |

> **Deprecated:** `/etc/kiro/kiro-client.env` vẫn được hỗ trợ cho backward compatibility. Env vars từ file này sẽ override YAML values. Khuyến nghị migrate sang YAML và xóa file `.env` sau khi xác nhận hoạt động đúng.


## Example Configurations

### Minimal (1 domain, Cloudflare)

```yaml
mode: full
license_key: KIRO-XXXX-XXXX

website:
  enabled: true
  cloudflare: true
  tls_mode: flexible_http
  sites:
    - domains: [example.com, www.example.com]
      backend: http://127.0.0.1:3000

protection:
  profile: balanced

client:
  cookie_secret: "generated-40-char-secret"
  master_url: https://firewall.vpsgen.com
```

### Multi-domain with Routes

```yaml
mode: full
license_key: KIRO-XXXX-XXXX

admin:
  allow_ips:
    - 203.0.113.10/32

website:
  enabled: true
  cloudflare: true
  tls_mode: full_strict
  sites:
    - domains: [app.example.com]
      backend: http://127.0.0.1:3000
      routes:
        - path: /api/
          backend: http://127.0.0.1:4000
          protection: strict
        - path: /admin
          backend: http://127.0.0.1:4000
          protection: strict
    - domains: [docs.example.com]
      backend: http://127.0.0.1:8080

protection:
  profile: balanced
  waf: true
  bot: true

client:
  cookie_secret: "generated-40-char-secret"
  master_url: https://firewall.vpsgen.com
  listen_addr: ":8090"
  challenge_all_new: false
```

### Full Client Configuration

```yaml
mode: full
plan: school_smb
license_key: KIRO-XXXX-XXXX

admin:
  allow_ips:
    - 203.0.113.10/32
    - 198.51.100.5/32

website:
  enabled: true
  cloudflare: true
  tls_mode: flexible_http
  sites:
    - domains: [example.com, www.example.com]
      backend: http://127.0.0.1:3000

protection:
  profile: strict

client:
  cookie_secret: "random-40-char-secret-here-abcdefghij"
  master_url: https://firewall.vpsgen.com
  listen_addr: ":8090"
  node_id: web-server-01
  pow_difficulty: 4
  hold_seconds: 2
  rpm_per_ip: 60
  subnet_rpm: 900
  hard_block_after: 180
  block_ttl_seconds: 900
  blocklist_file: /var/lib/kiro/xdp-blocklist.txt
  xdp_sync_command: ""
  heartbeat_seconds: 60
  update_seconds: 300
  challenge_all_new: false
  transparent_ttl: 30
  cookie_short_ttl: 300
  escalation_threshold: 3
  escalation_cooldown: 600
  cookie_rate_limit: 300
  cf_trust_mode: strict
  xdp_blocked_countries: ""
  geoip_csv_path: ""
```

### Server-only Mode (No reverse proxy)

```yaml
mode: server
admin:
  allow_ips: [203.0.113.10/32]
server:
  interface: eth0
  ssh_port: 22
```
