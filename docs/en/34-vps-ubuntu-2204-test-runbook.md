# Ubuntu 22.04 VPS Test Runbook

This runbook uploads the repo to an Ubuntu 22.04 VPS and runs a safe smoke. By
default it does not apply real firewall rules, reload real Nginx, or attach XDP.

## Goals

- Confirm the VPS can build all Go binaries.
- Run `go test ./...` on a real host.
- Validate config and preflight.
- Build the XDP object if `clang` and `llvm-objdump` are available.
- Run a local lab benchmark and save initial evidence.

## VPS Preparation

Recommended baseline:

```bash
apt-get update
apt-get install -y git rsync ca-certificates clang llvm bpftool iproute2 nftables nginx
```

Install the Go version declared in `go.mod` or newer before building. If only
Go/config smoke is required, `clang`/`bpftool` can be missing; the smoke will
warn and skip XDP object build when the toolchain is unavailable.

## Upload And Run From Local Machine

Do not write the password into a file. Pass it via environment variables:

```bash
export KIRO_VPS_HOST=example.vps.ip
export KIRO_VPS_USER=root
export KIRO_VPS_PASSWORD='set-in-shell-only'
export KIRO_REMOTE_DIR=/opt/kiro_waf
# Optional: use a lab-specific known_hosts file instead of editing ~/.ssh/known_hosts.
export KIRO_SSH_KNOWN_HOSTS_FILE=/tmp/kiro-vps-known-hosts

bash scripts/vps-upload-smoke.sh
```

The script creates the remote directory, uploads the repo with `rsync`, excludes
common local build/cache/secret paths, and runs:

```bash
bash scripts/vps-ubuntu2204-smoke.sh /opt/kiro_waf
```

## Run Directly On The VPS

```bash
cd /opt/kiro_waf
bash scripts/vps-ubuntu2204-smoke.sh /opt/kiro_waf
```

Artifacts are written to:

```text
build/vps/
```

This includes binaries, benchmark JSON, benchmark evidence JSON, preflight JSON,
and XDP plan.

## Load/Benchmark Lab

The smoke runs a short benchmark. For a longer loop:

```bash
KIRO_BENCHMARK_ITERATIONS=20000 bash scripts/vps-load-lab.sh /opt/kiro_waf
```

This benchmark only measures local generator/evaluator paths. It does not
generate attack traffic and must not be used as a public DDoS capacity claim.

## Real XDP Attach

Only run this on an isolated lab VPS with provider/VNC fallback console access
after reading the XDP runbook:

```bash
kiro-agent \
  --config configs/kiro.advanced.example.yaml \
  --xdp-apply-lab \
  --xdp-interface eth0 \
  --xdp-mode generic \
  --xdp-program build/vps/kiro_xdp_drop.o \
  --xdp-lab-ack KIRO_LAB_XDP_APPLY \
  --xdp-health-command "ping -c 1 1.1.1.1"
```

If access is lost, use the provider console to detach or roll back according to
[the XDP runbook](32-xdp-ebpf-vps-runbook.md).

## Pass/Fail

Minimum pass:

- `go test ./...` passes.
- `kiro-agent`, `kiro-cli`, and `kiro-provider` build.
- Config check passes.
- Preflight JSON is produced.
- Benchmark/evidence JSON is produced.
- XDP plan is produced; XDP object build passes when the toolchain is installed.

Failures to fix before production:

- Go is missing or build fails.
- Config/runtime expansion fails.
- Preflight has `fail`.
- Benchmark command fails.
- There is no fallback console for any real firewall/XDP apply.
