# Event Log Rate Limit And Rotation Runbook

This completes part of the “agent stability” production gate: event logs have
rate limiting and JSONL files rotate to avoid unbounded disk growth.

The current commands apply to:

- Governor decisions through `--governor-event-file`.
- Runtime security alerts through `--runtime-alert-file`.

## Runtime Alert Log

```text
rm -f /tmp/kiro-runtime-alerts.jsonl /tmp/kiro-runtime-alerts.jsonl.*
go run ./cmd/kiro-agent \
  --config configs/kiro.advanced.example.yaml \
  --runtime-security-scan \
  --runtime-process-user www-data \
  --runtime-process-command bash \
  --runtime-alert-file /tmp/kiro-runtime-alerts.jsonl \
  --event-log-max-bytes 1024 \
  --event-log-max-backups 2 \
  --event-rate-limit-window-seconds 60 \
  --event-rate-limit-max 1
```

Running the same command again within 60 seconds still returns the alert on
stdout, but it does not append another JSONL line for the same rate-limit key.

Checks:

```text
wc -l /tmp/kiro-runtime-alerts.jsonl
test -f /tmp/kiro-runtime-alerts.jsonl.ratelimit.json
```

## Governor Event Log

```text
rm -f /tmp/kiro-governor-events.jsonl /tmp/kiro-governor-events.jsonl.*
go run ./cmd/kiro-agent \
  --config configs/tenant.server-only.example.yaml \
  --governor-evaluate \
  --sample-cpu-percent 86 \
  --sample-ram-available-percent 50 \
  --governor-event-file /tmp/kiro-governor-events.jsonl \
  --event-log-max-bytes 1024 \
  --event-log-max-backups 2 \
  --event-rate-limit-window-seconds 60 \
  --event-rate-limit-max 1
```

## Rotation

When the current file plus the next event exceeds `--event-log-max-bytes`, the
file is rotated:

```text
/tmp/kiro-runtime-alerts.jsonl
/tmp/kiro-runtime-alerts.jsonl.1
/tmp/kiro-runtime-alerts.jsonl.2
```

`--event-log-max-backups 0` removes the old file instead of keeping a backup. Do
not use that in production unless an external log collector is already in place.

## Limits

- The rate limit is local file state, not a distributed quota.
- Rotation is best-effort on append and does not replace logrotate/systemd
  journal policy.
- Suppressed events may still appear on lab command stdout; suppression here
  only prevents repeated JSONL writes.
