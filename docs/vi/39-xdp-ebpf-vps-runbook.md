# Runbook XDP/eBPF Và Chuẩn Bị VPS

XDP/eBPF cần 2 phần:

- Program chạy trong kernel: repo dùng C ở `ebpf/xdp/kiro_xdp_drop.c`.
- Loader/orchestrator: repo dùng Go/CLI `kiro-agent` để build object, tạo plan,
  attach lab, detach/rollback và confirm.

Không cần C++ cho phần hiện tại. eBPF thường viết bằng C hoặc Rust; dùng C là
đường ổn định nhất cho VPS Ubuntu vì `clang -target bpf` hỗ trợ trực tiếp.

## Trạng Thái Hiện Tại

Đã có:

- Source eBPF C: `ebpf/xdp/kiro_xdp_drop.c`.
- Maps: `ipv4_allowlist`, `ipv4_blocklist`, `kiro_config`, `kiro_stats`.
- Default runtime an toàn: nếu chưa set map config/blocklist thì phần lớn
  traffic `XDP_PASS`.
- Build script: `scripts/build-xdp.sh`.
- CLI build object: `kiro-agent --xdp-build-object`.
- CLI plan/apply/detach/confirm lab có ACK và root guard.

Chưa claim production:

- Chưa chạy attach thật trong CI.
- Chưa có map manager production để tự đồng bộ allowlist/blocklist vào BPF maps.
- Chưa có PPS benchmark trên VPS thật.

## Build Object

Trên VPS/lab Ubuntu cài toolchain:

```text
sudo apt-get update
sudo apt-get install -y clang llvm linux-tools-common linux-tools-generic iproute2 bpftool
```

Build bằng script:

```text
scripts/build-xdp.sh
```

Hoặc build bằng agent:

```text
go run ./cmd/kiro-agent \
  --xdp-build-object \
  --xdp-source ebpf/xdp/kiro_xdp_drop.c \
  --xdp-build-output build/ebpf/kiro_xdp_drop.o \
  --xdp-target-arch x86
```

Cài object vào path runtime:

```text
sudo install -D -m 0644 build/ebpf/kiro_xdp_drop.o /usr/lib/kiro/xdp/kiro_xdp_drop.o
```

## Plan Trước Khi Attach

```text
sudo ./kiro-agent \
  --config /etc/kiro/kiro.yaml \
  --xdp-plan-lab \
  --xdp-output-file /var/lib/kiro/xdp-plan.json \
  --xdp-interface eth0 \
  --xdp-mode generic
```

Dùng `generic` cho lần đầu trên VPS. Sau khi xác minh driver/kernel ổn mới thử
`native`.

## Attach Lab Có Rollback

Chỉ chạy khi có console ngoài băng của VPS.

```text
sudo ./kiro-agent \
  --config /etc/kiro/kiro.yaml \
  --xdp-apply-lab \
  --xdp-lab-ack KIRO_LAB_XDP_APPLY \
  --xdp-interface eth0 \
  --xdp-mode generic \
  --xdp-program /usr/lib/kiro/xdp/kiro_xdp_drop.o \
  --xdp-health-command "ping -c1 -W1 1.1.1.1 >/dev/null" \
  --xdp-rollback-seconds 60
```

Nếu health command fail, agent tự detach. Nếu health ổn:

```text
sudo ./kiro-agent --config /etc/kiro/kiro.yaml --xdp-confirm
```

Detach thủ công:

```text
sudo ./kiro-agent \
  --config /etc/kiro/kiro.yaml \
  --xdp-detach-lab \
  --xdp-lab-ack KIRO_LAB_XDP_APPLY
```

Rollback nếu pending quá hạn:

```text
sudo ./kiro-agent \
  --config /etc/kiro/kiro.yaml \
  --xdp-rollback-if-expired \
  --xdp-lab-ack KIRO_LAB_XDP_APPLY
```

## VPS Gate Trước Khi Upload

- `go test ./...` pass trên máy dev.
- `bash scripts/ci-phase10-smoke.sh` pass.
- `kiro-cli preflight` trên VPS không có check critical fail.
- Build được `kiro_xdp_drop.o` trên VPS hoặc copy artifact đã build đúng arch.
- Apply firewall/proxy/XDP đều chạy ở mode lab trước, có pending rollback và
  confirm sau khi health pass.
