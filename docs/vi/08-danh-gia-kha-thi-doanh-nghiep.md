# Đánh Giá Khả Thi Doanh Nghiệp

## Kết luận thẳng

Thiết kế hiện tại khả thi để phát triển thành sản phẩm cho trường học, doanh
nghiệp nhỏ và máy chủ đơn. Tuy nhiên, trạng thái hiện tại mới là blueprint,
config mẫu và deployment mẫu. Nó chưa thể được coi là hệ thống chống tấn công
đã hoạt động tốt thật sự cho khách hàng doanh nghiệp cho đến khi có:

- `kiro-agent` thật.
- eBPF/XDP program thật.
- nftables manager có rollback an toàn.
- Nginx/HAProxy config generator.
- WAF integration thật.
- Bộ kiểm thử tải, kiểm thử attack và benchmark.
- Quy trình update ký số được test.
- Quy trình support, backup, incident response.

Nói ngắn gọn: hướng kiến trúc đúng, nhưng chưa đủ để bán như sản phẩm hoàn
chỉnh nếu chưa triển khai và kiểm chứng bằng số liệu.

## Điểm mạnh của hướng hiện tại

- Phù hợp máy chủ đơn, không bắt khách hàng mua cụm phức tạp.
- Không dùng SQL trong MVP nên dễ setup, dễ backup, ít lỗi vận hành.
- Có mode `server` và `full` rõ ràng.
- Có Cloudflare Free để giảm bot rác website và giảm nguy cơ lộ IP gốc.
- Có licensing theo fingerprint máy.
- Có update manifest ký số.
- Có domain/backend mapping đủ linh hoạt cho nhiều mô hình website.
- Có resource governor để tránh server bị kéo chết khi tải tăng.

## Điểm chưa đủ cấp doanh nghiệp

### 1. Chưa có chứng minh hiệu năng

Cần số liệu:

- XDP drop được bao nhiêu PPS trên từng loại VPS.
- nftables chịu được bao nhiêu dynamic block.
- Nginx chịu được bao nhiêu request/giây ở từng profile.
- WAF bật CRS làm tăng latency bao nhiêu.
- Bot challenge làm giảm bao nhiêu request vào backend.
- Khi attack dừng, hệ thống hạ mode có ổn định không.

Không có benchmark thì không nên quảng cáo “chống DDoS mạnh”.

### 2. File-based provider storage chỉ phù hợp giai đoạn đầu

Không dùng SQL là đúng cho MVP. Nhưng nếu nhà cung cấp quản lý nhiều khách hàng,
phải có giới hạn rõ:

```text
File-based ổn cho: 1-200 server
Cần cân nhắc database khi: nhiều người vận hành cùng lúc, hàng nghìn server,
cần query/report phức tạp, cần phân quyền nhiều nhân viên.
```

Agent trên server khách hàng vẫn nên giữ file-based lâu dài. Provider backend
có thể nâng cấp sau mà không ảnh hưởng agent.

### 3. Cloudflare Free không phải cam kết chống DDoS đầy đủ

Cloudflare Free hữu ích cho website, nhưng không nên mô tả như lớp chống mọi
DDoS. Cần viết rõ:

- Chỉ bảo vệ traffic đi qua Cloudflare.
- Phải origin-lock.
- Không bảo vệ SSH/game/custom TCP port.
- Nếu IP gốc bị lộ và bị đánh đầy băng thông, server vẫn bị ảnh hưởng.

### 4. WAF không thay thế bảo mật ứng dụng

WAF chặn được nhiều payload phổ biến, nhưng không sửa lỗi logic ứng dụng:

- Broken access control.
- IDOR.
- Business logic abuse.
- Lộ dữ liệu do phân quyền sai.
- Lỗi auth/session.

Vì vậy sản phẩm nên nói là “giảm rủi ro và phát hiện/chặn nhiều lớp”, không
nên nói “chặn mọi tấn công”.

### 5. Thiếu chính sách an toàn khi apply firewall

Sản phẩm thương mại bắt buộc phải có:

- Dry-run config.
- SSH safety check.
- Rollback timer.
- Last known good config.
- Console recovery guide.
- Không apply firewall nếu admin IP chưa hợp lệ.

### 6. Thiếu privacy/data policy

Trường học và doanh nghiệp nhỏ cần biết hệ thống gửi gì về provider:

- Health metrics nào được gửi.
- Có gửi log request không.
- Có gửi IP client không.
- Dữ liệu lưu bao lâu.
- Cách tắt telemetry.
- Cách xuất support bundle đã ẩn secret.

## Định vị sản phẩm hợp lý

Không nên định vị là “thay thế CDN/scrubbing/enterprise WAF toàn cầu”.

Nên định vị:

```text
Kiro WAF là hệ thống bảo vệ máy chủ đơn cho website, API và dịch vụ server,
giúp giảm DDoS vừa/nhỏ, bot rác, khai thác web phổ biến, quá tải tài nguyên và
dấu hiệu bị xâm nhập, phù hợp trường học, SME, VPS, server nội bộ và nhà cung
cấp hosting nhỏ.
```

## Điều kiện để gọi là bản production

Một bản production tối thiểu cần đạt:

- Cài được bằng một lệnh hoặc wizard.
- Không làm mất SSH khi apply firewall.
- Rollback tự động khi config lỗi.
- Có test profile `server` và `full`.
- Có dashboard/CLI rõ ràng.
- Có update ký số.
- Có support bundle.
- Có benchmark công khai.
- Có tài liệu giới hạn sản phẩm.
- Có chính sách privacy.

