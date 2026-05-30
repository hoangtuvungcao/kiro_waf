package handlers

import "html/template"

var viConfigReference template.HTML = `<h2>Tham Chiếu Cấu Hình</h2>
<p>Tất cả cấu hình được lưu trong <code>/etc/kiro/kiro.yaml</code>. Dưới đây là tham chiếu đầy đủ các tùy chọn.</p>

<h3>Tùy Chọn Cấp Cao Nhất</h3>
<table>
<tr><th>Tùy Chọn</th><th>Kiểu</th><th>Mặc Định</th><th>Phạm Vi/Giá Trị</th><th>Mô Tả</th></tr>
<tr><td><code>mode</code></td><td>string</td><td>full</td><td>server, full</td><td>Chế độ hoạt động. "server" chỉ tường lửa, "full" tường lửa + bảo vệ web</td></tr>
<tr><td><code>plan</code></td><td>string</td><td>-</td><td>community, school_smb, professional, enterprise_lite</td><td>Gói license xác định tính năng khả dụng</td></tr>
<tr><td><code>license_key</code></td><td>string</td><td>-</td><td>Định dạng: KIRO-XXXX-XXXX</td><td>License key Kiro WAF để xác thực</td></tr>
</table>

<h3>Phần Admin</h3>
<table>
<tr><th>Tùy Chọn</th><th>Kiểu</th><th>Mặc Định</th><th>Phạm Vi/Giá Trị</th><th>Mô Tả</th></tr>
<tr><td><code>admin.allow_ips</code></td><td>[]string</td><td>[]</td><td>Ký hiệu CIDR</td><td>Địa chỉ IP được phép truy cập admin/SSH</td></tr>
</table>
<p><strong>Ví dụ:</strong></p>
<pre><code>admin:
  allow_ips:
    - 203.0.113.10/32
    - 10.0.0.0/8</code></pre>

<h3>Phần Server</h3>
<table>
<tr><th>Tùy Chọn</th><th>Kiểu</th><th>Mặc Định</th><th>Phạm Vi/Giá Trị</th><th>Mô Tả</th></tr>
<tr><td><code>server.interface</code></td><td>string</td><td>eth0</td><td>Tên giao diện mạng</td><td>Giao diện mạng chính cho lọc gói tin</td></tr>
<tr><td><code>server.ssh_port</code></td><td>integer</td><td>22</td><td>1-65535</td><td>Cổng SSH giữ mở trong quy tắc tường lửa</td></tr>
</table>

<h3>Phần Website</h3>
<table>
<tr><th>Tùy Chọn</th><th>Kiểu</th><th>Mặc Định</th><th>Phạm Vi/Giá Trị</th><th>Mô Tả</th></tr>
<tr><td><code>website.enabled</code></td><td>boolean</td><td>true</td><td>true, false</td><td>Bật bảo vệ ứng dụng web</td></tr>
<tr><td><code>website.cloudflare</code></td><td>boolean</td><td>true</td><td>true, false</td><td>Bật tích hợp Cloudflare để khôi phục IP thực</td></tr>
<tr><td><code>website.tls_mode</code></td><td>string</td><td>flexible_http</td><td>flexible_http, full_tls, full_strict</td><td>Chế độ TLS giữa Cloudflare và origin server</td></tr>
</table>

<h3>Website Sites</h3>
<table>
<tr><th>Tùy Chọn</th><th>Kiểu</th><th>Mặc Định</th><th>Phạm Vi/Giá Trị</th><th>Mô Tả</th></tr>
<tr><td><code>website.sites[].domains</code></td><td>[]string</td><td>-</td><td>Tên miền hợp lệ</td><td>Danh sách domain mà site phản hồi</td></tr>
<tr><td><code>website.sites[].backend</code></td><td>string</td><td>-</td><td>Định dạng URL</td><td>URL backend server để proxy request đến</td></tr>
<tr><td><code>website.sites[].routes[].path</code></td><td>string</td><td>/</td><td>Tiền tố đường dẫn URL</td><td>Tiền tố đường dẫn URL để khớp route</td></tr>
<tr><td><code>website.sites[].routes[].backend</code></td><td>string</td><td>-</td><td>Định dạng URL</td><td>Ghi đè backend cho route cụ thể này</td></tr>
<tr><td><code>website.sites[].routes[].protection</code></td><td>string</td><td>balanced</td><td>light, balanced, strict</td><td>Mức bảo vệ cho route này</td></tr>
</table>
<p><strong>Ví dụ:</strong></p>
<pre><code>website:
  sites:
    - domains:
        - example.com
        - www.example.com
      backend: http://127.0.0.1:3000
      routes:
        - path: /api/
          backend: http://127.0.0.1:4000
        - path: /login
          protection: strict</code></pre>

<h3>Phần Protection</h3>
<table>
<tr><th>Tùy Chọn</th><th>Kiểu</th><th>Mặc Định</th><th>Phạm Vi/Giá Trị</th><th>Mô Tả</th></tr>
<tr><td><code>protection.profile</code></td><td>string</td><td>balanced</td><td>light, balanced, strict, lockdown</td><td>Mức độ bảo vệ tổng thể</td></tr>
<tr><td><code>protection.waf</code></td><td>boolean</td><td>true</td><td>true, false</td><td>Bật quy tắc Web Application Firewall</td></tr>
<tr><td><code>protection.bot</code></td><td>boolean</td><td>true</td><td>true, false</td><td>Bật phát hiện bot và hệ thống challenge</td></tr>
<tr><td><code>protection.auto_attack_mode</code></td><td>boolean</td><td>true</td><td>true, false</td><td>Tự động nâng mức bảo vệ khi phát hiện tấn công</td></tr>
</table>

<h3>Phần Updates</h3>
<table>
<tr><th>Tùy Chọn</th><th>Kiểu</th><th>Mặc Định</th><th>Phạm Vi/Giá Trị</th><th>Mô Tả</th></tr>
<tr><td><code>updates.auto_security_updates</code></td><td>boolean</td><td>true</td><td>true, false</td><td>Bật cập nhật bảo mật OTA tự động</td></tr>
<tr><td><code>updates.channel</code></td><td>string</td><td>stable</td><td>stable, beta</td><td>Kênh cập nhật theo dõi</td></tr>
</table>

<h3>Phần Telemetry</h3>
<table>
<tr><th>Tùy Chọn</th><th>Kiểu</th><th>Mặc Định</th><th>Phạm Vi/Giá Trị</th><th>Mô Tả</th></tr>
<tr><td><code>telemetry.enabled</code></td><td>boolean</td><td>false</td><td>true, false</td><td>Bật gửi báo cáo sức khỏe đến provider</td></tr>
</table>
`
