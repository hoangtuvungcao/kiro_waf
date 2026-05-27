# Cấu Hình Tối Giản Cho Mô Hình Cho Thuê

## Kết luận

Cấu hình đầy đủ hơn 200 dòng là quá dài cho người thuê bình thường. Với mô hình
cho thuê, phải tách cấu hình thành 3 lớp:

```text
Lớp 1: tenant simple config
  Người thuê hoặc kỹ thuật hỗ trợ chỉ sửa vài dòng.

Lớp 2: provider profile
  Nhà cung cấp định nghĩa profile light/balanced/strict/lockdown.

Lớp 3: advanced config
  Kỹ thuật cao dùng khi cần chỉnh XDP, nftables, WAF, bot, governor chi tiết.
```

## File cấu hình nên dùng

Người thuê bình thường:

```text
/etc/kiro/kiro.yaml
```

Mẫu:

```text
configs/kiro.example.yaml
configs/tenant.server-only.example.yaml
configs/tenant.full-cloudflare.example.yaml
configs/tenant.full-strict.example.yaml
```

Kỹ thuật/provider:

```text
configs/kiro.advanced.example.yaml
```

## Cấu hình tối giản full mode

```yaml
mode: full
plan: school_smb
license_key: KIRO-XXXX-XXXX

admin:
  allow_ips:
    - 203.0.113.10/32

server:
  interface: eth0
  ssh_port: 22

website:
  enabled: true
  cloudflare: true
  tls_mode: flexible_http
  sites:
    - domains:
        - example.com
        - www.example.com
      backend: http://127.0.0.1:3000

protection:
  profile: balanced
  waf: true
  bot: true
  auto_attack_mode: true
```

Người thuê chỉ cần hiểu:

- `mode`: bảo vệ server hay cả website.
- `license_key`: key thuê dịch vụ.
- `admin.allow_ips`: IP quản trị.
- `website.sites.domains`: domain.
- `website.sites.backend`: app thật đang chạy ở đâu.
- `tls_mode`: dùng Cloudflare Flexible hay Full Strict.
- `protection.profile`: mức bảo vệ.

## Profile bảo vệ

### light

Dùng cho website ít traffic, ít rủi ro false positive.

- Rate limit nhẹ.
- WAF bật rule cơ bản.
- Bot challenge ít.

### balanced

Mặc định cho trường học/SMB.

- WAF bật OWASP CRS.
- Bot scoring bật.
- Cloudflare origin lock bật nếu dùng Cloudflare.
- Auto attack mode bật.

### strict

Dùng khi bị tấn công thường xuyên hoặc website có login/API.

- Route `/login`, `/api`, `/upload` bị giới hạn chặt hơn.
- Bot challenge mạnh hơn.
- Runtime alert bật đầy đủ.

### lockdown

Dùng khi đang bị tấn công.

- Chỉ cho client tin cậy/admin.
- Giảm route nặng.
- Ưu tiên giữ backend sống.

## Cách agent xử lý config tối giản

Agent sẽ expand config tối giản thành advanced config nội bộ:

```text
tenant config
  + provider profile
  + license entitlement
  + default rules
  = runtime config đầy đủ
```

Người thuê không cần nhìn thấy:

- XDP thresholds.
- nftables chain chi tiết.
- Cloudflare IP ranges.
- WAF anomaly score.
- Bot score weight.
- Governor hysteresis.
- File path nội bộ.

## Khi nào cần advanced config?

Chỉ dùng khi:

- Một domain có nhiều backend phức tạp.
- Nhiều domain dùng policy khác nhau.
- Cần custom route quota.
- Cần cert/key Full Strict nhiều domain.
- Cần mở port custom.
- Cần chỉnh threshold DDoS riêng.
- Khách hàng professional/enterprise-lite.

## Quy tắc sản phẩm cho thuê

- Mặc định đưa người thuê dùng config tối giản.
- Provider giữ profile mặc định và update profile qua signed policy.
- Advanced config chỉ mở cho kỹ thuật hoặc gói cao hơn.
- CLI/wizard nên sinh config tối giản, không bắt người dùng sửa YAML dài.

