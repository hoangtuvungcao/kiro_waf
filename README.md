# kiro_waf

`kiro_waf` là nền tảng bảo vệ một máy chủ Ubuntu 22.04 LTS theo hướng sản phẩm:
có cấu hình rõ ràng, có bản quyền/key theo máy, có cập nhật ký số, có chế độ
`server` và `full`, nhưng vẫn giữ kiến trúc đơn giản nhất có thể.

Với mô hình cho thuê, người thuê dùng config tối giản khoảng vài chục dòng.
Config nâng cao được giữ riêng cho provider/kỹ thuật.

MVP hiện tại không dùng SQL. Trạng thái hệ thống, license, key list, server list,
incident và update manifest được lưu bằng file `YAML`, `JSON`, `JSONL` có chữ ký
số khi cần. Khi sản phẩm lớn hơn mới cần nâng cấp sang database.

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

## Tài liệu tiếng Việt

- [Tổng quan](docs/vi/00-tong-quan.md)
- [Kiến trúc](docs/vi/01-kien-truc.md)
- [Chế độ vận hành](docs/vi/02-che-do-van-hanh.md)
- [Domain và backend](docs/vi/03-domain-backend.md)
- [Quản lý bản quyền/key list](docs/vi/04-ban-quyen-key-list.md)
- [Cloudflare Free](docs/vi/05-cloudflare-free.md)
- [Cập nhật và bảo hành](docs/vi/06-cap-nhat-bao-hanh.md)
- [Bảo mật hệ thống](docs/vi/07-bao-mat-he-thong.md)
- [Đánh giá khả thi doanh nghiệp](docs/vi/08-danh-gia-kha-thi-doanh-nghiep.md)
- [Lộ trình sản phẩm](docs/vi/09-lo-trinh-san-pham.md)
- [Kiểm thử và vận hành](docs/vi/10-kiem-thu-van-hanh.md)
- [Gói triển khai và cộng đồng](docs/vi/11-goi-trien-khai-cong-dong.md)
- [Căn cứ công nghệ hiện tại](docs/vi/12-can-cu-cong-nghe-hien-tai.md)
- [Threat model và yêu cầu hệ thống](docs/vi/13-threat-model-va-yeu-cau-he-thong.md)
- [Cấu trúc code và module](docs/vi/14-cau-truc-code-va-module.md)
- [Kế hoạch khởi tạo và test từng bước](docs/vi/15-ke-hoach-khoi-tao-va-test-tung-buoc.md)
- [Checklist sẵn sàng trước khi code](docs/vi/16-checklist-san-sang-truoc-khi-code.md)
- [SSL/TLS domain, key và pem](docs/vi/17-ssl-tls-domain-key-pem.md)
- [Phân tách provider server và server khách hàng](docs/vi/18-phan-tach-provider-va-server-khach-hang.md)
- [Cấu hình tối giản cho thuê](docs/vi/19-cau-hinh-toi-gian-cho-thue.md)
- [Readiness cuối trước khi code](docs/vi/20-readiness-cuoi-truoc-khi-code.md)
- [Production và thương mại readiness](docs/vi/21-production-va-thuong-mai-readiness.md)
- [PRD/SRS sản phẩm](docs/vi/22-prd-srs-san-pham.md)
- [Runbook lab firewall apply](docs/vi/23-firewall-apply-lab-runbook.md)
- [Runbook proxy generator](docs/vi/24-proxy-generator-runbook.md)

## English Docs

- [Overview](docs/en/00-overview.md)
- [Architecture](docs/en/01-architecture.md)
- [Operating modes](docs/en/02-operating-modes.md)
- [Domain and backend mapping](docs/en/03-domain-backend.md)
- [License and key list](docs/en/04-license-key-list.md)
- [Cloudflare Free](docs/en/05-cloudflare-free.md)
- [Updates and support](docs/en/06-updates-support.md)
- [System hardening](docs/en/07-system-hardening.md)
- [Enterprise readiness](docs/en/08-enterprise-readiness.md)
- [Current technology basis](docs/en/09-current-technology-basis.md)
- [Build and test plan](docs/en/10-build-and-test-plan.md)
- [SSL/TLS modes](docs/en/11-ssl-tls-modes.md)
- [Provider and protected server roles](docs/en/12-provider-and-protected-server-roles.md)
- [Minimal tenant configuration](docs/en/13-minimal-tenant-configuration.md)
- [Final readiness](docs/en/14-final-readiness.md)
- [Production and commercial readiness](docs/en/15-production-commercial-readiness.md)
- [Firewall apply lab runbook](docs/en/16-firewall-apply-lab-runbook.md)
- [Proxy generator runbook](docs/en/17-proxy-generator-runbook.md)
