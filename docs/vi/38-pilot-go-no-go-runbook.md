# Runbook Pilot Go/No-Go

Phase 18 gom evidence từ các phase trước để tạo report quyết định release
candidate. Report không tự chứng minh production readiness; nó chỉ nêu rõ phần
nào đã có evidence và phần nào còn hold.

## Tạo Report

```text
go run ./cmd/kiro-cli pilot report \
  --config configs/kiro.advanced.example.yaml \
  --output-dir /tmp/kiro-pilot \
  --pilot-id pilot_001 \
  --server-count 3 \
  --started-at 2026-04-28T00:00:00Z \
  --ended-at 2026-05-28T00:00:00Z \
  --health-file /tmp/health.json \
  --benchmark-file /tmp/kiro-benchmark.json \
  --benchmark-evidence-file /tmp/kiro-benchmark-evidence.json \
  --incident-dir /tmp/incidents/inc_001 \
  --update-evidence-file /tmp/update-apply.txt \
  --revocation-file /tmp/revocations.json \
  --proxy-evidence-file /tmp/kiro-proxy/kiro-nginx.conf
```

Output:

```text
/tmp/kiro-pilot/pilot_001/pilot-go-no-go.json
/tmp/kiro-pilot/pilot_001/pilot-go-no-go.md
```

## Rule Quyết Định

`decision: go` chỉ khi:

- Pilot có 3-5 VPS/server.
- Duration ít nhất 30 ngày.
- Có evidence cho health, benchmark, benchmark evidence, incident drill,
  update/rollback, revocation sync và proxy/WAF/Bot.

Nếu thiếu evidence hoặc check fail, report trả `decision: hold` và liệt kê
`blockers`.
