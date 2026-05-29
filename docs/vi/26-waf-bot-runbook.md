# Runbook WAF Và Bot Defense

Phase 7 thêm lớp kiểm tra HTTP trong lab. Lệnh ở phase này chỉ dry-run/evaluate,
chưa reload proxy và chưa thay đổi firewall.

## Sinh WAF SecLang Dry-run

```text
go run ./cmd/kiro-agent \
  --config configs/tenant.full-cloudflare.example.yaml \
  --waf-dry-run
```

Output là cấu hình SecLang tương thích Coraza/ModSecurity và include OWASP CRS.
Lab evaluator hiện mirror các nhóm rule CRS phổ biến:

- `KIRO-CRS-930-TRAVERSAL`
- `KIRO-CRS-941-XSS`
- `KIRO-CRS-942-SQLI`

## Evaluate Request Qua WAF + Bot

SQLi phải bị block:

```text
go run ./cmd/kiro-agent \
  --config configs/tenant.full-cloudflare.example.yaml \
  --web-defense-evaluate \
  --web-path /login \
  --web-query "user=admin' OR 1=1--"
```

Browser chưa có challenge cookie phải bị challenge:

```text
go run ./cmd/kiro-agent \
  --config configs/tenant.full-cloudflare.example.yaml \
  --web-defense-evaluate \
  --web-client-ip 198.51.100.20 \
  --web-user-agent "Mozilla/5.0 Chrome/120" \
  --web-path /
```

Admin allowlist không bị challenge:

```text
go run ./cmd/kiro-agent \
  --config configs/tenant.full-cloudflare.example.yaml \
  --web-defense-evaluate \
  --web-client-ip 203.0.113.10 \
  --web-user-agent "" \
  --web-path /
```

## False Positive Exception Trong Lab

Có thể exclude rule theo path khi evaluate:

```text
go run ./cmd/kiro-agent \
  --config configs/tenant.full-cloudflare.example.yaml \
  --web-defense-evaluate \
  --web-path /search \
  --web-query "q=union select" \
  --waf-exclude-rules KIRO-CRS-942-SQLI
```

## Proxy Production Plan

Phase 15 đưa WAF/Bot vào Nginx plan:

```text
go run ./cmd/kiro-agent \
  --config configs/kiro.advanced.example.yaml \
  --proxy-dry-run \
  --proxy-output-dir /tmp/kiro-proxy
```

Kiểm tra:

```text
rg 'modsecurity_rules_file kiro-waf.conf' /tmp/kiro-proxy/kiro-nginx.conf
rg 'location = /kiro-challenge' /tmp/kiro-proxy/kiro-nginx.conf
rg 'SecRuleEngine On' /tmp/kiro-proxy/kiro-waf.conf
```

False positive allowlist trong route `waf_exclude_rules` được ghi vào
`kiro-waf.conf`. Rule ID dạng số sinh `ctl:ruleRemoveById`; rule ID lab dạng
`KIRO-CRS-*` được giữ bằng comment để operator map sang CRS rule thật.

## Giới hạn Phase 7

- Phase 15 đã sinh proxy artifacts cho request path thật, nhưng host vẫn phải có
  module Coraza/ModSecurity và OWASP CRS đã cài đúng path.
- Không log request body/cookie/token vào event mặc định.
