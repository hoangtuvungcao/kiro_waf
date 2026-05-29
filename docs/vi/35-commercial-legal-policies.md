# Chính Sách Thương Mại Và Pháp Lý

Tài liệu này là bản khung để đạt commercial gate. Trước khi bán rộng, nội dung
cần được provider rà soát pháp lý theo quốc gia/khu vực kinh doanh.

## Gói Dịch Vụ

| Gói | Mode | Server/domain | Feature chính | Support | Update | Rebind |
| --- | --- | --- | --- | --- | --- | --- |
| Community | `server` | 1 server, không SLA | nftables dry-run, CLI local, tài liệu | Community/best effort | Manual/stable | Không cam kết |
| School/SMB | `server`, `full` | 1-3 server, 1-10 domain | Nginx generator, WAF, bot, Cloudflare origin lock, signed updates | Business hours | Stable/security | 1 lần/tháng theo xác minh |
| Professional | `server`, `full` | 1-10 server, nhiều domain | Runtime security, incident report, priority update, route policy | Priority business hours | Stable/security/beta tùy chọn | Nhanh hơn, cần audit reason |
| Enterprise-lite | `server`, `full` | Theo hợp đồng | Audit log, staged rollout, báo cáo định kỳ, quy trình bảo hành riêng | Theo hợp đồng | Stable/security/staged | Theo hợp đồng |

## SLA/SLO Thực Tế

Không cam kết chống mọi DDoS. Chỉ cam kết các phần provider kiểm soát được:

- License activation khi provider online: mục tiêu dưới 5 phút.
- Security update delivery: 24-72 giờ tùy severity và mức ảnh hưởng.
- Rollback config/update trong lab: mục tiêu dưới 30 giây sau khi bắt đầu lệnh.
- Support bundle local: mục tiêu dưới 60 giây với cấu hình bình thường.
- Incident report template: tạo local trong dưới 60 giây.

Support response tham khảo:

| Gói | Phản hồi mục tiêu |
| --- | --- |
| Community | Không có SLA |
| School/SMB | 1-2 ngày làm việc |
| Professional | 4-8 giờ làm việc |
| Enterprise-lite | Theo hợp đồng |

## Giới Hạn Sản Phẩm

- Không thay thế upstream bandwidth DDoS protection.
- Không đảm bảo chặn mọi botnet, mọi tầng mạng hoặc mọi kiểu tấn công.
- Cloudflare Free chỉ bảo vệ traffic đi qua Cloudflare và cấu hình DNS/proxy đúng.
- Flexible HTTP không mã hóa đoạn Cloudflare tới origin.
- WAF không sửa lỗi logic ứng dụng, lỗi auth hoặc lỗi business workflow.
- Hiệu quả phụ thuộc tài nguyên server, kernel, cấu hình, traffic profile và loại tấn công.
- Benchmark local/lab không được dùng làm claim public nếu chưa có lab cô lập và phương pháp đo rõ.

## Privacy

Mặc định:

- Telemetry tắt.
- Không gửi request body.
- Không gửi cookie.
- Không gửi Authorization header.
- Không gửi token/password/license key.
- Support bundle redact secret trước khi ghi file.

Health/support data có thể gồm:

- Version, mode, plan.
- Trạng thái module.
- CPU/RAM/load/counter tổng hợp.
- Health/preflight status.
- Runtime alert đã redact.
- Incident timeline do operator nhập.

Retention khuyến nghị:

| Dữ liệu | Retention mặc định |
| --- | --- |
| Health report | 180 ngày |
| Incident report | 365 ngày |
| Support bundle | Xóa sau khi ticket đóng hoặc theo hợp đồng |
| License audit | Theo yêu cầu kế toán/pháp lý |

Khách hàng có quyền yêu cầu xóa support bundle và incident attachment không còn
cần thiết cho bảo hành, trừ khi provider buộc phải giữ theo nghĩa vụ pháp lý.

## Data Processing Note

Nếu bật telemetry hoặc gửi support bundle cho provider:

- Provider chỉ xử lý dữ liệu để hỗ trợ, bảo hành, phát hành update và điều tra incident.
- Provider không bán request/client data.
- Provider không yêu cầu request body/cookie/token để support thông thường.
- Dữ liệu nhạy cảm phải được redact trước khi gửi qua kênh support.
- Nếu cần dữ liệu chưa redact trong sự cố nghiêm trọng, phải có xác nhận riêng của khách hàng.

## Security Vulnerability Policy

Kênh report:

```text
security@example.com
```

Nội dung report nên có:

- Phiên bản bị ảnh hưởng.
- Mô tả lỗ hổng.
- Bước tái hiện tối thiểu.
- Impact thực tế.
- Log/screenshot đã redact.

Severity:

| Severity | Ví dụ | Phản hồi mục tiêu |
| --- | --- | --- |
| Critical | RCE, private key leak, bypass update signature | 24 giờ |
| High | Privilege escalation, auth bypass, serious data exposure | 2 ngày làm việc |
| Medium | Local DoS, limited info leak, policy bypass có điều kiện | 5 ngày làm việc |
| Low | Hardening issue, minor disclosure | 10 ngày làm việc |

Provider không public exploit detail trước khi có bản vá hoặc mitigation hợp lý.

## Acceptable Use Policy

Không được dùng `kiro_waf`, benchmark hoặc tooling liên quan để:

- Tấn công hệ thống không sở hữu hoặc không được phép kiểm thử.
- Tạo hoặc điều phối traffic DDoS public.
- Né rate limit của bên thứ ba.
- Thu thập dữ liệu trái phép.
- Che giấu hoạt động malware/phishing/spam.

Provider có quyền từ chối support hoặc thu hồi service nếu khách hàng vi phạm AUP.

## Terms Of Service Khung

- Khách hàng chịu trách nhiệm cung cấp admin IP/domain/backend/license thông tin chính xác.
- Khách hàng phải có quyền hợp pháp với server/domain cần bảo vệ.
- Provider không chịu trách nhiệm cho downtime do cấu hình sai ngoài quy trình đã khuyến nghị.
- Các lệnh apply thật cần preflight, dry-run, rollback và xác nhận operator.
- Provider không cam kết loại bỏ hoàn toàn DDoS hoặc vulnerability của ứng dụng gốc.

## Hoàn Tiền Và Bảo Hành

Khuyến nghị chính sách tối thiểu:

- Hoàn tiền trong giai đoạn onboarding nếu không cài được do lỗi sản phẩm và support không khắc phục được.
- Không hoàn tiền cho server không đáp ứng điều kiện hệ thống đã công bố.
- Không bảo hành khi khách hàng tự sửa firewall/proxy ngoài runbook rồi gây mất truy cập.
- Update lỗi do provider phát hành phải có rollback/mitigation và incident note.
- Rebind license cần audit reason; lạm dụng rebind có thể bị từ chối.

## Checklist Commercial-Ready

- Service plan công khai.
- SLA/SLO công khai và không phóng đại.
- Privacy statement công khai.
- Security report contact công khai.
- AUP/ToS khung đã được duyệt.
- DDoS limitation disclaimer nằm trong tài liệu bán hàng.
- Refund/warranty policy rõ ràng.
