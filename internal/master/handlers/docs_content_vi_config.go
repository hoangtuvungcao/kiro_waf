package handlers

import "html/template"

var viConfigReference template.HTML = `<h2>Tham Chiếu Cấu Hình</h2>
<p>Kiro WAF sử dụng hai loại cấu hình: <strong>biến môi trường</strong> cho binaries (<code>kiro-master</code>, <code>kiro-client-waf</code>) và <strong>file YAML</strong> cho <code>kiro-cli</code> và tính năng nâng cao.</p>

<h3>Biến Môi Trường Master Server</h3>
<p>File: <code>/etc/kiro-master/master.env</code></p>
<table>
<tr><th>Biến</th><th>Mặc Định</th><th>Bắt Buộc</th><th>Mô Tả</th></tr>
<tr><td><code>KIRO_MASTER_ADDR</code></td><td>:8080</td><td>Không</td><td>Địa chỉ lắng nghe (host:port)</td></tr>
<tr><td><code>KIRO_MASTER_DB</code></td><td>/var/lib/kiro-master/master.db</td><td>Không</td><td>Đường dẫn database SQLite</td></tr>
<tr><td><code>KIRO_MASTER_ADMIN_KEY</code></td><td>—</td><td><strong>Có</strong></td><td>Admin API key (fatal nếu trống)</td></tr>
<tr><td><code>KIRO_MASTER_ADMIN_IPS</code></td><td>(trống)</td><td>Không</td><td>Danh sách IP admin (phân cách bằng dấu phẩy)</td></tr>
<tr><td><code>KIRO_MASTER_SESSION_TTL</code></td><td>12h</td><td>Không</td><td>TTL phiên admin (định dạng Go duration)</td></tr>
</table>

<h3>Biến Môi Trường Client WAF</h3>
<p>File: <code>/etc/kiro/client-waf.env</code></p>
<table>
<tr><th>Biến</th><th>Mặc Định</th><th>Bắt Buộc</th><th>Mô Tả</th></tr>
<tr><td><code>KIRO_LICENSE_KEY</code></td><td>—</td><td><strong>Có</strong></td><td>License key (fatal nếu trống)</td></tr>
<tr><td><code>KIRO_CLIENT_COOKIE_SECRET</code></td><td>—</td><td><strong>Có</strong></td><td>HMAC cookie secret (fatal nếu trống)</td></tr>
<tr><td><code>KIRO_BACKEND_URL</code></td><td>—</td><td><strong>Có</strong></td><td>URL backend để proxy (fatal nếu trống)</td></tr>
<tr><td><code>KIRO_MASTER_URL</code></td><td>—</td><td><strong>Có</strong></td><td>URL master server cho heartbeat/updates (fatal nếu trống)</td></tr>
<tr><td><code>KIRO_CLIENT_LISTEN</code></td><td>:8090</td><td>Không</td><td>Địa chỉ lắng nghe WAF proxy</td></tr>
<tr><td><code>KIRO_NODE_ID</code></td><td>hostname</td><td>Không</td><td>ID node cho heartbeat</td></tr>
<tr><td><code>KIRO_POW_DIFFICULTY</code></td><td>4</td><td>Không</td><td>Độ khó Proof-of-Work (số leading zeros)</td></tr>
<tr><td><code>KIRO_HOLD_SECONDS</code></td><td>2</td><td>Không</td><td>Thời gian hold page (giây)</td></tr>
<tr><td><code>KIRO_RPM_PER_IP</code></td><td>120</td><td>Không</td><td>Request/phút mỗi IP (ngưỡng mềm)</td></tr>
<tr><td><code>KIRO_SUBNET_RPM</code></td><td>1800</td><td>Không</td><td>Request/phút mỗi /24 subnet</td></tr>
<tr><td><code>KIRO_HARD_BLOCK_AFTER</code></td><td>360</td><td>Không</td><td>Ngưỡng RPM để hard block</td></tr>
<tr><td><code>KIRO_BLOCK_TTL_SECONDS</code></td><td>900</td><td>Không</td><td>Thời gian ban (15 phút)</td></tr>
<tr><td><code>KIRO_XDP_BLOCKLIST_FILE</code></td><td>/var/lib/kiro/xdp-blocklist.txt</td><td>Không</td><td>Đường dẫn file blocklist XDP</td></tr>
<tr><td><code>KIRO_HEARTBEAT_SECONDS</code></td><td>60</td><td>Không</td><td>Khoảng cách heartbeat đến master</td></tr>
<tr><td><code>KIRO_UPDATE_SECONDS</code></td><td>300</td><td>Không</td><td>Khoảng cách kiểm tra cập nhật (5 phút)</td></tr>
<tr><td><code>KIRO_ADMIN_IPS</code></td><td>(trống)</td><td>Không</td><td>IP admin (phân cách bằng dấu phẩy, bypass lockdown)</td></tr>
</table>

<h3>Cấu Hình YAML (kiro.yaml)</h3>
<p>File YAML tại <code>/etc/kiro/kiro.yaml</code> được sử dụng bởi các lệnh <code>kiro-cli</code>.</p>

<h4>Tùy Chọn Cấp Cao Nhất</h4>
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
