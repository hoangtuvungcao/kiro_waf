# Runbook Proxy Generator

Phase 5 sinh cấu hình Nginx từ runtime config. Phase này chưa reload Nginx thật.

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

## Cảnh báo

Nếu route nhạy cảm như `/login`, `/admin`, `/api/export` dùng `flexible_http`,
agent sẽ in warning. Production nhạy cảm nên dùng `full_strict`.
