# Cloudflare Free

## Mục tiêu

Trong `full` mode, Cloudflare Free có thể dùng làm lớp chắn đầu tiên cho website:

- Giảm bot rác cơ bản.
- Cache static asset.
- Ẩn IP gốc nếu DNS cấu hình đúng.
- Giảm request vào server.

Cloudflare Free không thay thế được scrubbing DDoS chuyên dụng, nhưng rất hữu ích
cho server đơn.

## Origin lock

Nếu bật Cloudflare, origin phải chỉ nhận HTTP/HTTPS từ Cloudflare:

```text
allow 80/443 từ Cloudflare IP ranges
drop 80/443 từ nguồn khác
allow SSH từ admin IP
```

Nếu không khóa origin, attacker biết IP gốc vẫn có thể đánh thẳng vào server.
Phải khóa cả IPv4 và IPv6. Nếu domain có AAAA record hoặc server có IPv6 public
nhưng firewall chỉ khóa IPv4, attacker có thể bypass qua IPv6.

## SSL/TLS dễ setup

Với khách hàng nhỏ hoặc trường học, mode dễ nhất là:

```text
Browser HTTPS -> Cloudflare -> HTTP port 80 origin
```

Trong `kiro_waf`, gọi là `flexible_http`.

Ưu điểm:

- Origin không cần file `.pem`.
- Origin không cần private key TLS.
- Nginx chỉ cần listen port 80.
- Cloudflare cấp HTTPS miễn phí phía người truy cập.
- Setup nhanh hơn cho người dùng bình thường.

Nhược điểm:

- Đường Cloudflare -> origin là HTTP, không mã hóa.
- Không phù hợp nếu site có dữ liệu nhạy cảm, login quan trọng, thanh toán hoặc
  yêu cầu compliance.
- Không dùng được Authenticated Origin Pull.

Vì vậy `flexible_http` nên là profile dễ dùng, còn production nghiêm túc nên
nâng lên `full_strict`.

## Kiểm tra lộ IP gốc

Agent nên cảnh báo khi:

- A/AAAA record trỏ thẳng về IP gốc.
- `www` proxied nhưng root domain không proxied.
- Subdomain cũ để lộ IP gốc.
- Mail/panel record dùng chung IP website.
- Truy cập trực tiếp `http://origin-ip` vẫn thành công.

## Real client IP

Chỉ tin `CF-Connecting-IP` khi remote address thuộc Cloudflare IP ranges. Nếu
không, client có thể tự fake header này.

Cloudflare ranges được lưu local:

```text
rules/cloudflare/ips-v4.txt
rules/cloudflare/ips-v6.txt
```
