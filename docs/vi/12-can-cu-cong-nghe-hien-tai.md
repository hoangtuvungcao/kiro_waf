# Căn Cứ Công Nghệ Hiện Tại

Tài liệu này ghi lại các quyết định công nghệ chính để tránh thiết kế theo cảm
tính.

## Ubuntu 22.04 LTS

Ubuntu 22.04 LTS vẫn phù hợp cho sản phẩm trong giai đoạn hiện tại vì còn
standard security maintenance đến tháng 05/2027 và có thể kéo dài bằng Ubuntu
Pro/ESM. Tuy nhiên roadmap sản phẩm phải chuẩn bị hỗ trợ Ubuntu 24.04 LTS.

Nguồn tham khảo: https://ubuntu.com/about/release-cycle

## Cloudflare Free

Cloudflare DDoS Protection có trên tất cả plan, nhưng khi dùng bản Free cho
website phải hiểu đúng giới hạn:

- Chỉ bảo vệ traffic đi qua Cloudflare.
- Origin phải khóa 80/443 chỉ cho Cloudflare IP.
- Phải cập nhật cả IPv4 và IPv6 Cloudflare ranges.
- Không bảo vệ SSH, game port hoặc custom TCP/UDP nếu traffic không đi qua
  Cloudflare.

Nguồn tham khảo:

- https://developers.cloudflare.com/ddos-protection/
- https://www.cloudflare.com/ips/

## WAF open-source

Coraza và OWASP CRS là hướng hợp lý cho WAF local:

- Coraza hỗ trợ ModSecurity SecLang và tương thích OWASP CRS.
- OWASP CRS là bộ rule phát hiện tấn công phổ biến cho ModSecurity hoặc WAF
  tương thích.

Nguồn tham khảo:

- https://owasp.org/www-project-coraza-web-application-firewall/
- https://owasp.org/www-project-modsecurity-core-rule-set/

## Tiêu chuẩn quản trị bảo mật

Để sản phẩm có hướng doanh nghiệp, nên dùng NIST CSF 2.0 làm khung quản trị:

```text
Govern
Identify
Protect
Detect
Respond
Recover
```

Với bảo mật web/app, nên dùng OWASP ASVS để định nghĩa yêu cầu kiểm thử kỹ thuật.

Nguồn tham khảo:

- https://www.nist.gov/publications/nist-cybersecurity-framework-csf-20
- https://owasp.org/www-project-application-security-verification-standard/

