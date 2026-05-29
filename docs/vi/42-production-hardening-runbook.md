# Runbook Hardening Production Và DDoS

Runbook này mô tả cách đưa `kiro_waf` từ VPS smoke sang pilot thật cho
`vutrungocrong.fun`.

## Giới Hạn Công Suất

Không claim một VPS đơn lẻ chống được 50Gbps hoặc 10.000.000 request/s nếu chưa
có bằng chứng từ nhà mạng/CDN/lab. Để đạt ngưỡng đó cần ít nhất:

- CDN/Anycast/WAF biên như Cloudflare để hấp thụ L7 request flood.
- Upstream DDoS scrubbing của nhà cung cấp VPS để chặn 50Gbps trước khi vào NIC.
- Origin lock để chỉ Cloudflare hoặc IP quản trị vào được origin.
- Benchmark có traffic generator riêng, không chạy trên cùng VPS.

Code hiện tại chỉ có thể bảo vệ ở mức host/origin: XDP drop sớm, rate-limit
L3/L4 theo source IP, subnet `/24`, TCP SYN, UDP, ICMP, nftables,
Nginx/WAF/Bot/rate limit và governor để giữ backend sống lâu hơn.

## Phân Tách Provider Và Protected Server

Provider server:

- Chạy `kiro-provider`.
- Giữ private signing key tại `/etc/kiro-provider/ed25519-private.key`.
- Phát hành license/update/revocation.
- Không chạy proxy traffic khách hàng.
- API nên dùng scoped token:

```bash
kiro-provider --config configs/provider.example.yaml serve-lab-api \
  --listen 127.0.0.1:8080 \
  --auth-token-scope agent-read=updates:read,revocations:read,licenses:read \
  --auth-token-scope ingest=health:write,incidents:write \
  --rate-limit-per-minute 120
```

Protected server/client:

- Chạy `kiro-agent` và `kiro-cli`.
- Chỉ giữ `license.json` và `provider-public-key.pem`.
- Không bao giờ có provider private key.
- License ràng buộc fingerprint máy, kiểm tra signature và revocation list.

Lưu ý thực tế: không có cơ chế chống crack tuyệt đối nếu attacker kiểm soát được
binary và root. Mục tiêu là làm giả mạo khó hơn: signed license, machine binding,
revocation, audit và update ký số.

## Domain `vutrungocrong.fun`

DNS hiện trả về Cloudflare IP. Đây là trạng thái phù hợp nếu Cloudflare proxy đã
bật. Khi đó:

- Origin VPS không nên public trực tiếp HTTP/HTTPS ngoài Cloudflare.
- Firewall chỉ mở SSH cho admin CIDR và HTTP/HTTPS cho Cloudflare ranges.
- Nginx phải đọc real client IP từ `CF-Connecting-IP`.

Template config nằm ở:

```text
configs/vutrungocrong.fun.example.yaml
```

Trước khi apply firewall thật, thay `203.0.113.10/32` bằng IP quản trị thật.

## XDP Map Sync

Sau khi attach XDP, sync config/map:

```bash
kiro-agent \
  --config /etc/kiro/kiro.yaml \
  --xdp-sync-maps-lab \
  --xdp-map-ack KIRO_LAB_XDP_MAP_SYNC
```

File input:

```text
/etc/kiro/xdp-allowlist.txt
/etc/kiro/xdp-blocklist.txt
```

Mỗi dòng là IPv4 hoặc CIDR, ví dụ:

```text
203.0.113.10
198.51.100.0/24
```

`kiro_config` cũng nhận các ngưỡng từ `server_protection.ddos`:

- `per_ip_pps`
- `per_subnet24_pps`
- `syn_per_ip_per_second`
- `udp_per_ip_per_second`
- `icmp_per_ip_per_second`

Nếu các ngưỡng này lớn hơn `0`, XDP bật LRU map `ipv4_rate_state` để drop sớm
theo cửa sổ 1 giây.

## XDP Attach Generic Có Rollback

Chỉ chạy khi có console fallback từ provider:

```bash
cd /opt/kiro_waf
KIRO_LAB_XDP_APPLY=1 \
KIRO_XDP_INTERFACE=eth0 \
KIRO_XDP_MODE=generic \
KIRO_XDP_CONFIRM=0 \
bash scripts/vps-xdp-generic-apply-lab.sh /opt/kiro_waf
```

Nếu SSH vẫn ổn và log sạch, confirm:

```bash
/opt/kiro_waf/build/vps/kiro-agent \
  --config /opt/kiro_waf/configs/kiro.advanced.example.yaml \
  --xdp-confirm \
  --xdp-state-dir /var/lib/kiro
```

Nếu không confirm, rollback watcher sẽ detach sau rollback timer.

## Firewall Thật

Chưa apply firewall thật nếu chưa có:

- admin CIDR thật;
- console fallback;
- Cloudflare IP ranges đã cập nhật;
- dry-run reviewed;
- rollback command đã test.

## Bước Sau Phase 21

- Đọc thêm [runbook VPS homepage/provider-client](43-vps-homepage-provider-client-runbook.md)
  để cài trang chủ, issue license VPS và replace XDP mới.
- Thêm service/timer cho revocation/update sync.
- Tạo production provider host riêng.
- Chạy pilot 30 ngày trước khi bán production.
- Dựng traffic lab riêng để đo PPS/conntrack/CPU-RAM.
