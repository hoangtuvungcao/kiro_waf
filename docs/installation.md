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

Tính năng Community:
- 1 domain
- Rate limit 60 RPM/IP
- WAF + Bot detection cơ bản
- Không giới hạn bandwidth
- Không có XDP filter
- Cập nhật thủ công

### Pro Plan (Có license key)

```bash
curl -fsSL https://firewall.vpsgen.com/install.sh | bash -s -- --key KIRO-XXXX-XXXX
```

Tính năng Pro:
- 5 domains
- Rate limit 120 RPM/IP
- XDP/eBPF filter
- OTA auto-update
- Priority support

### Enterprise Plan

```bash
curl -fsSL https://firewall.vpsgen.com/install.sh | bash -s -- --key KIRO-ENT-XXXX-XXXX
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

# Chỉ định mode
curl -fsSL https://firewall.vpsgen.com/install.sh | bash -s -- --mode full

# Chỉ định master URL
curl -fsSL https://firewall.vpsgen.com/install.sh | bash -s -- --master-url https://firewall.vpsgen.com

# Server mode only (không reverse proxy)
curl -fsSL https://firewall.vpsgen.com/install.sh | bash -s -- --mode server
```

## Manual Installation

### Bước 1: Cài đặt dependencies

```bash
apt update
apt install -y nginx nftables curl jq
```

### Bước 2: Tải binary

```bash
# Tải từ master server
curl -fsSL https://firewall.vpsgen.com/download/kiro-client -o /usr/local/bin/kiro-client
curl -fsSL https://firewall.vpsgen.com/download/kiro-cli -o /usr/local/bin/kiro-cli
chmod +x /usr/local/bin/kiro-client /usr/local/bin/kiro-cli
```

### Bước 3: Tạo cấu hình

```bash
mkdir -p /etc/kiro
cat > /etc/kiro/kiro.yaml << 'EOF'
mode: full
plan: community
license_key: ""

admin:
  allow_ips:
    - YOUR_ADMIN_IP/32

server:
  interface: eth0
  ssh_port: 22

website:
  enabled: true
  cloudflare: true
  tls_mode: flexible_http
  sites:
    - domains:
        - yourdomain.com
      backend: http://127.0.0.1:3000

protection:
  profile: balanced
  waf: true
  bot: true
EOF
```

### Bước 4: Cài đặt systemd service

```bash
cat > /etc/systemd/system/kiro-client-waf.service << 'EOF'
[Unit]
Description=Kiro WAF Client
After=network-online.target nginx.service
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/kiro-client --config /etc/kiro/kiro.yaml
Restart=on-failure
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable --now kiro-client-waf
```

### Bước 5: Kiểm tra

```bash
# Kiểm tra trạng thái
kiro-cli status --config /etc/kiro/kiro.yaml

# Kiểm tra health
kiro-cli health --config /etc/kiro/kiro.yaml

# Preflight check
kiro-cli preflight --config /etc/kiro/kiro.yaml
```

## Post-Installation

### Cấu hình Nginx

Kiro tự động tạo config Nginx tại `/etc/nginx/sites-available/kiro-waf.conf`. Kiểm tra:

```bash
nginx -t
systemctl reload nginx
```

### Cấu hình nftables

Áp dụng rules:

```bash
# Xem rules sẽ được áp dụng
kiro-cli install plan --config /etc/kiro/kiro.yaml

# Áp dụng
nft -f /etc/kiro/nftables/kiro-full-mode.nft
```

### Cấu hình Cloudflare

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
  --binary-path /usr/local/bin/kiro-client \
  --service kiro-client-waf

# Rollback nếu có lỗi
kiro-cli update rollback \
  --binary-path /usr/local/bin/kiro-client \
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
