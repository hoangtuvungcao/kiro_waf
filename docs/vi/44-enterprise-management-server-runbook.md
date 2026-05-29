# Runbook Management Server Enterprise

Runbook này mô tả phần Management Server cho `firewall.vpsgen.com`.

## Kiến Trúc

Management Server:

- Domain: `firewall.vpsgen.com`
- Binary: `kiro-provider`
- API nội bộ: `127.0.0.1:8080`
- Public qua Nginx: `http://firewall.vpsgen.com`
- Private key: `/etc/kiro-provider/ed25519-private.key`
- Storage: `/var/lib/kiro-provider`
- Dashboard: `/var/www/kiro-console`

Client Node / Edge Shield:

- Binary: `kiro-agent`, `kiro-cli`
- Chỉ giữ `license.json` và `provider-public-key.pem`
- Chạy XDP/nftables/proxy/WAF local
- Gửi telemetry/incident về Management Server

## API License

Các endpoint quản trị mới:

```text
GET  /api/v1/summary
GET  /api/v1/licenses
POST /api/v1/licenses
GET  /api/v1/licenses/{license_id}
POST /api/v1/licenses/check
POST /api/v1/licenses/{license_id}/renew
POST /api/v1/licenses/{license_id}/revoke
```

Scope token:

- `licenses:*` cho issue/check/renew/revoke/list.
- `admin` cho summary.
- `updates:read`, `revocations:read`, `health:write`, `incidents:write` cho
  agent/client.

Chạy API:

```bash
kiro-provider --config /etc/kiro-provider/provider.yaml serve-api \
  --listen 127.0.0.1:8080 \
  --auth-token-scope "${KIRO_PROVIDER_ADMIN_TOKEN}=*" \
  --rate-limit-per-minute 600
```

## Cài Tự Động

```bash
cd /opt/kiro_waf
sudo KIRO_MANAGEMENT_DOMAIN=firewall.vpsgen.com \
  bash scripts/vps-management-server-setup.sh /opt/kiro_waf
```

Script sẽ:

- build/cài `kiro-provider`;
- cài config `configs/management.firewall.vpsgen.com.example.yaml`;
- tạo hoặc tái sử dụng Ed25519 signing key;
- tạo `/etc/kiro-provider/api.env`;
- cài systemd service `kiro-provider.service`;
- cài dashboard `site/kiro-console`;
- cấu hình Nginx reverse proxy `/api/`, `/updates/`, `/revocations/`.

## XDP L4 Hiện Tại

File: `ebpf/xdp/kiro_xdp_drop.c`

Chức năng:

- IPv4 allowlist/blocklist qua LPM trie.
- Drop private source IP nếu bật.
- Drop malformed IPv4/TCP nếu bật.
- Drop IPv4 fragment nếu `drop_fragments: true`.
- Rate-limit source IP, subnet `/24`, TCP SYN, UDP, ICMP qua LRU hash.
- Counter pass/drop theo lý do trong `kiro_stats`.

Đây là host-level XDP. Mục tiêu 50Gbps chỉ hợp lệ khi chạy trên hạ tầng đủ NIC,
native/offload mode và upstream scrubbing. Một VPS đơn lẻ không nên dùng để
claim con số đó.

## Dashboard

Source: `site/kiro-console`

Tính năng:

- nhập API token quản trị;
- xem summary license/customer/server/revocation;
- biểu đồ telemetry file health/incidents;
- cấp license mới từ UI;
- hiển thị trạng thái vận hành.

Dashboard không lưu token lên server; token chỉ nằm trong `sessionStorage` của
trình duyệt.
