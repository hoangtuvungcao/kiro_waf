# Kiến Trúc

## Luồng full mode

```text
Internet
  |
Cloudflare Free, nếu bật
  |
Origin firewall chỉ cho Cloudflare vào 80/443
  |
XDP/eBPF fast drop
  |
TC/eBPF + nftables raw/notrack
  |
conntrack-protected firewall
  |
Nginx/HAProxy gateway
  |
Bot scoring + challenge + route quota
  |
WAF/API validation
  |
Backend ứng dụng
  |
Runtime security + resource governor
```

## Luồng server mode

```text
Internet
  |
XDP/eBPF fast drop
  |
TC/eBPF + nftables raw/notrack
  |
conntrack-protected firewall
  |
Dịch vụ server được bảo vệ
  |
Runtime security + resource governor
```

## Dịch vụ chính

### kiro-agent

Chạy trên server khách hàng.

Chức năng:

- Đọc và validate `/etc/kiro/kiro.yaml`.
- Kiểm tra license local.
- Load XDP/eBPF.
- Quản lý BPF maps.
- Quản lý nftables dynamic sets.
- Generate cấu hình Nginx/HAProxy trong `full` mode.
- Thu thập metrics.
- Tự chuyển mức phòng thủ: `NORMAL`, `ELEVATED`, `ATTACK`, `LOCKDOWN`.
- Ghi event vào JSONL.
- Apply update/policy đã ký số.
- Rollback khi cấu hình mới lỗi.

### kiro-cli

Công cụ quản trị local:

```text
kiro status
kiro health
kiro mode show
kiro mode set server
kiro mode set full
kiro license show
kiro license activate --key KIRO-XXXX-XXXX
kiro block ip 1.2.3.4 --ttl 1h
kiro top ip
kiro update check
kiro rollback list
```

### kiro-provider

Phần quản lý của nhà cung cấp. Bản đơn giản dùng thư mục file, không dùng SQL.

Chức năng:

- Tạo license.
- Quản lý key list.
- Quản lý server đã kích hoạt.
- Sinh update manifest ký số.
- Lưu health report, incident và lịch sử bảo hành.

## Module nội bộ

```text
internal/
  admission/      route quota, concurrency, overload queue
  bot/            score, cookie challenge, JS challenge
  config/         load, validate, hot reload
  ddos/           phát hiện L3/L4
  ebpf/           loader, maps, ring buffer events
  firewall/       nftables manager
  governor/       CPU/RAM/load/conntrack mode switch
  license/        verify license, fingerprint, feature gate
  policy/         policy bundle đã ký số
  response/       block, greylist, lockdown
  runtime/        process/file/network monitor
  storage/        file storage JSON/YAML/JSONL
  update/         update manifest đã ký số
  waf/            Coraza/ModSecurity/OWASP CRS
```

