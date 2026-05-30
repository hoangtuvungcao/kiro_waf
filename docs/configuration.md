# Configuration

## Configuration File

File cấu hình chính: `/etc/kiro/kiro.yaml`

Kiro hỗ trợ 2 mức cấu hình:
- **Simple** (`kiro.example.yaml`): Đủ cho 80-90% use cases
- **Advanced** (`kiro.advanced.example.yaml`): Full control cho enterprise

## Simple Configuration Reference

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

### license

| Key | Type | Default | Mô tả |
|-----|------|---------|--------|
| `license.file` | string | `/etc/kiro/license.json` | File license |
| `license.provider_public_key` | string | `/etc/kiro/provider-public-key.pem` | Public key verify |
| `license.require_valid_license` | bool | `true` | Yêu cầu license hợp lệ |
| `license.allow_grace_period` | bool | `true` | Cho phép grace period |

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

### resource_governor

| Key | Type | Default | Mô tả |
|-----|------|---------|--------|
| `resource_governor.enabled` | bool | `true` | Bật resource governor |
| `resource_governor.baseline.learning_days` | int | `7` | Số ngày học baseline |
| `resource_governor.hysteresis.cooldown_seconds` | int | `600` | Cooldown trước khi hạ level |

### logging

| Key | Type | Default | Mô tả |
|-----|------|---------|--------|
| `logging.level` | string | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `logging.directory` | string | `/var/log/kiro` | Thư mục log |
| `logging.rate_limit_per_second` | int | `100` | Max log entries/sec |

## Environment Variables

| Variable | Mô tả |
|----------|--------|
| `KIRO_CONFIG` | Path đến config file (override default) |
| `KIRO_LOG_LEVEL` | Override log level |
| `KIRO_DRY_RUN` | Force dry-run mode (`true`/`false`) |

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
