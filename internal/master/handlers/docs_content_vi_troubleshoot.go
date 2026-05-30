package handlers

import "html/template"

var viTroubleshooting template.HTML = `<h2>Xử Lý Sự Cố</h2>
<p>Các vấn đề thường gặp và cách giải quyết khi vận hành Kiro WAF.</p>

<h3>1. Service Không Khởi Động: Thiếu License Key</h3>
<p><strong>Triệu chứng:</strong> Service thoát ngay sau khi khởi động với lỗi trong journal.</p>
<p><strong>Lỗi:</strong> <code>missing required config: license_key</code></p>
<p><strong>Giải pháp:</strong> Đảm bảo <code>license_key</code> được đặt trong <code>/etc/kiro/kiro.yaml</code>. Định dạng key là <code>KIRO-XXXX-XXXX</code>.</p>
<pre><code>sudo journalctl -u kiro-client-waf --no-pager -n 20
# Kiểm tra và sửa config:
sudo nano /etc/kiro/kiro.yaml
sudo systemctl restart kiro-client-waf</code></pre>

<h3>2. Service Không Khởi Động: Thiếu Backend URL</h3>
<p><strong>Triệu chứng:</strong> Service thoát với lỗi về cấu hình backend bị thiếu.</p>
<p><strong>Lỗi:</strong> <code>missing required config: backend_url</code></p>
<p><strong>Giải pháp:</strong> Cấu hình ít nhất một site với backend URL trong phần <code>website.sites</code>.</p>
<pre><code>website:
  sites:
    - domains:
        - yourdomain.com
      backend: http://127.0.0.1:3000</code></pre>

<h3>3. Lỗi 502 Bad Gateway</h3>
<p><strong>Triệu chứng:</strong> Khách truy cập thấy lỗi 502 khi truy cập website.</p>
<p><strong>Nguyên nhân:</strong> Backend server không thể truy cập hoặc không phản hồi trong 5 giây.</p>
<p><strong>Giải pháp:</strong></p>
<ul>
<li>Xác minh backend đang chạy: <code>curl -I http://127.0.0.1:3000</code></li>
<li>Kiểm tra backend URL trong config khớp với địa chỉ lắng nghe thực tế</li>
<li>Đảm bảo cổng backend không bị chặn bởi quy tắc tường lửa cục bộ</li>
</ul>

<h3>4. Lỗi 503 Service Unavailable Khi Tải Cao</h3>
<p><strong>Triệu chứng:</strong> Website trả về 503 trong các đợt tăng lưu lượng.</p>
<p><strong>Nguyên nhân:</strong> Connection pool cạn kiệt hoặc giới hạn goroutine đã đạt.</p>
<p><strong>Giải pháp:</strong> Đây là hành vi bình thường trong tải cực đoan để bảo vệ backend. Nếu lưu lượng hợp lệ bị từ chối, hãy xem xét nâng cấp gói hoặc điều chỉnh rate limit trong protection profile.</p>

<h3>5. Script Cài Đặt Thất Bại: OS Không Được Hỗ Trợ</h3>
<p><strong>Triệu chứng:</strong> Script cài đặt thoát với lỗi "unsupported operating system".</p>
<p><strong>Giải pháp:</strong> Kiro WAF hỗ trợ Ubuntu 20.04+, Debian 11+, CentOS 8+, Rocky 8+, Fedora 36+, và Arch Linux. Kiểm tra phiên bản OS:</p>
<pre><code>cat /etc/os-release</code></pre>

<h3>6. Xác Minh Checksum SHA-256 Thất Bại</h3>
<p><strong>Triệu chứng:</strong> Cài đặt hoặc cập nhật OTA bị hủy với lỗi checksum không khớp.</p>
<p><strong>Nguyên nhân:</strong> Tải xuống bị hỏng hoặc bị chặn.</p>
<p><strong>Giải pháp:</strong></p>
<ul>
<li>Thử lại cài đặt — vấn đề mạng có thể gây tải xuống không hoàn chỉnh</li>
<li>Xác minh kết nối mạng không bị chặn bởi proxy</li>
<li>Nếu vấn đề tiếp tục, liên hệ hỗ trợ với giá trị checksum mong đợi và thực tế</li>
</ul>

<h3>7. Cập Nhật OTA Tự Động Rollback</h3>
<p><strong>Triệu chứng:</strong> Journal hiển thị "rollback to previous version" sau cập nhật.</p>
<p><strong>Nguyên nhân:</strong> Binary mới không đạt health check trong 30 giây sau restart.</p>
<p><strong>Giải pháp:</strong> Đây là cơ chế an toàn. Phiên bản hoạt động trước đó được khôi phục tự động. Kiểm tra journal để biết lý do thất bại cụ thể:</p>
<pre><code>sudo journalctl -u kiro-client-waf --since "30 minutes ago"</code></pre>

<h3>8. Không Thể Kết Nối Đến Máy Chủ Quản Lý</h3>
<p><strong>Triệu chứng:</strong> Lỗi heartbeat trong log, xác thực license thất bại.</p>
<p><strong>Nguyên nhân:</strong> Vấn đề kết nối mạng đến máy chủ quản lý.</p>
<p><strong>Giải pháp:</strong></p>
<ul>
<li>Xác minh kết nối HTTPS ra ngoài: <code>curl -I https://firewall.vpsgen.com</code></li>
<li>Kiểm tra tường lửa có chặn cổng 443 ra ngoài không</li>
<li>Xác minh phân giải DNS hoạt động</li>
<li>Client sẽ tự động thử lại ở khoảng polling tiếp theo</li>
</ul>

<h3>9. Sử Dụng Bộ Nhớ Cao</h3>
<p><strong>Triệu chứng:</strong> Tiến trình Kiro WAF tiêu thụ nhiều bộ nhớ hơn mong đợi.</p>
<p><strong>Giải pháp:</strong></p>
<ul>
<li>Sử dụng bộ nhớ dưới 512 MB ở 100K rps là bình thường</li>
<li>Các entry rate-limit được dọn dẹp tự động mỗi 120 giây</li>
<li>Challenge token được dọn dẹp mỗi 60 giây</li>
<li>Nếu bộ nhớ tăng không giới hạn, khởi động lại service và báo cáo vấn đề</li>
</ul>
<pre><code>sudo systemctl restart kiro-client-waf</code></pre>

<h3>10. XDP Filter Không Tải Được</h3>
<p><strong>Triệu chứng:</strong> Chế độ XDP được bật nhưng không có lọc gói tin xảy ra.</p>
<p><strong>Giải pháp:</strong></p>
<ul>
<li>Xác minh dependency XDP đã cài: <code>which clang llvm-strip</code></li>
<li>Kiểm tra phiên bản kernel hỗ trợ XDP (4.8+ cho generic, 4.18+ cho native)</li>
<li>Xác minh giao diện mạng hỗ trợ chế độ XDP native</li>
<li>Kiểm tra journal cho lỗi tải BPF</li>
</ul>
<pre><code>sudo journalctl -u kiro-client-waf | grep -i xdp</code></pre>

<h3>11. Lưu Lượng Hợp Lệ Bị Chặn</h3>
<p><strong>Triệu chứng:</strong> Người dùng thực nhận trang challenge hoặc bị rate-limit.</p>
<p><strong>Giải pháp:</strong></p>
<ul>
<li>Chuyển sang protection profile nhẹ hơn: đặt <code>protection.profile: light</code></li>
<li>Thêm IP tin cậy vào danh sách admin allow</li>
<li>Kiểm tra <code>auto_attack_mode</code> có nâng mức bảo vệ do false positive không</li>
<li>Xem lại cài đặt rate limit cho các route cụ thể</li>
</ul>

<h3>12. Lỗi Cú Pháp File Cấu Hình</h3>
<p><strong>Triệu chứng:</strong> Service không khởi động sau khi thay đổi config.</p>
<p><strong>Giải pháp:</strong> Kiểm tra cú pháp YAML:</p>
<ul>
<li>Đảm bảo thụt lề nhất quán (2 dấu cách, không dùng tab)</li>
<li>Kiểm tra thiếu dấu hai chấm sau key</li>
<li>Xác minh giá trị chuỗi có ký tự đặc biệt được đặt trong ngoặc kép</li>
</ul>
<pre><code># Kiểm tra cú pháp YAML
python3 -c "import yaml; yaml.safe_load(open('/etc/kiro/kiro.yaml'))"</code></pre>
`

var viFAQ template.HTML = `<h2>Câu Hỏi Thường Gặp</h2>

<h3>Kiro WAF là gì?</h3>
<p>Kiro WAF là tường lửa ứng dụng web hiệu năng cao kết hợp lọc gói tin XDP/eBPF ở cấp kernel với reverse proxy Go để bảo vệ DDoS và bảo mật web toàn diện.</p>

<h3>Kiro WAF bảo vệ những gì?</h3>
<ul>
<li><strong>Lớp 3/4:</strong> Lọc gói tin XDP/eBPF ở tốc độ 10 triệu gói/giây</li>
<li><strong>Lớp 7:</strong> Kiểm tra HTTP request, giới hạn tốc độ, phát hiện bot</li>
<li><strong>Quy tắc WAF:</strong> Engine quy tắc dựa trên OWASP CRS cho SQL injection, XSS, v.v.</li>
<li><strong>Bảo vệ Bot:</strong> Cookie challenge, JavaScript challenge, Proof-of-Work</li>
</ul>

<h3>Cập nhật tự động hoạt động như thế nào?</h3>
<p>Kiro WAF kiểm tra cập nhật bằng cách polling máy chủ quản lý ở khoảng thời gian có thể cấu hình (mặc định: mỗi 5 phút). Khi có cập nhật, nó tải binary mới, xác minh checksum SHA-256, và thực hiện thay thế nguyên tử. Nếu phiên bản mới không đạt health check trong 30 giây, nó tự động rollback về phiên bản trước.</p>

<h3>Tôi có thể tắt cập nhật tự động không?</h3>
<p>Có. Đặt <code>updates.auto_security_updates: false</code> trong file cấu hình. Tuy nhiên, chúng tôi khuyến nghị giữ cập nhật tự động bật cho các bản vá bảo mật.</p>

<h3>Điều gì xảy ra khi bị tấn công DDoS?</h3>
<p>Khi <code>auto_attack_mode</code> được bật, Kiro WAF tự động nâng mức bảo vệ dựa trên mẫu lưu lượng phát hiện được. XDP filter xử lý tấn công volumetric ở cấp kernel mà không ảnh hưởng hiệu năng ứng dụng.</p>

<h3>Kiro WAF có hoạt động với Cloudflare không?</h3>
<p>Có. Đặt <code>website.cloudflare: true</code> để bật tích hợp Cloudflare. Kiro WAF sẽ khôi phục IP thực của khách truy cập từ header Cloudflare và có thể được cấu hình chỉ chấp nhận lưu lượng từ dải IP Cloudflare.</p>

<h3>Các protection profile là gì?</h3>
<ul>
<li><strong>light:</strong> Lọc tối thiểu, phù hợp cho API với client đã biết</li>
<li><strong>balanced:</strong> Bảo vệ tiêu chuẩn cho hầu hết website (khuyến nghị)</li>
<li><strong>strict:</strong> Lọc mạnh cho mục tiêu giá trị cao</li>
<li><strong>lockdown:</strong> Chế độ khẩn cấp — chỉ admin và client đã biết được phép</li>
</ul>

<h3>Làm sao kiểm tra trạng thái service?</h3>
<pre><code>sudo systemctl status kiro-client-waf
sudo journalctl -u kiro-client-waf -f</code></pre>

<h3>Làm sao cập nhật license key?</h3>
<p>Chỉnh sửa <code>/etc/kiro/kiro.yaml</code>, cập nhật giá trị <code>license_key</code>, và khởi động lại service:</p>
<pre><code>sudo systemctl restart kiro-client-waf</code></pre>

<h3>Kiro WAF sử dụng những cổng nào?</h3>
<ul>
<li><strong>Đầu vào:</strong> Cổng 80 và 443 cho lưu lượng web (có thể cấu hình)</li>
<li><strong>Đầu ra:</strong> Cổng 443 (HTTPS) cho giao tiếp với máy chủ quản lý</li>
<li><strong>Cục bộ:</strong> Proxy đến backend trên cổng đã cấu hình</li>
</ul>

<h3>Dữ liệu của tôi có được gửi đi đâu không?</h3>
<p>Mặc định, telemetry bị tắt (<code>telemetry.enabled: false</code>). Chỉ tín hiệu heartbeat (xác thực license và trạng thái sức khỏe cơ bản) được gửi đến máy chủ quản lý. Không có nội dung request, dữ liệu khách truy cập, hoặc thông tin nhạy cảm nào được truyền đi.</p>
`
