# Proxy Generator Runbook

Phase 5 generates Nginx config from runtime config. It does not reload Nginx yet.

## Dry-run

```text
go run ./cmd/kiro-agent --config configs/kiro.example.yaml --proxy-dry-run
```

Write generated files:

```text
go run ./cmd/kiro-agent \
  --config configs/kiro.example.yaml \
  --proxy-dry-run \
  --proxy-output-dir /tmp/kiro-proxy
```

Generated files:

```text
/tmp/kiro-proxy/kiro-nginx.conf
/tmp/kiro-proxy/cloudflare-real-ip.conf
```

## Validate with Nginx

On a lab host with `nginx` installed:

```text
go run ./cmd/kiro-agent \
  --config configs/kiro.example.yaml \
  --proxy-dry-run \
  --proxy-output-dir /tmp/kiro-proxy \
  --proxy-validate
```

`--proxy-validate` runs `nginx -t` only. It does not reload Nginx.

## Server Mode

In `mode: server`, website proxy is disabled. The generator prints
`proxy disabled in server mode` and does not emit website server blocks.

## TLS

- `flexible_http`: emits `listen 80` and does not require cert/key files.
- `full_strict`: emits `listen 443 ssl` and requires cert/key in config.

## Cloudflare Real IP

When Cloudflare is enabled, the generated config includes:

```text
include cloudflare-real-ip.conf;
real_ip_header CF-Connecting-IP;
```

The real IP file is generated from:

```text
rules/cloudflare/ips-v4.txt
rules/cloudflare/ips-v6.txt
```
