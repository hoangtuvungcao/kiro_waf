# Runbook Test VPS Ubuntu 22.04

Runbook này dùng để upload repo lên VPS Ubuntu 22.04 và chạy smoke an toàn. Mặc
định không apply firewall thật, không reload Nginx thật, không attach XDP thật.

## Mục Tiêu

- Xác nhận VPS build được toàn bộ binary Go.
- Chạy `go test ./...` trên host thật.
- Validate config và preflight.
- Build XDP object nếu VPS có `clang` và `llvm-objdump`.
- Chạy benchmark lab local để lưu evidence ban đầu.

## Chuẩn Bị VPS

Khuyến nghị tối thiểu:

```bash
apt-get update
apt-get install -y git rsync ca-certificates clang llvm bpftool iproute2 nftables nginx
```

Cài Go đúng version trong `go.mod` hoặc mới hơn trước khi build. Nếu chỉ muốn
smoke Go/config trước, có thể thiếu `clang`/`bpftool`; script sẽ ghi cảnh báo và
bỏ qua build XDP object khi thiếu toolchain.

## Upload Và Chạy Smoke Từ Máy Local

Không ghi mật khẩu vào file. Truyền qua biến môi trường khi chạy:

```bash
export KIRO_VPS_HOST=example.vps.ip
export KIRO_VPS_USER=root
export KIRO_VPS_PASSWORD='set-in-shell-only'
export KIRO_REMOTE_DIR=/opt/kiro_waf
# Tùy chọn: dùng known_hosts riêng cho lab, không sửa ~/.ssh/known_hosts.
export KIRO_SSH_KNOWN_HOSTS_FILE=/tmp/kiro-vps-known-hosts

bash scripts/vps-upload-smoke.sh
```

Script sẽ:

- tạo thư mục remote nếu chưa có;
- `rsync` repo lên VPS, bỏ qua `.git`, `build`, `tmp`, cache và secret thường
  gặp;
- chạy `bash scripts/vps-ubuntu2204-smoke.sh /opt/kiro_waf` trên VPS.

## Chạy Trực Tiếp Trên VPS

```bash
cd /opt/kiro_waf
bash scripts/vps-ubuntu2204-smoke.sh /opt/kiro_waf
```

Artifact chính nằm trong:

```text
build/vps/
```

Bao gồm binary, benchmark JSON, benchmark evidence JSON, preflight JSON và XDP
plan.

## Load/Benchmark Lab

Smoke mặc định chạy benchmark ngắn. Để chạy vòng dài hơn:

```bash
KIRO_BENCHMARK_ITERATIONS=20000 bash scripts/vps-load-lab.sh /opt/kiro_waf
```

Benchmark này chỉ đo local generator/evaluator. Nó không tạo traffic tấn công và
không phải số liệu DDoS public capacity.

## XDP Attach Thật

Chỉ chạy khi VPS là lab cô lập, có console fallback từ provider/VNC và đã đọc
runbook XDP:

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

Nếu mất kết nối, dùng console provider để chạy detach/rollback theo
[runbook XDP](39-xdp-ebpf-vps-runbook.md).

## Pass/Fail

Pass tối thiểu:

- `go test ./...` pass.
- 3 binary `kiro-agent`, `kiro-cli`, `kiro-provider` build xong.
- Config check pass.
- Preflight JSON sinh được.
- Benchmark/evidence JSON sinh được.
- XDP plan sinh được; XDP object build pass nếu toolchain có sẵn.

Fail cần xử lý trước production:

- thiếu Go hoặc build fail;
- config/runtime expansion fail;
- preflight có `fail`;
- benchmark command lỗi;
- VPS không có console fallback nếu muốn apply firewall/XDP thật.
