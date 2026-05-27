# Gói Triển Khai Và Cộng Đồng

## Đối tượng

`kiro_waf` nên phục vụ tốt:

- Trường học.
- Doanh nghiệp nhỏ.
- Website nội bộ.
- VPS cá nhân cần bảo vệ tốt hơn mặc định.
- Nhà cung cấp hosting nhỏ.
- Đội kỹ thuật không có chuyên gia kernel/security riêng.

## Gói đề xuất

### Community

Miễn phí hoặc open-core.

- Server mode cơ bản.
- nftables template.
- Basic XDP blocklist nếu có.
- CLI local.
- Tài liệu tiếng Việt.
- Không có provider support SLA.

Mục tiêu: xây cộng đồng, nhận feedback, tạo niềm tin.

### School/SMB

Gói trả phí thấp, dễ setup.

- Full mode.
- Cloudflare Free wizard.
- Nginx config generator.
- Bot scoring cơ bản.
- WAF OWASP CRS.
- Update security.
- Support bundle.
- Hỗ trợ cài đặt.

Mục tiêu: trường học và doanh nghiệp nhỏ có thể dùng mà không cần đội bảo mật
riêng.

### Professional

Cho website/API quan trọng hơn.

- Runtime security nâng cao.
- Policy riêng theo domain.
- Route quota nâng cao.
- Incident report.
- Priority update.
- Hỗ trợ rebind/license nhanh.

### Enterprise-lite

Không phải enterprise toàn cầu, nhưng đủ cho tổ chức vừa.

- Dashboard local.
- Audit log.
- Staged update.
- Multi-admin local.
- Backup/restore config.
- Báo cáo định kỳ.
- Quy trình bảo hành rõ.

## Tiêu chí cộng đồng

Nếu muốn hệ thống phát triển lâu dài, cần:

- Tài liệu cài đặt rõ.
- Changelog minh bạch.
- Security policy.
- Cách report lỗ hổng.
- Không thu thập dữ liệu quá mức.
- Có bản community dùng được thật.
- Có benchmark công khai.
- Có ví dụ config theo tình huống thực tế.
- Có hướng dẫn gỡ cài đặt sạch.

## Chính sách privacy tối thiểu

Mặc định không gửi:

- Request body.
- Cookie.
- Authorization header.
- Password/token/session.
- Nội dung file ứng dụng.

Có thể gửi nếu bật health report:

- Server ID.
- Version.
- Mode.
- Defense level.
- CPU/RAM/load.
- Counters tổng hợp.
- Số lượng block/challenge/WAF event.

IP client nên hash hoặc rút gọn nếu không cần hỗ trợ incident chi tiết.

## Setup dễ dùng

Installer nên hỏi ít câu:

```text
1. Chọn mode: server/full.
2. Nhập admin IP cho SSH.
3. Nếu full: nhập domain và backend URL.
4. Nếu dùng Cloudflare: bật origin lock.
5. Nhập license key hoặc chọn community.
6. Dry-run.
7. Apply.
8. Health check.
```

Không nên bắt người dùng mới hiểu eBPF, nftables, Nginx trước khi dùng được.

