# Tổng Quan

`kiro_waf` được thiết kế cho mô hình một máy chủ, phù hợp với nhà cung cấp dịch
vụ bảo mật server muốn cài đặt, quản lý, bảo hành và cập nhật hệ thống cho nhiều
khách hàng.

Mục tiêu của bản thiết kế này là giữ hệ thống gọn nhất có thể:

- Không dùng SQL trong MVP.
- Không cần cụm phức tạp.
- Không phụ thuộc CDN trả phí.
- Có thể dùng Cloudflare Free cho website để giảm bot rác và ẩn IP gốc.
- Mọi cấu hình quan trọng nằm trong file rõ ràng.
- License, update và policy được ký số để chống sửa trái phép.

## Giới hạn thực tế

Nếu tấn công DDoS làm đầy băng thông trước khi gói tin tới server, phần mềm trên
server không thể lấy lại băng thông đã mất. `kiro_waf` tập trung vào việc:

- Drop traffic xấu càng sớm càng tốt.
- Giảm tải CPU, RAM, conntrack, Nginx, app và database ứng dụng.
- Tự chuyển mức phòng thủ khi bị tấn công.
- Chống bot rác và request tốn tài nguyên.
- Phát hiện dấu hiệu server bị khai thác.

## Không dùng SQL trong MVP

Thay vì SQLite/PostgreSQL, hệ thống dùng file:

```text
/etc/kiro/kiro.yaml                    cấu hình máy đang bảo vệ
/etc/kiro/license.json                 license đã ký số
/etc/kiro/provider-public-key.pem      public key của nhà cung cấp
/var/lib/kiro/state/state.json         trạng thái local
/var/lib/kiro/events/events.jsonl      sự kiện local
/var/lib/kiro/blocks/active.json       danh sách block hiện tại
/var/lib/kiro/baseline/baseline.json   baseline traffic/tài nguyên
```

Phía nhà cung cấp cũng có thể dùng file:

```text
provider-data/customers/*.json
provider-data/licenses/*.json
provider-data/servers/*.json
provider-data/activations/*.jsonl
provider-data/health/*.jsonl
provider-data/updates/manifests/*.json
provider-data/revocations/revocations.json
```

Khi cần mở rộng cho hàng nghìn server, phần provider có thể chuyển sang
database sau. Agent trên server khách hàng vẫn không bắt buộc dùng SQL.

