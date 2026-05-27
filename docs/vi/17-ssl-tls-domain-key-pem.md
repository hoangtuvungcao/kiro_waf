# SSL/TLS Domain, Key Và PEM

## Kết luận

Bạn đúng: nếu dùng Cloudflare Free với SSL/TLS Flexible thì origin server có thể
chỉ chạy HTTP port 80, không cần cấu hình `.pem` và private key cho từng domain.
Đây là cách setup đơn giản nhất cho trường học, website nhỏ hoặc người dùng
không rành kỹ thuật.

Tuy nhiên cần ghi rõ: Flexible chỉ mã hóa từ trình duyệt đến Cloudflare. Đoạn
Cloudflare đến origin server là HTTP.

## Ba chế độ origin TLS

### 1. flexible_http

```text
Browser --HTTPS--> Cloudflare --HTTP:80--> Origin
```

Config:

```yaml
website_protection:
  cloudflare:
    enabled: true
    require_proxied_traffic: true
    block_direct_origin_http: true
  tls:
    origin_mode: flexible_http
    origin_http_port: 80
    certificate_file: ""
    private_key_file: ""
```

Ưu điểm:

- Dễ setup nhất.
- Không cần cert/key/pem trên server.
- Không cần xử lý renew certificate.
- Phù hợp website thông tin, landing page, trường học nhỏ, site ít dữ liệu nhạy
  cảm.

Nhược điểm:

- Origin traffic không mã hóa.
- Không nên dùng cho hệ thống có login quan trọng, dữ liệu cá nhân nhạy cảm,
  thanh toán, hồ sơ học sinh/nhân sự hoặc yêu cầu compliance.
- Nếu app tự redirect HTTP sang HTTPS ở origin có thể gây redirect loop.

### 2. full_tls

```text
Browser --HTTPS--> Cloudflare --HTTPS:443--> Origin
```

Cloudflare kết nối HTTPS tới origin, nhưng không strict verify certificate như
`full_strict`.

Config:

```yaml
website_protection:
  tls:
    origin_mode: full_tls
    origin_https_port: 443
    certificate_file: /etc/kiro/certs/example.com.pem
    private_key_file: /etc/kiro/certs/example.com.key
```

Phù hợp khi muốn mã hóa origin nhanh nhưng chưa sẵn sàng strict validation.

### 3. full_strict

```text
Browser --HTTPS--> Cloudflare --HTTPS:443 + cert verify--> Origin
```

Đây là mode khuyến nghị cho production.

Config:

```yaml
website_protection:
  tls:
    origin_mode: full_strict
    origin_https_port: 443
    certificate_file: /etc/kiro/certs/example.com.pem
    private_key_file: /etc/kiro/certs/example.com.key
    cloudflare_origin_ca: true
```

Certificate có thể là:

- Cloudflare Origin CA certificate.
- Let's Encrypt.
- Certificate hợp lệ từ CA công khai khác.

## Mặc định đề xuất cho sản phẩm

### Community / School SMB

Mặc định:

```yaml
origin_mode: flexible_http
```

Lý do:

- Dễ cài.
- Ít lỗi.
- Không cần cert/key.
- Người dùng không chuyên vẫn dùng được.

Giao diện/CLI phải cảnh báo rõ nếu site có login hoặc dữ liệu nhạy cảm.

### Professional / Enterprise-lite

Mặc định:

```yaml
origin_mode: full_strict
```

Lý do:

- Bảo mật đường truyền end-to-end.
- Phù hợp hệ thống có dữ liệu người dùng.
- Giảm rủi ro bị nghe lén/sửa traffic giữa Cloudflare và origin.

## Origin lock vẫn bắt buộc

Dù dùng `flexible_http` hay `full_strict`, nếu bật Cloudflare thì origin phải
khóa direct traffic:

```text
80/443 chỉ allow Cloudflare IP ranges
SSH chỉ allow admin IP
drop direct origin HTTP/HTTPS từ nguồn khác
```

Nếu không khóa origin, attacker có thể bỏ qua Cloudflare và đánh thẳng IP gốc.

## Nginx trong flexible_http

Mẫu đơn giản:

```nginx
server {
    listen 80;
    server_name example.com www.example.com;

    include /etc/nginx/kiro/cloudflare-real-ip.conf;
    real_ip_header CF-Connecting-IP;

    location / {
        proxy_pass http://frontend_pool;
    }
}
```

Không cần:

```nginx
ssl_certificate
ssl_certificate_key
```

## Nginx trong full_strict

Mẫu:

```nginx
server {
    listen 443 ssl;
    server_name example.com www.example.com;

    ssl_certificate /etc/kiro/certs/example.com.pem;
    ssl_certificate_key /etc/kiro/certs/example.com.key;

    include /etc/nginx/kiro/cloudflare-real-ip.conf;
    real_ip_header CF-Connecting-IP;

    location / {
        proxy_pass http://frontend_pool;
    }
}
```

## Validation của agent

Agent phải kiểm tra:

- `flexible_http`: không yêu cầu cert/key.
- `full_tls`: yêu cầu cert/key tồn tại.
- `full_strict`: yêu cầu cert/key tồn tại và cảnh báo nếu không phải Origin CA
  hoặc cert hợp lệ.
- Nếu bật Cloudflare thì phải có origin lock.
- Nếu dùng Flexible và app có route login/admin, hiển thị cảnh báo bảo mật.

## Nguồn tham khảo

- Cloudflare Flexible mode: https://developers.cloudflare.com/ssl/origin-configuration/ssl-modes/flexible/
- Cloudflare SSL/TLS get started: https://developers.cloudflare.com/ssl/get-started/
- Cloudflare Origin CA: https://developers.cloudflare.com/ssl/origin-configuration/origin-ca

