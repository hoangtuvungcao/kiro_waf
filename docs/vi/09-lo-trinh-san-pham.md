# Lộ Trình Sản Phẩm

## Mục tiêu

Đưa `kiro_waf` từ blueprint thành sản phẩm có thể bán, hỗ trợ và phát triển lâu
dài cho trường học, doanh nghiệp nhỏ và nhà cung cấp server.

## Giai đoạn 0: Blueprint sạch

Trạng thái hiện tại.

Cần có:

- Tài liệu tiếng Việt và tiếng Anh.
- Config mẫu.
- Domain/backend mapping.
- License format.
- Update manifest.
- Deployment mẫu.
- Giới hạn sản phẩm được viết rõ.

## Giai đoạn 1: Agent server mode

Mục tiêu: bảo vệ server không website.

Phải làm:

- `kiro-agent` đọc config.
- Verify license.
- Generate/apply nftables.
- Safe rollback firewall.
- Basic XDP blocklist/allowlist.
- Metrics local.
- `kiro status`, `kiro health`, `kiro mode`.

Tiêu chí đạt:

- Cài được trên Ubuntu 22.04 LTS.
- Không khóa mất SSH.
- Block/unblock IP hoạt động.
- Restart agent không mất state.

## Giai đoạn 2: Full mode website

Mục tiêu: bảo vệ website/API.

Phải làm:

- Generate Nginx config từ `sites` và `backend_pools`.
- Route quota.
- Static cache.
- Basic bot scoring.
- Cookie challenge.
- WAF integration với Coraza hoặc ModSecurity.
- Cloudflare origin lock.
- Real IP restore an toàn.

Tiêu chí đạt:

- 1 domain nhiều backend hoạt động.
- Nhiều domain một backend hoạt động.
- Một domain nhiều route/backend hoạt động.
- Direct origin HTTP/HTTPS bị chặn khi bật Cloudflare.

## Giai đoạn 3: Overload control

Mục tiêu: server tự giảm tải khi attack hoặc khi app yếu.

Phải làm:

- Baseline traffic.
- Hysteresis để tránh nhảy mode liên tục.
- `NORMAL`, `ELEVATED`, `ATTACK`, `LOCKDOWN`.
- Cooldown khi attack dừng.
- Bảo vệ log không bị flood.
- Circuit breaker cho backend.

Tiêu chí đạt:

- Backend chậm thì proxy trả 429/503 sớm.
- Attack dừng thì hệ thống tự hạ mode.
- Không restart loop.

## Giai đoạn 4: Provider management

Mục tiêu: nhà cung cấp quản lý khách hàng, key và bảo hành.

Phải làm:

- File-based provider storage.
- Issue license.
- Rebind license.
- Revoke license.
- Health report.
- Support bundle.
- Signed update.

Tiêu chí đạt:

- Kích hoạt online/offline.
- License hết hạn vào grace mode đúng.
- Update fail thì rollback.

## Giai đoạn 5: Enterprise-lite

Mục tiêu: đủ ổn định cho SMB/trường học.

Phải làm:

- Dashboard local.
- Installer wizard.
- Policy template theo môi trường: trường học, SME, hosting nhỏ.
- Privacy controls.
- Role-based local admin tối thiểu.
- Audit log không sửa được dễ dàng.
- Backup/restore config.

Tiêu chí đạt:

- Người không chuyên kernel vẫn setup được.
- Có tài liệu xử lý sự cố.
- Có benchmark theo size server.

## Giai đoạn 6: Mở rộng sau này

Khi có nhiều khách hàng:

- Provider backend có thể chuyển từ file sang database.
- Agent vẫn giữ file-based.
- Có multi-operator provider console.
- Có staged rollout.
- Có marketplace rule/policy.
- Có plugin cho hosting panel.

## Giai đoạn 7: Production/commercial gate

Mục tiêu: không chỉ chạy được trong lab, mà đủ điều kiện pilot và bán có kiểm
soát.

Phải làm:

- Pass production gate trong `21-production-va-thuong-mai-readiness.md`.
- Có benchmark.
- Có pilot 30 ngày.
- Có security policy.
- Có privacy policy.
- Có release process ký số.
- Có runbook support.
- Có terms/limitation statement.

Tiêu chí đạt:

- Không còn lỗi khóa SSH/firewall nghiêm trọng.
- Update/rollback chạy ổn.
- Support xử lý được incident bằng bundle.
- Tài liệu bán hàng không quảng cáo vượt khả năng kỹ thuật.
