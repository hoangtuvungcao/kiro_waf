# Benchmark Lab Runbook

The benchmark lab writes a local JSON report for MVP latency/throughput checks
without generating public attack traffic.

## Run A Safe Benchmark

```text
go run ./cmd/kiro-agent \
  --config configs/tenant.full-cloudflare.example.yaml \
  --benchmark-lab \
  --benchmark-iterations 5000 \
  --benchmark-temp-ban-size 5000 \
  --benchmark-output-file /tmp/kiro-benchmark.json \
  --benchmark-evidence-file /tmp/kiro-benchmark-evidence.json
```

The command always prints JSON to stdout. When `--benchmark-output-file` is
provided, the same report is written to disk for support bundles or comparison
between runs.
`--benchmark-evidence-file` writes an evidence checklist for go/no-go review.

## Measured Metrics

- `http_lab_rps_waf_off`: local baseline loop for the HTTP path with WAF off.
- `http_lab_rps_waf_on`: WAF evaluator throughput and p50/p95/p99 latency on a
  benign request.
- `bot_evaluate_rps`: bot scoring/challenge throughput and p50/p95/p99 latency.
- `nftables_generate_ms`: deterministic nftables ruleset generation time.
- `nftables_temp_ban_statement_rps`: temp-ban statement generation throughput.
- `proxy_generate_ms`: Nginx config generation time.

## Metrics Not Claimed

These metrics are marked `not_measured` in the report:

- `xdp_pps_drop`.
- `conntrack_pressure`.
- `cpu_ram_under_attack`.

They require an isolated lab host, legal traffic generator, kernel counters, and
a separate runbook. Do not use this local benchmark as a public DDoS protection
claim.

## XDP Safe Plan

Phase 16 adds an XDP attach/detach plan. Phase 19 adds eBPF C source, object
build, and lab attach/detach with rollback:

```text
go run ./cmd/kiro-agent \
  --config configs/kiro.advanced.example.yaml \
  --xdp-plan-lab \
  --xdp-output-file /tmp/kiro-xdp-plan.json \
  --xdp-interface eth0 \
  --xdp-mode generic
```

The JSON plan contains `attach_command`, `detach_command`, `rollback_command`,
safety notes, and `plan_only` status. Run a real attach only in an isolated lab
with out-of-band console access and automatic health-check detach.

The eBPF source is `ebpf/xdp/kiro_xdp_drop.c`; build it with
`scripts/build-xdp.sh` or `kiro-agent --xdp-build-object`.

## Check The Report

```text
rg '"status": "not_measured"' /tmp/kiro-benchmark.json
rg '"name": "http_lab_rps_waf_on"' /tmp/kiro-benchmark.json
```

The report should include `version`, `goos`, `goarch`, `cpu_count`, `mode`,
`plan`, `iterations`, `temp_ban_size`, `metrics`, and `notes`.
