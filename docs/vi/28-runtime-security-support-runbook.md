# Runbook Runtime Security Và Support Bundle

Phase 9 thêm kiểm tra runtime security và support bundle cục bộ. Phase 13 bổ
sung input JSONL chuẩn hóa từ auditd/eBPF collector và export provider inbox
dạng file. Các lệnh dưới đây chỉ scan/evaluate và ghi file trong lab, không gọi
API provider thật.

## Process Alert Lab

Web user chạy shell phải tạo alert:

```text
go run ./cmd/kiro-agent --config configs/kiro.advanced.example.yaml \
  --runtime-security-scan \
  --runtime-process-user www-data \
  --runtime-process-command bash \
  --runtime-alert-file /tmp/kiro-runtime-alerts.jsonl
```

Các process trong `runtime_security.alert_when_web_user_executes` gồm `sh`,
`bash`, `curl`, `wget`, `nc`, `python`, `perl`.

## Process Events JSONL

Collector auditd/eBPF bên ngoài có thể ghi event đã chuẩn hóa:

```json
{"at":"2026-05-28T00:00:00Z","user":"www-data","command":"/bin/bash","pid":123}
```

Scan file event:

```text
go run ./cmd/kiro-agent --config configs/kiro.advanced.example.yaml \
  --runtime-security-scan \
  --runtime-process-events-file /tmp/kiro-process-events.jsonl \
  --runtime-alert-file /tmp/kiro-runtime-alerts.jsonl
```

## File Integrity Baseline

Tạo baseline:

```text
go run ./cmd/kiro-agent --config configs/kiro.advanced.example.yaml \
  --runtime-security-scan \
  --runtime-integrity-init \
  --runtime-paths /tmp/kiro-integrity-root \
  --runtime-baseline-file /tmp/kiro-integrity-baseline.json
```

Scan sau khi sửa file:

```text
go run ./cmd/kiro-agent --config configs/kiro.advanced.example.yaml \
  --runtime-security-scan \
  --runtime-paths /tmp/kiro-integrity-root \
  --runtime-baseline-file /tmp/kiro-integrity-baseline.json \
  --runtime-alert-file /tmp/kiro-runtime-alerts.jsonl
```

File mới, file bị sửa và file bị xóa sẽ tạo alert.

## Support Bundle

```text
go run ./cmd/kiro-agent --config configs/kiro.advanced.example.yaml \
  --support-bundle \
  --support-output-dir /tmp/kiro-support-bundle \
  --support-alert-file /tmp/kiro-runtime-alerts.jsonl
```

Bundle gồm:

- `summary.json`
- `config-redacted.yaml`
- `runtime-alerts.jsonl` nếu có
- `health-report.json` nếu truyền `--support-health-file`

Các key/value nhạy cảm như password, token, secret, cookie, authorization,
license key được redact trước khi ghi bundle.

## Provider Inbox Export

Export health, alerts, support bundle và incident report vào thư mục inbox local
cho provider sync thủ công:

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

Kết quả gồm `provider-inbox-index.json` và các file đã redact. Đây là file
handoff, chưa upload API.

## Giới Hạn Phase 9

- Chưa tail auditd/eBPF trực tiếp trong agent daemon; Phase 13 đọc JSONL đã
  chuẩn hóa từ collector ngoài.
- Chưa upload provider API; Phase 13 export provider inbox local.
- File integrity scan đọc file regular trong path được cấu hình, bỏ qua path
  không tồn tại để dễ chạy lab.
