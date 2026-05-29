# Runbook Benchmark Lab

Benchmark lab tạo report JSON cục bộ để kiểm tra độ trễ/throughput của các
khối đã có trong MVP mà không sinh traffic tấn công ra mạng public.

## Chạy Benchmark An Toàn

```text
go run ./cmd/kiro-agent \
  --config configs/tenant.full-cloudflare.example.yaml \
  --benchmark-lab \
  --benchmark-iterations 5000 \
  --benchmark-temp-ban-size 5000 \
  --benchmark-output-file /tmp/kiro-benchmark.json \
  --benchmark-evidence-file /tmp/kiro-benchmark-evidence.json
```

Lệnh luôn in JSON ra stdout. Nếu truyền `--benchmark-output-file`, cùng report
được ghi ra file để đính kèm vào support bundle hoặc so sánh giữa các lần chạy.
`--benchmark-evidence-file` ghi checklist evidence để dùng trong go/no-go.

## Metric Được Đo

- `http_lab_rps_waf_off`: baseline vòng xử lý local khi WAF tắt.
- `http_lab_rps_waf_on`: throughput và p50/p95/p99 của WAF evaluator trên
  request benign.
- `bot_evaluate_rps`: throughput và p50/p95/p99 của bot scoring/challenge.
- `nftables_generate_ms`: thời gian sinh ruleset nftables deterministic.
- `nftables_temp_ban_statement_rps`: tốc độ sinh statement temp-ban.
- `proxy_generate_ms`: thời gian sinh config Nginx.

## Metric Chưa Claim

Các metric sau được đánh dấu `not_measured` trong report:

- `xdp_pps_drop`.
- `conntrack_pressure`.
- `cpu_ram_under_attack`.

Các mục này cần host lab cô lập, traffic generator hợp pháp, counter kernel và
runbook riêng. Không dùng benchmark cục bộ này làm claim chống DDoS public.

## XDP Safe Plan

Phase 16 thêm plan attach/detach XDP. Phase 19 bổ sung source eBPF C, build
object và attach/detach lab có rollback:

```text
go run ./cmd/kiro-agent \
  --config configs/kiro.advanced.example.yaml \
  --xdp-plan-lab \
  --xdp-output-file /tmp/kiro-xdp-plan.json \
  --xdp-interface eth0 \
  --xdp-mode generic
```

Plan JSON chứa `attach_command`, `detach_command`, `rollback_command`, safety
notes và trạng thái `plan_only`. Chỉ chạy attach thật trong lab cô lập có console
ngoài băng và health check tự detach.

Source eBPF nằm ở `ebpf/xdp/kiro_xdp_drop.c`; build nhanh bằng
`scripts/build-xdp.sh` hoặc `kiro-agent --xdp-build-object`.

## Kiểm Tra Report

```text
rg '"status": "not_measured"' /tmp/kiro-benchmark.json
rg '"name": "http_lab_rps_waf_on"' /tmp/kiro-benchmark.json
```

Report phải có `version`, `goos`, `goarch`, `cpu_count`, `mode`, `plan`,
`iterations`, `temp_ban_size`, danh sách `metrics` và `notes`.
