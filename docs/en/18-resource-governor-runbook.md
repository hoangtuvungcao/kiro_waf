# Resource Governor Runbook

Phase 6 evaluates host load and chooses the runtime defense level. Phase 13 adds
lab action plans/overlays so proxy/firewall/WAF phases can consume one recorded
decision. By default commands still only evaluate and do not apply real system
changes.

## Evaluate a Synthetic Sample

```text
go run ./cmd/kiro-agent \
  --config configs/tenant.server-only.example.yaml \
  --governor-evaluate \
  --sample-cpu-percent 86 \
  --sample-ram-available-percent 50
```

The JSON output includes:

- `candidate_level`: level computed from the current sample.
- `level`: level after hysteresis/cooldown.
- `reasons`: metric thresholds that fired.
- `actions`: actions later phases can apply through firewall/proxy/WAF managers.

## Check Conntrack Lockdown

```text
go run ./cmd/kiro-agent \
  --config configs/kiro.advanced.example.yaml \
  --governor-evaluate \
  --sample-conntrack-percent 91 \
  --sample-ram-available-percent 50
```

With default thresholds this sample reaches `LOCKDOWN`.

## Write State and Events

```text
go run ./cmd/kiro-agent \
  --config configs/tenant.server-only.example.yaml \
  --governor-evaluate \
  --governor-state-file /tmp/kiro-governor-state.json \
  --governor-event-file /tmp/kiro-governor-events.jsonl \
  --sample-cpu-percent 90 \
  --sample-ram-available-percent 50
```

State preserves hysteresis/cooldown across evaluations. JSONL events are for
future incident reports and support bundles.

## Action Plan And Lab Overlay

Write an action plan JSON without an overlay:

```text
go run ./cmd/kiro-agent \
  --config configs/kiro.advanced.example.yaml \
  --governor-evaluate \
  --sample-cpu-percent 96 \
  --sample-conntrack-percent 91 \
  --sample-ram-available-percent 7 \
  --governor-action-plan-file /tmp/kiro-governor-action-plan.json
```

Write a lab overlay:

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

Generated files:

```text
/tmp/kiro-governor-action-plan.json
/tmp/kiro-governor-actions/governor-action-plan.json
/tmp/kiro-governor-actions/governor-response-overlay.json
```

The overlay describes temporary changes such as lower rate limits, new-client
challenge, cache boost, expensive route disablement, or admin/known-client
lockdown. It is a lab file and does not reload proxy/firewall.

## Safety Rules

- Escalate to a higher level immediately when a sample crosses a threshold.
- Downgrade only after `min_level_hold_seconds`, `cooldown_seconds`, and enough
  `require_recovery_samples`.
- Phase 13 action overlays write lab files only. Real firewall/proxy/WAF apply
  must still go through dedicated validation and rollback phases.
