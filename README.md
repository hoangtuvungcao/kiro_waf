# kiro_waf

`kiro_waf` là nền tảng bảo vệ một máy chủ Ubuntu 22.04 LTS theo hướng sản phẩm:
có cấu hình rõ ràng, có bản quyền/key theo máy, có cập nhật ký số, có chế độ
`server` và `full`, nhưng vẫn giữ kiến trúc đơn giản nhất có thể.

Với mô hình cho thuê, người thuê dùng config tối giản khoảng vài chục dòng.
Config nâng cao được giữ riêng cho provider/kỹ thuật.

Provider/agent gốc vẫn hỗ trợ storage dạng file `YAML`, `JSON`, `JSONL` có chữ
ký số khi cần. Đường triển khai standalone mới trong `master-server/` dùng
SQLite để quản lý license, heartbeat và report trên Master Server.

## Thành phần chính

- Anti-DDoS L3/L4: XDP/eBPF, TC/eBPF, nftables.
- Chống quá tải: resource governor, conntrack protection, overload mode.
- Website protection: Nginx/HAProxy, WAF, bot scoring, challenge, route quota.
- Cloudflare Free optional trong `full` mode để giảm bot rác và ẩn IP gốc.
- Bảo mật hệ thống: AppArmor, audit, file integrity, runtime detection.
- Quản lý nhà cung cấp: license/key list, server fingerprint, update manifest.

## Chế độ vận hành

`server`

- Chỉ bảo vệ máy chủ và các dịch vụ mạng.
- Không chạy website gateway, WAF HTTP, bot challenge hoặc Cloudflare origin lock.

`full`

- Bảo vệ cả máy chủ và website/API.
- Bật reverse proxy, WAF, bot defense, route quota.
- Có thể bắt buộc traffic website đi qua Cloudflare Free để tránh lộ IP gốc.

## Tài liệu

- [Documentation index](docs/README.md): danh mục gọn, link tới tài liệu Việt
  và English mirror.
- [Setup guide](docs/SETUP.md): hướng dẫn cài Management Server, Client Node,
  XDP và Provider API.
- [Setup Master](docs/SETUP_MASTER.md): cài `firewall.vpsgen.com`, Nginx,
  systemd, SQLite license DB và Admin/API.
- [Setup Client](docs/SETUP_CLIENT.md): cài WAF reverse proxy, heartbeat license
  và XDP object trên server được bảo vệ.
- [Project structure](docs/PROJECT_STRUCTURE.md): cấu trúc thư mục và ranh giới
  master/client.
- [Phase roadmap](PHASES.md): phase/task đã hoàn thành, bằng chứng test và phần
  còn thiếu.
- [Production gap analysis](docs/vi/40-production-gap-analysis.md): đánh giá
  phần đã xong/chưa xong trước khi claim production cho doanh nghiệp/trường học.
- [VPS Ubuntu 22.04 test runbook](docs/vi/41-vps-ubuntu-2204-test-runbook.md):
  upload, build, smoke và benchmark an toàn trên VPS.
