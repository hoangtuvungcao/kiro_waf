# Runbook Proxy Generator

Phase 5 sinh cấu hình Nginx từ runtime config. Phase 11 bổ sung luồng apply
lab có validate, reload và pending rollback.

## Dry-run

```text
go run ./cmd/kiro-agent --config configs/kiro.example.yaml --proxy-dry-run
```

Ghi file ra thư mục tạm:

```text
go run ./cmd/kiro-agent \
  --config configs/kiro.example.yaml \
  --proxy-dry-run \
  --proxy-output-dir /tmp/kiro-proxy
```

File sinh ra:

```text
/tmp/kiro-proxy/kiro-nginx.conf
/tmp/kiro-proxy/cloudflare-real-ip.conf
/tmp/kiro-proxy/kiro-waf.conf
```

## Validate bằng Nginx

Nếu máy lab có `nginx`:

```text
go run ./cmd/kiro-agent \
  --config configs/kiro.example.yaml \
  --proxy-dry-run \
  --proxy-output-dir /tmp/kiro-proxy \
  --proxy-validate
```

`--proxy-validate` chỉ chạy `nginx -t`, chưa reload service.

## Apply Lab Có Rollback

Lệnh này ghi config active vào `--proxy-target-dir`, chạy validate, reload
Nginx và giữ pending rollback. Chỉ chạy trên máy lab có Nginx, có console phụ
và đã biết cách khôi phục dịch vụ.

```text
go run ./cmd/kiro-agent \
  --config configs/kiro.example.yaml \
  --proxy-apply-lab \
  --proxy-lab-ack KIRO_LAB_PROXY_APPLY \
  --proxy-target-dir /etc/nginx/kiro \
  --proxy-state-dir /var/lib/kiro \
  --proxy-snapshot-dir /var/lib/kiro/last-good-config/proxy \
  --proxy-rollback-seconds 60
```

Luồng apply:

1. Sinh candidate config vào state dir.
2. Chạy `nginx -t` trên candidate.
3. Snapshot file active cũ.
4. Promote file mới bằng ghi atomic.
5. Chạy `nginx -t` trên target.
6. Chạy `nginx -s reload`.
7. Giữ `pending-proxy-apply.json` tới khi confirm.

Nếu validate target hoặc reload lỗi, agent tự restore file cũ và xóa pending
state.

## Confirm Hoặc Rollback

Sau khi kiểm tra website ổn:

```text
go run ./cmd/kiro-agent \
  --config configs/kiro.example.yaml \
  --proxy-confirm \
  --proxy-state-dir /var/lib/kiro
```

Rollback thủ công khi có lỗi:

```text
go run ./cmd/kiro-agent \
  --config configs/kiro.example.yaml \
  --proxy-rollback \
  --proxy-lab-ack KIRO_LAB_PROXY_APPLY \
  --proxy-state-dir /var/lib/kiro
```

Rollback nếu pending đã quá hạn:

```text
go run ./cmd/kiro-agent \
  --config configs/kiro.example.yaml \
  --proxy-rollback-if-expired \
  --proxy-lab-ack KIRO_LAB_PROXY_APPLY \
  --proxy-state-dir /var/lib/kiro
```

## Mode server

Ở `mode: server`, proxy website không hoạt động. Generator chỉ in thông báo
`proxy disabled in server mode` và không sinh server block website.

## TLS

- `flexible_http`: sinh `listen 80`, không yêu cầu cert/key.
- `full_strict`: sinh `listen 443 ssl`, bắt buộc có cert/key trong config.

## Cloudflare real IP

Khi bật Cloudflare, generator sinh include:

```text
include cloudflare-real-ip.conf;
real_ip_header CF-Connecting-IP;
```

File real IP được sinh từ:

```text
rules/cloudflare/ips-v4.txt
rules/cloudflare/ips-v6.txt
```

## WAF/Bot Trong Proxy Plan

Phase 15 nối cấu hình WAF/Bot vào proxy plan:

- Khi `website_protection.waf.enabled: true`, `kiro-nginx.conf` có
  `modsecurity on;` và `modsecurity_rules_file kiro-waf.conf;`.
- `kiro-waf.conf` chứa SecLang tương thích Coraza/ModSecurity, include OWASP
  CRS và ghi chú allowlist false positive theo route.
- Khi `website_protection.bot.cookie_challenge: true`, Nginx plan có endpoint
  `location = /kiro-challenge` để set cookie `kiro_challenge=ok` và guard cookie
  trong các location proxy.
- Apply lab snapshot/rollback cả `kiro-waf.conf`.

## Cảnh báo

Nếu route nhạy cảm như `/login`, `/admin`, `/api/export` dùng `flexible_http`,
agent sẽ in warning. Production nhạy cảm nên dùng `full_strict`.

## Giới Hạn

- Apply được guard bằng ACK lab, chưa phải installer production.
- Reload dùng `nginx -s reload` với config path được validate. Môi trường dùng
  systemd/nginx layout khác cần test lab trước khi production.
- Host production phải có Nginx module ModSecurity/Coraza tương thích trước khi
  bật validate/reload với WAF directive.
