# XDP/eBPF And VPS Preparation Runbook

XDP/eBPF has two parts:

- Kernel program: this repo uses C in `ebpf/xdp/kiro_xdp_drop.c`.
- Loader/orchestrator: this repo uses the Go `kiro-agent` CLI to build the
  object, create a plan, attach in lab mode, detach/rollback, and confirm.

C++ is not needed for the current implementation. eBPF is usually written in C
or Rust; C is the most stable path for Ubuntu VPS hosts because
`clang -target bpf` supports it directly.

## Current State

Implemented:

- eBPF C source: `ebpf/xdp/kiro_xdp_drop.c`.
- Maps: `ipv4_allowlist`, `ipv4_blocklist`, `kiro_config`, `kiro_stats`.
- Safe default runtime: without map config/blocklist, most traffic is
  `XDP_PASS`.
- Build script: `scripts/build-xdp.sh`.
- CLI object build: `kiro-agent --xdp-build-object`.
- CLI lab plan/apply/detach/confirm with acknowledgement and root guard.

Not production-claimed yet:

- CI does not attach XDP to a real interface.
- There is no production map manager that continuously syncs allow/block lists
  into BPF maps.
- PPS benchmarking still needs a real VPS/lab host.

## Build Object

Install the toolchain on the VPS/lab host:

```text
sudo apt-get update
sudo apt-get install -y clang llvm linux-tools-common linux-tools-generic iproute2 bpftool
```

Build with the script:

```text
scripts/build-xdp.sh
```

Or build with the agent:

```text
go run ./cmd/kiro-agent \
  --xdp-build-object \
  --xdp-source ebpf/xdp/kiro_xdp_drop.c \
  --xdp-build-output build/ebpf/kiro_xdp_drop.o \
  --xdp-target-arch x86
```

Install the object to the runtime path:

```text
sudo install -D -m 0644 build/ebpf/kiro_xdp_drop.o /usr/lib/kiro/xdp/kiro_xdp_drop.o
```

## Plan Before Attach

```text
sudo ./kiro-agent \
  --config /etc/kiro/kiro.yaml \
  --xdp-plan-lab \
  --xdp-output-file /var/lib/kiro/xdp-plan.json \
  --xdp-interface eth0 \
  --xdp-mode generic
```

Use `generic` for the first VPS run. Try `native` only after confirming the
driver/kernel path is healthy.

## Lab Attach With Rollback

Run only when you have out-of-band VPS console access.

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

If the health command fails, the agent detaches XDP. If healthy:

```text
sudo ./kiro-agent --config /etc/kiro/kiro.yaml --xdp-confirm
```

Manual detach:

```text
sudo ./kiro-agent \
  --config /etc/kiro/kiro.yaml \
  --xdp-detach-lab \
  --xdp-lab-ack KIRO_LAB_XDP_APPLY
```

Rollback when pending state has expired:

```text
sudo ./kiro-agent \
  --config /etc/kiro/kiro.yaml \
  --xdp-rollback-if-expired \
  --xdp-lab-ack KIRO_LAB_XDP_APPLY
```

## VPS Gate Before Upload

- `go test ./...` passes on the dev machine.
- `bash scripts/ci-phase10-smoke.sh` passes.
- `kiro-cli preflight` on the VPS has no critical failures.
- `kiro_xdp_drop.o` builds on the VPS or a matching-arch artifact is copied.
- Firewall/proxy/XDP apply paths run in lab mode first, keep pending rollback,
  and are confirmed only after health passes.
