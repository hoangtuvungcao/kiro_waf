# Runbook Event Log Rate Limit Và Rotation

Runbook này hoàn thiện một phần production gate “agent ổn định”: event log có
rate limit và file JSONL được rotate để tránh phình vô hạn.

Các lệnh hiện áp dụng cho:

- Governor decision event qua `--governor-event-file`.
- Runtime security alert qua `--runtime-alert-file`.

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

Chạy lại cùng lệnh trong 60 giây sẽ trả alert trong stdout nhưng không ghi thêm
dòng JSONL cho cùng rate-limit key.

Kiểm tra:

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

Khi file hiện tại cộng thêm event mới vượt `--event-log-max-bytes`, file được
rotate:

```text
/tmp/kiro-runtime-alerts.jsonl
/tmp/kiro-runtime-alerts.jsonl.1
/tmp/kiro-runtime-alerts.jsonl.2
```

`--event-log-max-backups 0` sẽ xóa file cũ thay vì giữ backup. Không nên dùng
trên production nếu chưa có log collector bên ngoài.

## Giới Hạn

- Rate limit là local file state, chưa phải distributed quota.
- Rotation là best-effort theo lần append, không thay thế logrotate/systemd
  journal policy.
- Event bị suppress vẫn có thể xuất hiện trong stdout của lệnh lab; suppress ở
  đây chỉ chặn ghi lặp vào JSONL.
