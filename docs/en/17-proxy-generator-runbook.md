# Proxy Generator Runbook

Phase 5 generates Nginx config from runtime config. Phase 11 adds lab apply
with validation, reload, and pending rollback.

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
/tmp/kiro-proxy/kiro-waf.conf
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

## Lab Apply With Rollback

This writes active config into `--proxy-target-dir`, validates it, reloads Nginx,
and keeps pending rollback state. Run it only on a lab host with Nginx installed,
a secondary console, and a known recovery path.

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

Apply flow:

1. Generate candidate config in the state dir.
2. Run `nginx -t` against the candidate.
3. Snapshot previous active files.
4. Promote new files with atomic writes.
5. Run `nginx -t` against the target.
6. Run `nginx -s reload`.
7. Keep `pending-proxy-apply.json` until confirm.

If target validation or reload fails, the agent restores previous files and
removes pending state.

## Confirm Or Rollback

After the website is healthy:

```text
go run ./cmd/kiro-agent \
  --config configs/kiro.example.yaml \
  --proxy-confirm \
  --proxy-state-dir /var/lib/kiro
```

Manual rollback:

```text
go run ./cmd/kiro-agent \
  --config configs/kiro.example.yaml \
  --proxy-rollback \
  --proxy-lab-ack KIRO_LAB_PROXY_APPLY \
  --proxy-state-dir /var/lib/kiro
```

Rollback only when pending state has expired:

```text
go run ./cmd/kiro-agent \
  --config configs/kiro.example.yaml \
  --proxy-rollback-if-expired \
  --proxy-lab-ack KIRO_LAB_PROXY_APPLY \
  --proxy-state-dir /var/lib/kiro
```

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

## WAF/Bot In The Proxy Plan

Phase 15 wires WAF/Bot artifacts into the proxy plan:

- When `website_protection.waf.enabled: true`, `kiro-nginx.conf` includes
  `modsecurity on;` and `modsecurity_rules_file kiro-waf.conf;`.
- `kiro-waf.conf` contains Coraza/ModSecurity-compatible SecLang, OWASP CRS
  includes, and route-scoped false-positive allowlist notes.
- When `website_protection.bot.cookie_challenge: true`, the Nginx plan includes
  `location = /kiro-challenge` to set `kiro_challenge=ok` and cookie guards in
  proxied locations.
- Lab apply snapshots and rolls back `kiro-waf.conf` with the other proxy files.

## Limits

- Apply is guarded by a lab acknowledgement; this is not the production
  installer yet.
- Reload uses `nginx -s reload` with the validated config path. Hosts using a
  different systemd/Nginx layout need lab verification before production.
- Production hosts need a compatible Nginx ModSecurity/Coraza module before
  validating or reloading configs with WAF directives.
