# Pilot Go/No-Go Runbook

Phase 18 gathers evidence from earlier phases into a release-candidate decision
report. The report does not prove production readiness by itself; it makes
completed evidence and hold blockers explicit.

## Create A Report

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

## Decision Rule

`decision: go` requires:

- 3-5 VPS/servers in the pilot.
- At least 30 days of duration.
- Evidence for health, benchmark, benchmark evidence, incident drill,
  update/rollback, revocation sync, and proxy/WAF/Bot.

If evidence is missing or a check fails, the report returns `decision: hold` and
lists `blockers`.
