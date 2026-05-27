# PRD/SRS Sản Phẩm

## 1. Tên sản phẩm

`kiro_waf`

## 2. Đối tượng sử dụng

- Trường học.
- Doanh nghiệp nhỏ.
- Website/API chạy trên VPS hoặc server đơn.
- Nhà cung cấp hosting nhỏ.
- Đội kỹ thuật không có chuyên gia bảo mật riêng.

## 3. Vấn đề cần giải quyết

Khách hàng nhỏ thường gặp:

- Bị bot rác scan website.
- Bị HTTP flood nhẹ/vừa, mạnh.
- Bij tấn công đánh cắp dữ liệu và tấn công là máy chủ quá tải băng fbotnet hay các pp bypass 1 cách mạnh mẽ
- Bị tấn công vào `/login`, `/api`, `/upload`.
- Bị scan `.env`, `.git`, `wp-login.php`, `phpmyadmin`.
- Server quá tải vì conntrack, Nginx, app, database.
- Không biết cấu hình firewall/WAF đúng.
- Không có quy trình update/bảo hành rõ.
- Không muốn tự quản lý cert/key phức tạp.

## 4. Giá trị sản phẩm

`kiro_waf` cung cấp:

- Config tối giản cho người thuê.
- Bảo vệ nhiều lớp cho server và website.
- Cloudflare Free integration để giảm bot rác và ẩn IP gốc.
- License/key theo server.
- Update ký số.
- Support bundle để bảo hành dễ hơn.
- Runtime alert khi server có dấu hiệu bị khai thác.

## 5. Persona

### Người thuê bình thường

Muốn:

- Nhập domain/backend/license.
- Bật bảo vệ.
- Không đọc config 200 dòng.
- Không tự xử lý eBPF/nftables/Nginx phức tạp.

### Kỹ thuật hỗ trợ

Muốn:

- Xem health.
- Xuất support bundle.
- Chỉnh profile.
- Rollback khi lỗi.
- Debug attack.

### Nhà cung cấp

Muốn:

- Quản lý license.
- Quản lý server đã kích hoạt.
- Rebind/revoke.
- Phát hành update.
- Theo dõi incident và bảo hành.

## 6. Phạm vi MVP

### Must-have

- `kiro-agent`.
- `kiro-cli`.
- `kiro-provider` skeleton.
- Config tối giản và advanced.
- License verify.
- Provider issue license.
- File storage.
- nftables dry-run/apply/rollback.
- Nginx config generator.
- Cloudflare origin lock IPv4/IPv6.
- Flexible HTTP.
- Full Strict.
- Basic WAF integration.
- Basic bot score/challenge.
- Resource governor.
- Support bundle.
- Signed update manifest.

### Should-have

- Runtime process/file alert.
- Prometheus metrics.
- Local dashboard đơn giản.
- Policy template theo profile.
- Benchmark report.

### Could-have

- Provider web UI.
- Multi-admin.
- Staged rollout.
- Database provider tùy chọn.
- Plugin hosting panel.

### Won't-have trong MVP

- Multi-node cluster.
- Anycast/CDN riêng.
- ML phức tạp.
- Thay thế upstream DDoS scrubbing.
- Cam kết chống mọi loại DDoS.

## 7. Yêu cầu chức năng

### Agent

- Đọc simple config.
- Expand simple config thành runtime config.
- Validate config.
- Verify license.
- Apply firewall/proxy an toàn.
- Tự chuyển defense level.
- Ghi event.
- Tạo support bundle.
- Apply signed update.

### Provider

- Tạo customer/license/server record bằng file storage.
- Issue signed license.
- Rebind license.
- Revoke license.
- Publish signed update manifest.
- Nhận health report nếu bật telemetry.

### CLI

Các lệnh tối thiểu:

```text
kiro version
kiro config check
kiro status
kiro health
kiro mode show
kiro mode set server|full
kiro license show
kiro license activate
kiro support bundle
kiro update check
kiro rollback list
```

## 8. Yêu cầu phi chức năng

- Không khóa SSH.
- Rollback được.
- Không gửi dữ liệu nhạy cảm mặc định.
- Log có rate limit.
- Config dễ đọc.
- Chạy được trên Ubuntu 22.04 LTS.
- Chuẩn bị roadmap Ubuntu 24.04 LTS.
- Agent không cần database.
- Provider MVP không cần database.
- Có benchmark trước khi bán.

## 9. Acceptance Criteria

MVP được chấp nhận khi:

- `go test ./...` pass.
- Agent check được simple/advanced config.
- Provider issue license, agent verify được.
- Firewall dry-run pass.
- Firewall apply/rollback pass trong lab.
- Nginx generator pass cho 3 mô hình domain/backend.
- Flexible HTTP và Full Strict pass.
- WAF chặn payload phổ biến.
- Bot challenge hoạt động.
- Update sai chữ ký bị từ chối.
- Support bundle không lộ secret.

## 10. Rủi ro chính

- Lock mất SSH do firewall sai.
- False positive WAF làm hỏng website.
- Cloudflare origin lock sai IP range.
- Flexible HTTP bị dùng nhầm cho dữ liệu nhạy cảm.
- Provider private key bị lộ.
- Update lỗi làm agent crash.
- Log flood làm đầy disk.
- Quảng cáo quá mức so với khả năng thật.

## 11. Cách giảm rủi ro

- Dry-run + rollback timer.
- Admin allowlist bắt buộc.
- Last known good config.
- WAF exception theo route.
- Cloudflare ranges cập nhật và ký số.
- Private key chỉ ở provider.
- Signed update + rollback.
- Log rate limit.
- Tài liệu giới hạn sản phẩm rõ.

