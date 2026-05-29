# Runbook VPS Homepage, Provider/Client Và XDP Rate Limit

Runbook này là phần nối sau Phase 21 cho `vutrungocrong.fun`.

## Mục Tiêu

- Cài trang chủ tĩnh cơ bản qua Nginx origin.
- Tách provider license server và protected server/client rõ ràng.
- Nâng XDP từ blocklist/allowlist cơ bản lên rate-limit L3/L4 tại kernel.
- Giữ mọi thao tác nguy hiểm ở dạng lab-gated, có ACK và rollback.

## XDP Hiện Tại

Source: `ebpf/xdp/kiro_xdp_drop.c`

Map chính:

- `kiro_config`: cấu hình runtime cho drop/rate-limit.
- `ipv4_allowlist`: IPv4/CIDR luôn pass.
- `ipv4_blocklist`: IPv4/CIDR luôn drop.
- `ipv4_rate_state`: LRU hash lưu cửa sổ rate theo source IP và subnet `/24`.
- `kiro_stats`: counter pass/drop theo lý do.

Các ngưỡng được lấy từ YAML:

```yaml
server_protection:
  xdp:
    enabled: true
    drop_private_source_ip: true
    drop_malformed: true
  ddos:
    per_ip_pps: 3000
    per_subnet24_pps: 30000
    syn_per_ip_per_second: 200
    udp_per_ip_per_second: 500
    icmp_per_ip_per_second: 30
```

Map sync:

```bash
kiro-agent \
  --config /etc/kiro/kiro.yaml \
  --xdp-sync-maps-lab \
  --xdp-map-ack KIRO_LAB_XDP_MAP_SYNC
```

## Cài Provider/Client Trên VPS

Script:

```bash
cd /opt/kiro_waf
sudo bash scripts/vps-provider-client-setup.sh /opt/kiro_waf
```

Kết quả:

- `/usr/local/bin/kiro-provider`, `kiro-agent`, `kiro-cli`.
- `/etc/kiro-provider/provider.yaml`.
- `/etc/kiro-provider/ed25519-private.key` chỉ nằm ở provider side.
- `/etc/kiro/license.json` và `/etc/kiro/provider-public-key.pem` nằm ở
  protected server/client side.
- `kiro-agent --check` verify license theo fingerprint VPS.

Không copy private signing key sang máy khách khác. Nếu cần tách provider thật,
đưa `/etc/kiro-provider` sang VPS provider riêng và chỉ cấp license/public key
cho protected server.

## Cài Trang Chủ

Source:

```text
site/kiro-home/
```

Cài lên Nginx:

```bash
cd /opt/kiro_waf
sudo KIRO_HOME_DOMAIN=vutrungocrong.fun \
  KIRO_HOME_ALT_DOMAIN=www.vutrungocrong.fun \
  bash scripts/vps-install-homepage.sh /opt/kiro_waf
```

Kiểm tra origin:

```bash
curl -i -H 'Host: vutrungocrong.fun' http://127.0.0.1/health
curl -I -H 'Host: vutrungocrong.fun' http://127.0.0.1/
```

Nếu domain bật Cloudflare proxy, response public có thể là Cloudflare challenge
tùy rule ở Cloudflare. Origin local vẫn phải trả `200`.

## Replace XDP An Toàn

Script `scripts/vps-xdp-generic-apply-lab.sh` mặc định detach chương trình cũ
trước khi attach object mới:

```bash
cd /opt/kiro_waf
sudo KIRO_LAB_XDP_APPLY=1 \
  KIRO_XDP_INTERFACE=eth0 \
  KIRO_XDP_MODE=generic \
  KIRO_XDP_CONFIRM=0 \
  bash scripts/vps-xdp-generic-apply-lab.sh /opt/kiro_waf
```

Nếu SSH và health check ổn:

```bash
sudo /opt/kiro_waf/build/vps/kiro-agent \
  --config /opt/kiro_waf/configs/vutrungocrong.fun.example.yaml \
  --xdp-confirm \
  --xdp-state-dir /var/lib/kiro
```

## Giới Hạn Production

- XDP host-level không cứu được khi băng thông upstream đã bị đầy.
- `generic` XDP an toàn hơn cho VPS đầu nhưng chậm hơn `native/offload`.
- Chưa nên apply nftables origin-lock thật nếu chưa có admin CIDR và Cloudflare
  IP ranges đã xác minh.
- Không claim 50Gbps hoặc 10M request/s nếu chưa có CDN/upstream scrubbing và
  traffic-generator lab độc lập.
