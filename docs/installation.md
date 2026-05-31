# Installation

## System Requirements

| Yêu cầu | Tối thiểu | Khuyến nghị |
|----------|-----------|-------------|
| OS | Ubuntu 22.04 LTS | Ubuntu 24.04 LTS |
| Architecture | x86_64 | x86_64 |
| RAM | 512 MB | 1 GB+ |
| Disk | 1 GB free | 5 GB+ |
| Network | 1 interface | eth0 |
| Kernel | 5.15+ | 6.x (XDP native) |

## Quick Install

### Community Plan (Miễn phí)

Gói Community tự động đăng ký, không cần license key:

```bash
curl -fsSL https://firewall.vpsgen.com/install.sh | bash
```

Script sẽ tự động gọi `POST /api/v1/register` trên master server để lấy license key miễn phí.
Script tạo file cấu hình YAML tại `/etc/kiro/kiro.yaml` với các giá trị cần thiết (license key, cookie secret, master URL).

Tính năng Community:
- 1 domain
- Rate limit 60 RPM/IP
- WAF + Bot detection + XDP L4 protection
- Không giới hạn bandwidth
- Cập nhật thủ công

### Pro Plan (Có license key)

```bash
curl -fsSL https://firewall.vpsgen.com/install.sh | bash -s -- --license-key KIRO-XXXX-XXXX
```

Tính năng Pro:
- 5 domains
- Rate limit 120 RPM/IP
- XDP/eBPF filter
- OTA auto-update
- Priority support

### Enterprise Plan

```bash
curl -fsSL https://firewall.vpsgen.com/install.sh | bash -s -- --license-key KIRO-ENT-XXXX-XXXX
```

Tính năng Enterprise:
- Unlimited domains
- Custom rate limits
- Full XDP + advanced DDoS
- Dedicated support
- Custom rules

## Install Script Options

```bash
# Xem help
curl -fsSL https://firewall.vpsgen.com/install.sh | bash -s -- --help

# Chỉ định master URL
curl -fsSL https://firewall.vpsgen.com/install.sh | bash -s -- --master-url https://firewall.vpsgen.com

# Bật XDP mode (cài thêm clang, llvm, libbpf-dev)
curl -fsSL https://firewall.vpsgen.com/install.sh | bash -s -- --xdp-mode

# Quiet mode (cho CI/CD, tắt animation và màu sắc)
curl -fsSL https://firewall.vpsgen.com/install.sh | bash -s -- --quiet
```

## Manual Installation

### Bước 1: Cài đặt dependencies

```bash
apt update
apt install -y curl coreutils systemd
```

### Bước 2: Tải binary

```bash
# Tải từ master server (cần license key)
curl -fsSL -H "X-License-Key: YOUR-KEY" \
  https://firewall.vpsgen.com/api/v1/download/client-waf \
  -o /usr/local/bin/kiro-client-waf
curl -fsSL https://firewall.vpsgen.com/download/kiro-cli \
  -o /usr/local/bin/kiro-cli
chmod +x /usr/local/bin/kiro-client-waf /usr/local/bin/kiro-cli
```

### Bước 3: Tạo cấu hình

```bash
mkdir -p /etc/kiro /var/lib/kiro /var/log/kiro

cat > /etc/kiro/kiro.yaml << 'EOF'
# Kiro WAF - Cấu hình thống nhất
mode: full
license_key: YOUR-LICENSE-KEY

admin:
  allow_ips: []

website:
  enabled: true
  cloudflare: true
  tls_mode: flexible_http
  sites:
    - domains: []
      backend: http://127.0.0.1:3000

protection:
  profile: balanced

client:
  cookie_secret: "CHANGE-ME-RANDOM-SECRET"
  master_url: https://firewall.vpsgen.com
  listen_addr: ":8090"
  node_id: my-server
  heartbeat_seconds: 30
  challenge_all_new: true
  cookie_short_ttl: 1800
EOF

chmod 600 /etc/kiro/kiro.yaml
```

> **Lưu ý:** File `/etc/kiro/kiro.yaml` là cấu hình chính cho cả `kiro-client-waf` và `kiro-cli`. Environment variables vẫn được hỗ trợ và sẽ override giá trị YAML nếu được set (xem `docs/configuration.md` để biết chi tiết).

### Bước 4: Cài đặt systemd service

```bash
cat > /etc/systemd/system/kiro-client-waf.service << 'EOF'
[Unit]
Description=Kiro WAF Client - Bảo vệ máy chủ web
Documentation=https://firewall.vpsgen.com
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/kiro-client-waf --config /etc/kiro/kiro.yaml
WorkingDirectory=/var/lib/kiro
Restart=always
RestartSec=5
LimitNOFILE=65535
LimitMEMLOCK=infinity

# Bảo mật
NoNewPrivileges=false
ProtectSystem=full
ProtectHome=true
ReadWritePaths=/var/log/kiro /var/lib/kiro

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=kiro-client-waf

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable --now kiro-client-waf
```

### Bước 5: Kiểm tra

```bash
# Kiểm tra trạng thái service
systemctl status kiro-client-waf

# Xem log realtime
journalctl -u kiro-client-waf -f

# Kiểm tra health endpoint
curl -s http://127.0.0.1:80/healthz

# Kiểm tra trạng thái (cần config YAML — tùy chọn)
# kiro-cli status --config /etc/kiro/kiro.yaml
```

## Post-Installation

### Cấu hình Cloudflare (nếu dùng)

1. Trỏ DNS domain về IP server (Proxied - orange cloud)
2. SSL/TLS mode: Flexible (nếu dùng `flexible_http`) hoặc Full Strict
3. Bật "Always Use HTTPS" trong Cloudflare dashboard

## Upgrade

```bash
# Kiểm tra phiên bản mới
kiro-cli update check --master-url https://firewall.vpsgen.com

# Áp dụng cập nhật
kiro-cli update apply \
  --master-url https://firewall.vpsgen.com \
  --binary-path /usr/local/bin/kiro-client-waf \
  --service kiro-client-waf

# Rollback nếu có lỗi
kiro-cli update rollback \
  --binary-path /usr/local/bin/kiro-client-waf \
  --service kiro-client-waf
```

## Uninstall

```bash
# Xem kế hoạch gỡ cài đặt
kiro-cli install uninstall-plan --config /etc/kiro/kiro.yaml

# Gỡ cài đặt (giữ config)
kiro-cli install uninstall-apply-lab --config /etc/kiro/kiro.yaml --ack KIRO_LAB_UNINSTALL_APPLY

# Gỡ hoàn toàn (xóa config + data)
kiro-cli install uninstall-apply-lab --config /etc/kiro/kiro.yaml --purge --ack KIRO_LAB_UNINSTALL_APPLY
```
