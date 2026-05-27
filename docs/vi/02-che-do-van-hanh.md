# Chế Độ Vận Hành

## mode: server

Dùng khi khách hàng chỉ cần bảo vệ server, game server, API port riêng, SSH hoặc
dịch vụ mạng khác. Website protection không hoạt động.

Bật:

- XDP/eBPF.
- TC/eBPF.
- nftables.
- Anti-DDoS L3/L4.
- Conntrack protection.
- Runtime security.
- Resource governor.
- License validation.

Tắt:

- Reverse proxy website.
- WAF HTTP.
- Bot challenge website.
- Route quota HTTP.
- Cloudflare origin lock.

```yaml
mode: server
server_protection:
  enabled: true
website_protection:
  enabled: false
```

## mode: full

Dùng khi server có website hoặc API public.

Bật:

- Tất cả chức năng của `server`.
- Nginx/HAProxy gateway.
- WAF/API validation.
- Bot scoring.
- Cookie/JS challenge.
- Route quota.
- Cloudflare Free origin lock nếu cấu hình yêu cầu.

```yaml
mode: full
server_protection:
  enabled: true
website_protection:
  enabled: true
  cloudflare:
    enabled: true
    require_proxied_traffic: true
    block_direct_origin_http: true
```

## Mức phòng thủ runtime

Mức phòng thủ khác với mode sản phẩm.

```text
NORMAL      bình thường
ELEVATED    nghi ngờ có tấn công hoặc tải tăng
ATTACK      đang bị tấn công rõ ràng
LOCKDOWN    ưu tiên giữ server/backend sống
```

Trigger:

- CPU cao.
- RAM thấp.
- Load average cao.
- Conntrack gần đầy.
- PPS/BPS vượt baseline.
- Nginx active connections tăng đột biến.
- Backend latency cao.
- Database ứng dụng gần hết connection.

