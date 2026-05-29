# Runbook Resource Governor

Phase 6 đánh giá tải hệ thống và quyết định mức phòng thủ runtime. Phase 13 bổ
sung action plan/overlay lab để các phase proxy/firewall/WAF có thể dùng chung
một quyết định đã ghi file. Mặc định lệnh vẫn chỉ evaluate, không apply hệ thống
thật.

## Evaluate bằng sample giả lập

```text
go run ./cmd/kiro-agent \
  --config configs/tenant.server-only.example.yaml \
  --governor-evaluate \
  --sample-cpu-percent 86 \
  --sample-ram-available-percent 50
```

Output là JSON gồm:

- `candidate_level`: mức tính từ sample hiện tại.
- `level`: mức sau hysteresis/cooldown.
- `reasons`: metric nào chạm ngưỡng.
- `actions`: hành động agent/proxy/firewall nên áp dụng ở phase sau.

## Kiểm tra conntrack lockdown

```text
go run ./cmd/kiro-agent \
  --config configs/kiro.advanced.example.yaml \
  --governor-evaluate \
  --sample-conntrack-percent 91 \
  --sample-ram-available-percent 50
```

Với ngưỡng mặc định, sample này lên `LOCKDOWN`.

## Ghi state và event

```text
go run ./cmd/kiro-agent \
  --config configs/tenant.server-only.example.yaml \
  --governor-evaluate \
  --governor-state-file /tmp/kiro-governor-state.json \
  --governor-event-file /tmp/kiro-governor-events.jsonl \
  --sample-cpu-percent 90 \
  --sample-ram-available-percent 50
```

State dùng để giữ hysteresis/cooldown giữa các lần evaluate. Event JSONL dùng
cho incident/support bundle ở các phase sau.

## Action Plan Và Overlay Lab

Tạo action plan JSON mà không ghi overlay:

```text
go run ./cmd/kiro-agent \
  --config configs/kiro.advanced.example.yaml \
  --governor-evaluate \
  --sample-cpu-percent 96 \
  --sample-conntrack-percent 91 \
  --sample-ram-available-percent 7 \
  --governor-action-plan-file /tmp/kiro-governor-action-plan.json
```

Ghi overlay lab:

```text
go run ./cmd/kiro-agent \
  --config configs/kiro.advanced.example.yaml \
  --governor-evaluate \
  --sample-cpu-percent 96 \
  --sample-conntrack-percent 91 \
  --sample-ram-available-percent 7 \
  --governor-action-plan-file /tmp/kiro-governor-action-plan.json \
  --governor-action-output-dir /tmp/kiro-governor-actions \
  --governor-action-apply-lab
```

File sinh ra:

```text
/tmp/kiro-governor-action-plan.json
/tmp/kiro-governor-actions/governor-action-plan.json
/tmp/kiro-governor-actions/governor-response-overlay.json
```

Overlay mô tả các thay đổi tạm thời như giảm rate limit, bật challenge client
mới, tăng cache, tắt route tốn tài nguyên hoặc lockdown chỉ admin/known client.
Đây là file lab, chưa tự reload proxy/firewall.

## Nguyên tắc an toàn

- Escalate lên mức cao hơn ngay khi sample chạm ngưỡng.
- Hạ mức chỉ khi đã qua `min_level_hold_seconds`, `cooldown_seconds` và đủ
  `require_recovery_samples`.
- Action overlay Phase 13 chỉ ghi file lab. Apply sang firewall/proxy/WAF thật
  vẫn phải đi qua phase riêng có validate/rollback.
