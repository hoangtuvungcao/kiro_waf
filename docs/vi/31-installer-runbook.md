# Runbook Installer Và Uninstall Lab

Runbook này bổ sung production gate cài đặt bước đầu: tạo install plan JSON,
stage file vào root lab, apply lab có guard, kiểm tra systemd artifact và
uninstall plan/apply. Lệnh `stage-lab` chỉ ghi vào thư mục được truyền qua
`--install-root`, không chạm trực tiếp `/etc`, `/usr`, `/var` hoặc `systemctl`
trên máy hiện tại.

## Build Binary Lab

```text
mkdir -p /tmp/kiro-build
go build -o /tmp/kiro-build/kiro-agent ./cmd/kiro-agent
go build -o /tmp/kiro-build/kiro-cli ./cmd/kiro-cli
```

## Install Plan

```text
go run ./cmd/kiro-cli install plan \
  --config configs/tenant.full-cloudflare.example.yaml \
  --agent-binary /tmp/kiro-build/kiro-agent \
  --cli-binary /tmp/kiro-build/kiro-cli
```

Plan gồm các bước:

- Chạy preflight trước khi cài.
- Tạo `/etc/kiro`, `/var/lib/kiro`, `/var/log/kiro`, `/run/kiro`.
- Copy config, binary và `kiro-agent.service`.
- Chạy firewall dry-run kèm snapshot last-good.
- Chạy proxy dry-run.
- Reload systemd và enable service trong môi trường thật.

Các step ghi rõ `requires_root` để phân biệt plan thật và lab/staging.

## Stage Lab

```text
rm -rf /tmp/kiro-install-root
go run ./cmd/kiro-cli install stage-lab \
  --config configs/tenant.full-cloudflare.example.yaml \
  --install-root /tmp/kiro-install-root \
  --agent-binary /tmp/kiro-build/kiro-agent \
  --cli-binary /tmp/kiro-build/kiro-cli
```

Kết quả mong đợi:

```text
/tmp/kiro-install-root/etc/kiro/kiro.yaml
/tmp/kiro-install-root/etc/systemd/system/kiro-agent.service
/tmp/kiro-install-root/usr/local/bin/kiro-agent
/tmp/kiro-install-root/usr/local/bin/kiro-cli
/tmp/kiro-install-root/var/lib/kiro/install-manifest.json
```

Kiểm tra nhanh:

```text
test -f /tmp/kiro-install-root/var/lib/kiro/install-manifest.json
rg '"target": "/usr/local/bin/kiro-agent"' /tmp/kiro-install-root/var/lib/kiro/install-manifest.json
```

## Apply Lab Vào Install Root

Lệnh này copy file thật vào `--install-root` và ghi manifest apply. Khi có
`--install-root`, các bước `run` như preflight/systemd/firewall/proxy được skip
mặc định để không chạm host hiện tại.

```text
go run ./cmd/kiro-cli install apply-lab \
  --config configs/tenant.full-cloudflare.example.yaml \
  --install-root /tmp/kiro-install-root \
  --agent-binary /tmp/kiro-build/kiro-agent \
  --cli-binary /tmp/kiro-build/kiro-cli \
  --ack KIRO_LAB_INSTALL_APPLY
```

Kết quả thêm:

```text
/tmp/kiro-install-root/var/lib/kiro/install-apply-manifest.json
```

Nếu muốn mô phỏng cả command step bằng runner thật trong môi trường lab root,
truyền `--run-steps`. Không dùng `--run-steps` với `--install-root` trên máy
dev/CI thông thường.

## Apply Lab Vào Root Thật

Chỉ chạy trên VPS lab Ubuntu 22.04/24.04, có console fallback và backup. Khi
không truyền `--install-root`, lệnh yêu cầu root và sẽ chạy các bước command:
preflight, firewall dry-run snapshot, proxy dry-run, `systemctl daemon-reload`
và `systemctl enable --now kiro-agent.service`.

```text
sudo ./kiro-cli install apply-lab \
  --config /path/to/kiro.yaml \
  --agent-binary /tmp/kiro-build/kiro-agent \
  --cli-binary /tmp/kiro-build/kiro-cli \
  --ack KIRO_LAB_INSTALL_APPLY
```

Guard bắt buộc:

- ACK đúng: `KIRO_LAB_INSTALL_APPLY`.
- Root nếu apply vào `/`.
- `/etc/os-release` là Ubuntu 22.04 hoặc 24.04.

## Uninstall Plan

Không purge dữ liệu:

```text
go run ./cmd/kiro-cli install uninstall-plan \
  --config configs/tenant.full-cloudflare.example.yaml
```

Plan mặc định remove service/binary nhưng giữ config, license, state và log.

Purge phá hủy dữ liệu:

```text
go run ./cmd/kiro-cli install uninstall-plan \
  --config configs/tenant.full-cloudflare.example.yaml \
  --purge
```

Chỉ dùng purge khi đã backup và xác nhận muốn xóa sạch config/state/log.

## Uninstall Apply Lab

Không purge dữ liệu:

```text
go run ./cmd/kiro-cli install uninstall-apply-lab \
  --config configs/tenant.full-cloudflare.example.yaml \
  --install-root /tmp/kiro-install-root \
  --ack KIRO_LAB_UNINSTALL_APPLY
```

Lệnh này remove service/binary nhưng giữ config, state và log.

Purge phá hủy dữ liệu:

```text
go run ./cmd/kiro-cli install uninstall-apply-lab \
  --config configs/tenant.full-cloudflare.example.yaml \
  --install-root /tmp/kiro-install-root \
  --ack KIRO_LAB_UNINSTALL_APPLY \
  --purge
```

Khi không có `--install-root`, uninstall apply yêu cầu root và Ubuntu guard như
install apply.

## Giới Hạn

- `stage-lab` không gọi `systemctl`.
- Chưa cài package hệ thống thiếu như `nftables` hoặc `nginx`.
- Apply thật hiện là lab-gated, chưa phải installer wizard production.
- Chưa có rollback tự động cho install apply; uninstall apply là luồng khôi
  phục thủ công chính.
