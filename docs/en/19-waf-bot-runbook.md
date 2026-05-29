# WAF And Bot Defense Runbook

Phase 7 adds lab HTTP defense evaluation. Commands in this phase only dry-run or
evaluate. They do not reload the proxy or change firewall state.

## Generate WAF SecLang Dry-run

```text
go run ./cmd/kiro-agent \
  --config configs/tenant.full-cloudflare.example.yaml \
  --waf-dry-run
```

The output is SecLang config compatible with Coraza/ModSecurity and OWASP CRS
includes. The lab evaluator mirrors common CRS categories:

- `KIRO-CRS-930-TRAVERSAL`
- `KIRO-CRS-941-XSS`
- `KIRO-CRS-942-SQLI`

## Evaluate A Request Through WAF + Bot

SQL injection should be blocked:

```text
go run ./cmd/kiro-agent \
  --config configs/tenant.full-cloudflare.example.yaml \
  --web-defense-evaluate \
  --web-path /login \
  --web-query "user=admin' OR 1=1--"
```

A browser without the challenge cookie should be challenged:

```text
go run ./cmd/kiro-agent \
  --config configs/tenant.full-cloudflare.example.yaml \
  --web-defense-evaluate \
  --web-client-ip 198.51.100.20 \
  --web-user-agent "Mozilla/5.0 Chrome/120" \
  --web-path /
```

Admin allowlist clients should not be challenged:

```text
go run ./cmd/kiro-agent \
  --config configs/tenant.full-cloudflare.example.yaml \
  --web-defense-evaluate \
  --web-client-ip 203.0.113.10 \
  --web-user-agent "" \
  --web-path /
```

## False Positive Exception In Lab

Exclude a rule for the evaluated path:

```text
go run ./cmd/kiro-agent \
  --config configs/tenant.full-cloudflare.example.yaml \
  --web-defense-evaluate \
  --web-path /search \
  --web-query "q=union select" \
  --waf-exclude-rules KIRO-CRS-942-SQLI
```

## Production Proxy Plan

Phase 15 emits WAF/Bot artifacts in the Nginx plan:

```text
go run ./cmd/kiro-agent \
  --config configs/kiro.advanced.example.yaml \
  --proxy-dry-run \
  --proxy-output-dir /tmp/kiro-proxy
```

Check:

```text
rg 'modsecurity_rules_file kiro-waf.conf' /tmp/kiro-proxy/kiro-nginx.conf
rg 'location = /kiro-challenge' /tmp/kiro-proxy/kiro-nginx.conf
rg 'SecRuleEngine On' /tmp/kiro-proxy/kiro-waf.conf
```

Route `waf_exclude_rules` are written to `kiro-waf.conf`. Numeric rule IDs emit
`ctl:ruleRemoveById`; lab IDs such as `KIRO-CRS-*` are kept as comments so the
operator can map them to real CRS IDs.

## Phase 7 Limits

- Phase 15 emits proxy artifacts for the real request path, but the host must
  still have a compatible Coraza/ModSecurity module and OWASP CRS paths.
- Request body, cookies, and tokens are not logged to events by default.
