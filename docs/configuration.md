# Configuration

## Overview

Kiro WAF sử dụng hai loại cấu hình:

1. **Environment variables** — Cấu hình runtime cho binaries (`kiro-master`, `kiro-client-waf`)
2. **YAML config files** — Cấu hình chi tiết cho kiro-cli và advanced features

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

## Client WAF Environment Variables

File: `/etc/kiro/client-waf.env` (hoặc `/etc/kiro/kiro-client.env`)

### Required Variables

| Variable | Default | Mô tả |
|----------|---------|--------|
| `KIRO_LICENSE_KEY` | — | License key (fatal if empty) |
| `KIRO_CLIENT_COOKIE_SECRET` | — | HMAC cookie secret (fatal if empty) |
| `KIRO_BACKEND_URL` | — | Backend URL to proxy to (fatal if empty) |
| `KIRO_MASTER_URL` | — | Master server URL for heartbeat/updates (fatal if empty) |

### Optional Variables

| Variable | Default | Mô tả |
|----------|---------|--------|
| `KIRO_CLIENT_LISTEN` | `:8090` | Listen address for WAF proxy |
| `KIRO_NODE_ID` | hostname | Node identifier for heartbeat |
| `KIRO_POW_DIFFICULTY` | `4` | Proof-of-Work difficulty (number of leading zeros) |
| `KIRO_HOLD_SECONDS` | `2` | Hold page duration in seconds |
| `KIRO_RPM_PER_IP` | `120` | Requests per minute per IP (soft threshold) |
| `KIRO_SUBNET_RPM` | `1800` | Requests per minute per /24 subnet |
| `KIRO_HARD_BLOCK_AFTER` | `360` | RPM threshold for hard block |
| `KIRO_BLOCK_TTL_SECONDS` | `900` | Ban duration in seconds (15 min) |
| `KIRO_XDP_BLOCKLIST_FILE` | `/var/lib/kiro/xdp-blocklist.txt` | XDP blocklist file path |
| `KIRO_XDP_SYNC_COMMAND` | `""` | Command to sync XDP blocklist |
| `KIRO_HEARTBEAT_SECONDS` | `60` | Heartbeat interval to master |
| `KIRO_UPDATE_SECONDS` | `300` | Update check interval (5 min) |
| `KIRO_ADMIN_IPS` | `""` | Comma-separated admin IPs (bypass lockdown) |

**Example:**
```bash
KIRO_CLIENT_LISTEN=:8090
KIRO_BACKEND_URL=http://127.0.0.1:3000
KIRO_MASTER_URL=https://firewall.vpsgen.com
KIRO_LICENSE_KEY=KIRO-XXXX-XXXX
KIRO_CLIENT_COOKIE_SECRET=random-64-char-secret
KIRO_NODE_ID=web-server-01
KIRO_RPM_PER_IP=120
KIRO_SUBNET_RPM=1800
KIRO_HARD_BLOCK_AFTER=360
KIRO_BLOCK_TTL_SECONDS=900
KIRO_POW_DIFFICULTY=4
KIRO_HOLD_SECONDS=2
KIRO_HEARTBEAT_SECONDS=60
KIRO_UPDATE_SECONDS=300
KIRO_XDP_BLOCKLIST_FILE=/var/lib/kiro/xdp-blocklist.txt
KIRO_ADMIN_IPS=203.0.113.10,198.51.100.5
```

## YAML Configuration File

File: `/etc/kiro/kiro.yaml`

Kiro hỗ trợ 2 mức cấu hình YAML:
- **Simple** (`kiro.example.yaml`): Đủ cho 80-90% use cases
- **Advanced** (`kiro.advanced.example.yaml`): Full control cho enterprise

### Top-level Options

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

| Profile | WAF | Bot | Rate Limit | Use case |
|---------|-----|-----|-----------|----------|
| `light` | Basic rules | Cookie only | 200 RPM | Blog, static site |
| `balanced` | OWASP CRS | Cookie + JS | 120 RPM | Web app thông thường |
| `strict` | Full CRS + custom | All challenges | 60 RPM | E-commerce, banking |
| `lockdown` | Maximum | PoW required | 30 RPM | Đang bị tấn công |

## File Paths Summary

| Path | Purpose |
|------|---------|
| `/usr/local/bin/kiro-master` | Master server binary |
| `/usr/local/bin/kiro-client-waf` | Client WAF binary |
| `/usr/local/bin/kiro-cli` | CLI tool binary |
| `/etc/kiro-master/master.env` | Master environment config |
| `/etc/kiro/client-waf.env` | Client environment config |
| `/etc/kiro/kiro.yaml` | YAML config (for kiro-cli) |
| `/var/lib/kiro-master/master.db` | Master SQLite database |
| `/var/lib/kiro/` | Client state data |
| `/var/log/kiro/` | Client logs |
| `/usr/lib/kiro/xdp/xdp_filter.o` | XDP eBPF object |

## Example Configurations

### Minimal (1 domain, Cloudflare)

```yaml
mode: full
website:
  enabled: true
  cloudflare: true
  tls_mode: flexible_http
  sites:
    - domains: [example.com, www.example.com]
      backend: http://127.0.0.1:3000
protection:
  profile: balanced
```

### Multi-domain with Routes

```yaml
mode: full
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
