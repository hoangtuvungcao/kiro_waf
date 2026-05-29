# Runtime Security And Support Bundle Runbook

Phase 9 adds local runtime security checks and support bundles. Phase 13 adds
normalized auditd/eBPF collector JSONL input and local provider inbox export.
The commands below scan/evaluate and write local lab files only. They do not
call a real provider API.

## Process Alert Lab

A web user executing a shell should create an alert:

```text
go run ./cmd/kiro-agent --config configs/kiro.advanced.example.yaml \
  --runtime-security-scan \
  --runtime-process-user www-data \
  --runtime-process-command bash \
  --runtime-alert-file /tmp/kiro-runtime-alerts.jsonl
```

Processes listed in `runtime_security.alert_when_web_user_executes` include
`sh`, `bash`, `curl`, `wget`, `nc`, `python`, and `perl`.

## Process Events JSONL

An external auditd/eBPF collector can write normalized events:

```json
{"at":"2026-05-28T00:00:00Z","user":"www-data","command":"/bin/bash","pid":123}
```

Scan the event file:

```text
go run ./cmd/kiro-agent --config configs/kiro.advanced.example.yaml \
  --runtime-security-scan \
  --runtime-process-events-file /tmp/kiro-process-events.jsonl \
  --runtime-alert-file /tmp/kiro-runtime-alerts.jsonl
```

## File Integrity Baseline

Create a baseline:

```text
go run ./cmd/kiro-agent --config configs/kiro.advanced.example.yaml \
  --runtime-security-scan \
  --runtime-integrity-init \
  --runtime-paths /tmp/kiro-integrity-root \
  --runtime-baseline-file /tmp/kiro-integrity-baseline.json
```

Scan after modifying files:

```text
go run ./cmd/kiro-agent --config configs/kiro.advanced.example.yaml \
  --runtime-security-scan \
  --runtime-paths /tmp/kiro-integrity-root \
  --runtime-baseline-file /tmp/kiro-integrity-baseline.json \
  --runtime-alert-file /tmp/kiro-runtime-alerts.jsonl
```

Added, modified, and deleted files generate alerts.

## Support Bundle

```text
go run ./cmd/kiro-agent --config configs/kiro.advanced.example.yaml \
  --support-bundle \
  --support-output-dir /tmp/kiro-support-bundle \
  --support-alert-file /tmp/kiro-runtime-alerts.jsonl
```

The bundle includes:

- `summary.json`
- `config-redacted.yaml`
- `runtime-alerts.jsonl` if provided
- `health-report.json` if `--support-health-file` is provided

Sensitive keys/values such as password, token, secret, cookie, authorization,
and license keys are redacted before writing the bundle.

## Provider Inbox Export

Export health, alerts, support bundle, and incident report files into a local
provider inbox for manual sync:

```text
go run ./cmd/kiro-agent --config configs/kiro.advanced.example.yaml \
  --support-provider-export \
  --support-provider-inbox-dir /tmp/kiro-provider-inbox \
  --support-provider-server-id srv_001 \
  --support-provider-bundle-dir /tmp/kiro-support-bundle \
  --support-provider-incident-dir /tmp/kiro-incident/reports/inc_001 \
  --support-health-file /tmp/kiro-health.json \
  --support-alert-file /tmp/kiro-runtime-alerts.jsonl
```

The result includes `provider-inbox-index.json` and redacted files. This is a
file handoff, not an API upload.

## Phase 9 Limits

- The agent daemon does not tail auditd/eBPF directly yet; Phase 13 reads
  normalized JSONL from an external collector.
- Provider API upload is not implemented yet; Phase 13 exports a local provider
  inbox.
- File integrity scanning reads regular files under configured paths and skips
  missing paths for lab use.
