package handlers

import "html/template"

func buildVietnamesePages() map[string]DocsContentPage {
	return map[string]DocsContentPage{
		"quick-start":      {Title: "Bắt Đầu Nhanh", Content: viQuickStart},
		"installation":     {Title: "Hướng Dẫn Cài Đặt", Content: viInstallation},
		"cli-commands":     {Title: "Lệnh CLI", Content: viCLICommands},
		"config-reference": {Title: "Tham Chiếu Cấu Hình", Content: viConfigReference},
		"common-issues":    {Title: "Lỗi Thường Gặp", Content: viTroubleshooting},
		"faq":              {Title: "Câu Hỏi Thường Gặp", Content: viFAQ},
		"cli":              {Title: "Tổng Quan CLI", Content: viCLIOverview},
		"cli/version":      {Title: "Lệnh version", Content: viCLIVersion},
		"cli/license":      {Title: "Lệnh license", Content: viCLILicense},
		"cli/status":       {Title: "Lệnh status", Content: viCLIStatus},
		"cli/health":       {Title: "Lệnh health", Content: viCLIHealth},
		"cli/preflight":    {Title: "Lệnh preflight", Content: viCLIPreflight},
		"cli/mode":         {Title: "Lệnh mode", Content: viCLIMode},
		"cli/install":      {Title: "Lệnh install", Content: viCLIInstall},
		"cli/update":       {Title: "Lệnh update", Content: viCLIUpdate},
		"cli/incident":     {Title: "Lệnh incident", Content: viCLIIncident},
		"cli/pilot":        {Title: "Lệnh pilot", Content: viCLIPilot},
		"cli/report":       {Title: "Lệnh report", Content: viCLIReport},
	}
}

var viQuickStart template.HTML = `<div class="docs-welcome-card">
<h2>Hướng Dẫn Bắt Đầu Nhanh</h2>
<p>Cài đặt và bảo vệ server của bạn với Kiro WAF trong vòng 15 phút.</p>
</div>

<h2>Yêu Cầu</h2>
<ul>
<li>Máy chủ Linux (Ubuntu 20.04+, Debian 11+, CentOS 8+, Rocky 8+, Fedora 36+, hoặc Arch)</li>
<li>Quyền root hoặc sudo</li>
<li>Kết nối mạng đến máy chủ quản lý Kiro</li>
</ul>

<h2>Bước 1: Tải và Chạy Script Cài Đặt</h2>
<h3>Community Plan (miễn phí, tự đăng ký)</h3>
<pre><code>curl -fsSL https://firewall.vpsgen.com/install.sh | sudo bash</code></pre>
<p>Script tự động đăng ký gói Community miễn phí (không cần license key).</p>

<h3>Pro/Enterprise Plan (có license key)</h3>
<pre><code>curl -fsSL https://firewall.vpsgen.com/install.sh | sudo bash -s -- --license-key YOUR-LICENSE-KEY</code></pre>
<p>Script tự động phát hiện OS, cài đặt dependency, tải binary <code>kiro-client-waf</code>, và khởi động service <code>kiro-client-waf</code>.</p>

<h2>Bước 2: Cấu Hình Website</h2>
<p>Chỉnh sửa file cấu hình tại <code>/etc/kiro/kiro.yaml</code>:</p>
<pre><code>mode: full
license_key: YOUR-LICENSE-KEY

website:
  enabled: true
  cloudflare: true
  tls_mode: flexible_http
  sites:
    - domains:
        - example.com
      backend: http://127.0.0.1:3000

protection:
  profile: balanced
  waf: true
  bot: true</code></pre>

<h2>Bước 3: Khởi Động Lại Service</h2>
<pre><code>sudo systemctl restart kiro-client-waf</code></pre>

<h2>Bước 4: Xác Minh</h2>
<pre><code>sudo systemctl status kiro-client-waf</code></pre>
<p>Bạn sẽ thấy <code>active (running)</code>. Website của bạn đã được bảo vệ bởi Kiro WAF.</p>
`

var viInstallation template.HTML = `<h2>Hướng Dẫn Cài Đặt</h2>

<h3>Hệ Điều Hành Được Hỗ Trợ</h3>
<table>
<tr><th>Bản Phân Phối</th><th>Phiên Bản Tối Thiểu</th><th>Trình Quản Lý Gói</th></tr>
<tr><td>Ubuntu</td><td>20.04 LTS</td><td>apt</td></tr>
<tr><td>Debian</td><td>11 (Bullseye)</td><td>apt</td></tr>
<tr><td>CentOS</td><td>8</td><td>yum/dnf</td></tr>
<tr><td>Rocky Linux</td><td>8</td><td>dnf</td></tr>
<tr><td>Fedora</td><td>36</td><td>dnf</td></tr>
<tr><td>Arch Linux</td><td>Rolling</td><td>pacman</td></tr>
</table>

<h3>Yêu Cầu Hệ Thống</h3>
<ul>
<li><strong>CPU:</strong> Tối thiểu 1 core, khuyến nghị 2+ cores</li>
<li><strong>RAM:</strong> Tối thiểu 512 MB, khuyến nghị 1 GB</li>
<li><strong>Ổ đĩa:</strong> 100 MB dung lượng trống</li>
<li><strong>Mạng:</strong> Kết nối HTTPS ra ngoài đến máy chủ quản lý</li>
</ul>

<h3>Cài Đặt Tự Động</h3>
<p>Phương pháp cài đặt khuyến nghị sử dụng script tự động:</p>

<h4>Community Plan (miễn phí)</h4>
<pre><code>curl -fsSL https://firewall.vpsgen.com/install.sh | sudo bash</code></pre>
<p>Không cần license key — script tự động đăng ký gói Community miễn phí qua máy chủ quản lý.</p>

<h4>Pro/Enterprise Plan</h4>
<pre><code>curl -fsSL https://firewall.vpsgen.com/install.sh | sudo bash -s -- --license-key YOUR-LICENSE-KEY</code></pre>

<h3>Cài Đặt Chế Độ XDP</h3>
<p>Để lọc gói tin hiệu năng cao với XDP/eBPF:</p>
<pre><code>curl -fsSL https://firewall.vpsgen.com/install.sh | sudo bash -s -- --xdp-mode</code></pre>
<p>Lệnh này cài thêm dependency build (clang, llvm, libbpf-dev) để biên dịch XDP filter.</p>

<h3>Script Cài Đặt Thực Hiện Gì</h3>
<ol>
<li>Phát hiện bản phân phối và phiên bản OS</li>
<li>Cài đặt dependency cần thiết (curl, sha256sum, systemctl)</li>
<li>Tự đăng ký Community license nếu không có <code>--license-key</code> (gọi endpoint đăng ký trên máy chủ quản lý)</li>
<li>Tải binary <code>kiro-client-waf</code> với xác minh SHA-256</li>
<li>Cài đặt tại <code>/usr/local/bin/kiro-client-waf</code></li>
<li>Tạo thư mục cấu hình tại <code>/etc/kiro/</code></li>
<li>Cài đặt và kích hoạt systemd service <code>kiro-client-waf</code></li>
</ol>

<h3>Sau Khi Cài Đặt</h3>
<p>Cấu hình website trong <code>/etc/kiro/kiro.yaml</code> (file đã được tạo tự động) và khởi động lại service:</p>
<pre><code>sudo nano /etc/kiro/kiro.yaml    # Chỉnh domain và backend URL
sudo systemctl restart kiro-client-waf
sudo systemctl status kiro-client-waf</code></pre>

<h3>Cập Nhật</h3>
<pre><code>kiro-cli update check --master-url https://firewall.vpsgen.com
kiro-cli update apply --master-url https://firewall.vpsgen.com \
  --binary-path /usr/local/bin/kiro-client-waf --service kiro-client-waf</code></pre>

<h3>Gỡ Cài Đặt</h3>
<pre><code>sudo systemctl stop kiro-client-waf
sudo systemctl disable kiro-client-waf
sudo rm /usr/local/bin/kiro-client-waf
sudo rm -rf /etc/kiro/</code></pre>
`

var viCLICommands template.HTML = `<h2>Lệnh CLI (kiro-cli)</h2>
<p>Kiro CLI cung cấp các lệnh quản lý và giám sát hệ thống WAF từ dòng lệnh. Xem <a href="/docs/vi/cli">trang tổng quan CLI</a> để biết chi tiết đầy đủ cho từng lệnh.</p>

<h3>Danh Sách Lệnh</h3>
<table>
<tr><th>Lệnh</th><th>Mô Tả</th><th>Chi Tiết</th></tr>
<tr><td><code>version</code></td><td>Hiển thị phiên bản hiện tại</td><td><a href="/docs/vi/cli/version">Xem</a></td></tr>
<tr><td><code>license fingerprint</code></td><td>Tạo machine fingerprint hash</td><td><a href="/docs/vi/cli/license">Xem</a></td></tr>
<tr><td><code>status</code></td><td>Trạng thái runtime hệ thống</td><td><a href="/docs/vi/cli/status">Xem</a></td></tr>
<tr><td><code>health</code></td><td>Kiểm tra sức khỏe tổng hợp</td><td><a href="/docs/vi/cli/health">Xem</a></td></tr>
<tr><td><code>preflight</code></td><td>Kiểm tra điều kiện tiên quyết</td><td><a href="/docs/vi/cli/preflight">Xem</a></td></tr>
<tr><td><code>mode</code></td><td>Hiển thị/thay đổi chế độ hoạt động</td><td><a href="/docs/vi/cli/mode">Xem</a></td></tr>
<tr><td><code>install</code></td><td>Quản lý cài đặt (plan, stage, apply)</td><td><a href="/docs/vi/cli/install">Xem</a></td></tr>
<tr><td><code>update</code></td><td>Quản lý cập nhật (check, apply, rollback)</td><td><a href="/docs/vi/cli/update">Xem</a></td></tr>
<tr><td><code>incident</code></td><td>Tạo báo cáo sự cố</td><td><a href="/docs/vi/cli/incident">Xem</a></td></tr>
<tr><td><code>pilot</code></td><td>Tạo báo cáo pilot go/no-go</td><td><a href="/docs/vi/cli/pilot">Xem</a></td></tr>
<tr><td><code>report</code></td><td>Báo cáo tổng hợp hệ thống</td><td><a href="/docs/vi/cli/report">Xem</a></td></tr>
</table>

<h3>Sử Dụng Nhanh</h3>
<pre><code>kiro-cli version
kiro-cli status
kiro-cli health
kiro-cli update check --master-url https://firewall.vpsgen.com</code></pre>

<h3>Mã Thoát Chung</h3>
<table>
<tr><th>Mã</th><th>Ý Nghĩa</th></tr>
<tr><td>0</td><td>Thành công</td></tr>
<tr><td>1</td><td>Lỗi thực thi (validation, permission, runtime)</td></tr>
<tr><td>2</td><td>Lỗi sử dụng (lệnh không hợp lệ, thiếu tham số bắt buộc)</td></tr>
</table>
`

var viCLIOverview template.HTML = `<div class="docs-welcome-card">
<h2>Tổng Quan CLI (kiro-cli)</h2>
<p>Công cụ dòng lệnh Kiro CLI cung cấp các lệnh quản trị và chẩn đoán hệ thống WAF.</p>
</div>

<div class="cli-search-box">
<input type="text" id="cli-search" placeholder="Tìm kiếm lệnh..." onkeyup="filterCLICommands()" aria-label="Tìm kiếm lệnh CLI">
</div>

<h2>Mục Lục Lệnh</h2>
<div class="cli-toc" id="cli-command-list">
<table>
<tr><th>Lệnh</th><th>Mô Tả</th><th>Trang Chi Tiết</th></tr>
<tr class="cli-cmd-row" data-cmd="version"><td><code>kiro-cli version</code></td><td>Hiển thị phiên bản build (semver X.Y.Z)</td><td><a href="/docs/vi/cli/version">Chi tiết →</a></td></tr>
<tr class="cli-cmd-row" data-cmd="license fingerprint"><td><code>kiro-cli license fingerprint</code></td><td>Tạo machine fingerprint hash duy nhất</td><td><a href="/docs/vi/cli/license">Chi tiết →</a></td></tr>
<tr class="cli-cmd-row" data-cmd="status"><td><code>kiro-cli status</code></td><td>Trạng thái runtime (mode, uptime, license, version)</td><td><a href="/docs/vi/cli/status">Chi tiết →</a></td></tr>
<tr class="cli-cmd-row" data-cmd="health"><td><code>kiro-cli health</code></td><td>Kiểm tra sức khỏe tổng hợp hệ thống</td><td><a href="/docs/vi/cli/health">Chi tiết →</a></td></tr>
<tr class="cli-cmd-row" data-cmd="preflight"><td><code>kiro-cli preflight</code></td><td>Kiểm tra điều kiện tiên quyết triển khai</td><td><a href="/docs/vi/cli/preflight">Chi tiết →</a></td></tr>
<tr class="cli-cmd-row" data-cmd="mode show set"><td><code>kiro-cli mode</code></td><td>Hiển thị/thay đổi chế độ hoạt động (server/full)</td><td><a href="/docs/vi/cli/mode">Chi tiết →</a></td></tr>
<tr class="cli-cmd-row" data-cmd="install plan stage-lab apply-lab"><td><code>kiro-cli install</code></td><td>Quản lý cài đặt (plan, stage-lab, apply-lab)</td><td><a href="/docs/vi/cli/install">Chi tiết →</a></td></tr>
<tr class="cli-cmd-row" data-cmd="update check apply rollback"><td><code>kiro-cli update</code></td><td>Quản lý cập nhật OTA (check, apply, rollback)</td><td><a href="/docs/vi/cli/update">Chi tiết →</a></td></tr>
<tr class="cli-cmd-row" data-cmd="incident report"><td><code>kiro-cli incident report</code></td><td>Tạo báo cáo sự cố (JSON + Markdown)</td><td><a href="/docs/vi/cli/incident">Chi tiết →</a></td></tr>
<tr class="cli-cmd-row" data-cmd="pilot report"><td><code>kiro-cli pilot report</code></td><td>Tạo báo cáo pilot go/no-go</td><td><a href="/docs/vi/cli/pilot">Chi tiết →</a></td></tr>
<tr class="cli-cmd-row" data-cmd="report"><td><code>kiro-cli report</code></td><td>Báo cáo tổng hợp hệ thống</td><td><a href="/docs/vi/cli/report">Chi tiết →</a></td></tr>
</table>
</div>

<h2>Cài Đặt</h2>
<p>Kiro CLI được cài đặt tự động cùng với Kiro WAF client. Binary nằm tại <code>/usr/local/bin/kiro-cli</code>.</p>

<h2>Mã Thoát Chung</h2>
<table>
<tr><th>Mã</th><th>Ý Nghĩa</th></tr>
<tr><td>0</td><td>Thành công — lệnh thực thi hoàn tất không lỗi</td></tr>
<tr><td>1</td><td>Lỗi thực thi — validation thất bại, permission denied, hoặc runtime error</td></tr>
<tr><td>2</td><td>Lỗi sử dụng — lệnh không hợp lệ hoặc thiếu tham số bắt buộc</td></tr>
</table>

<h2>Sử Dụng Nhanh</h2>
<pre><code># Kiểm tra phiên bản
kiro-cli version

# Xem trạng thái hệ thống
kiro-cli status

# Kiểm tra sức khỏe
kiro-cli health

# Kiểm tra cập nhật
kiro-cli update check --master-url https://firewall.vpsgen.com</code></pre>

<script>
function filterCLICommands() {
  var input = document.getElementById("cli-search").value.toLowerCase();
  var rows = document.querySelectorAll(".cli-cmd-row");
  rows.forEach(function(row) {
    var cmd = row.getAttribute("data-cmd");
    var text = row.textContent.toLowerCase();
    if (cmd.indexOf(input) > -1 || text.indexOf(input) > -1) {
      row.style.display = "";
    } else {
      row.style.display = "none";
    }
  });
}
</script>
`

var viCLIVersion template.HTML = `<h2>kiro-cli version</h2>
<p>Hiển thị phiên bản build hiện tại của Kiro CLI theo định dạng semver.</p>

<h3>Cú Pháp</h3>
<pre><code>kiro-cli version</code></pre>

<h3>Tham Số</h3>
<p>Lệnh này không có tham số.</p>

<h3>Output</h3>
<p>Trả về chuỗi phiên bản theo định dạng semver <code>X.Y.Z</code> (ví dụ: <code>1.2.3</code>) hoặc <code>X.Y.Z-suffix</code> (ví dụ: <code>0.1.0-dev</code>).</p>

<h3>Ví Dụ</h3>
<pre><code>$ kiro-cli version
1.0.0</code></pre>

<h3>Mã Thoát</h3>
<table>
<tr><th>Mã</th><th>Ý Nghĩa</th></tr>
<tr><td>0</td><td>Thành công — phiên bản được hiển thị</td></tr>
</table>

<h3>Liên Quan</h3>
<ul>
<li><a href="/docs/vi/cli/status">kiro-cli status</a> — xem trạng thái đầy đủ bao gồm phiên bản</li>
<li><a href="/docs/vi/cli/report">kiro-cli report</a> — báo cáo hệ thống bao gồm phiên bản</li>
</ul>
`

var viCLILicense template.HTML = `<h2>kiro-cli license fingerprint</h2>
<p>Tạo machine fingerprint hash duy nhất cho máy chủ hiện tại. Fingerprint được sử dụng để xác định máy chủ khi đăng ký license.</p>

<h3>Cú Pháp</h3>
<pre><code>kiro-cli license fingerprint [--salt &lt;value&gt;]</code></pre>

<h3>Tham Số</h3>
<table>
<tr><th>Tham Số</th><th>Bắt Buộc</th><th>Kiểu</th><th>Mặc Định</th><th>Mô Tả</th></tr>
<tr><td><code>--salt</code></td><td>Không</td><td>string</td><td>(rỗng)</td><td>Giá trị salt tùy chỉnh để tạo fingerprint khác biệt</td></tr>
</table>

<h3>Output</h3>
<p>Trả về chuỗi hash hex 64 ký tự lowercase (SHA-256). Kết quả là deterministic — cùng máy và cùng salt luôn cho cùng kết quả.</p>

<h3>Ví Dụ</h3>
<pre><code># Fingerprint mặc định
$ kiro-cli license fingerprint
a1b2c3d4e5f6789012345678901234567890123456789012345678901234abcd

# Fingerprint với salt tùy chỉnh
$ kiro-cli license fingerprint --salt "production-server-01"
f9e8d7c6b5a4321098765432109876543210987654321098765432109876fedc</code></pre>

<h3>Mã Thoát</h3>
<table>
<tr><th>Mã</th><th>Ý Nghĩa</th></tr>
<tr><td>0</td><td>Thành công — fingerprint được hiển thị</td></tr>
<tr><td>1</td><td>Lỗi — không thể đọc thông tin phần cứng</td></tr>
</table>

<h3>Liên Quan</h3>
<ul>
<li><a href="/docs/vi/cli/status">kiro-cli status</a> — xem trạng thái license hiện tại</li>
</ul>
`

var viCLIStatus template.HTML = `<h2>kiro-cli status</h2>
<p>Hiển thị trạng thái runtime hiện tại của hệ thống Kiro WAF dưới dạng JSON.</p>

<h3>Cú Pháp</h3>
<pre><code>kiro-cli status [--config &lt;path&gt;]</code></pre>

<h3>Tham Số</h3>
<table>
<tr><th>Tham Số</th><th>Bắt Buộc</th><th>Kiểu</th><th>Mặc Định</th><th>Mô Tả</th></tr>
<tr><td><code>--config</code></td><td>Không</td><td>string</td><td>/etc/kiro/kiro.yaml</td><td>Đường dẫn file cấu hình YAML (mặc định: /etc/kiro/kiro.yaml)</td></tr>
</table>

<h3>Output JSON</h3>
<p>Trả về JSON chứa các trường:</p>
<table>
<tr><th>Trường</th><th>Kiểu</th><th>Mô Tả</th></tr>
<tr><td><code>mode</code></td><td>string</td><td>Chế độ hoạt động: "server" hoặc "full"</td></tr>
<tr><td><code>uptime</code></td><td>string</td><td>Thời gian hoạt động (ví dụ: "2h30m")</td></tr>
<tr><td><code>license_status</code></td><td>string</td><td>Trạng thái license: active, suspended, downgraded</td></tr>
<tr><td><code>version</code></td><td>string</td><td>Phiên bản hiện tại (semver)</td></tr>
</table>

<h3>Ví Dụ</h3>
<pre><code>$ kiro-cli status
{
  "mode": "full",
  "uptime": "48h12m",
  "license_status": "active",
  "version": "1.0.0",
  "plan": "community",
  "sites": 1,
  "services": {
    "firewall": "active",
    "proxy": "active",
    "waf": "active",
    "bot_protection": "active"
  }
}</code></pre>

<h3>Mã Thoát</h3>
<table>
<tr><th>Mã</th><th>Ý Nghĩa</th></tr>
<tr><td>0</td><td>Thành công — trạng thái được hiển thị</td></tr>
<tr><td>1</td><td>Lỗi — không thể đọc cấu hình hoặc truy vấn trạng thái</td></tr>
<tr><td>2</td><td>Lỗi đọc cấu hình</td></tr>
</table>

<h3>Liên Quan</h3>
<ul>
<li><a href="/docs/vi/cli/health">kiro-cli health</a> — kiểm tra sức khỏe chi tiết</li>
<li><a href="/docs/vi/cli/report">kiro-cli report</a> — báo cáo tổng hợp</li>
</ul>
`

var viCLIHealth template.HTML = `<h2>kiro-cli health</h2>
<p>Kiểm tra sức khỏe tổng hợp của hệ thống bao gồm service status, preflight checks, và overall health.</p>

<h3>Cú Pháp</h3>
<pre><code>kiro-cli health [--config &lt;path&gt;] [--os-release &lt;path&gt;] [--preflight-writable-root &lt;path&gt;] [--skip-command-checks]</code></pre>

<h3>Tham Số</h3>
<table>
<tr><th>Tham Số</th><th>Bắt Buộc</th><th>Kiểu</th><th>Mặc Định</th><th>Mô Tả</th></tr>
<tr><td><code>--config</code></td><td>Không</td><td>string</td><td>/etc/kiro/kiro.yaml</td><td>Đường dẫn file cấu hình (mặc định: /etc/kiro/kiro.yaml)</td></tr>
<tr><td><code>--os-release</code></td><td>Không</td><td>string</td><td>/etc/os-release</td><td>Đường dẫn file os-release (cho testing)</td></tr>
<tr><td><code>--preflight-writable-root</code></td><td>Không</td><td>string</td><td>/</td><td>Thư mục gốc cho kiểm tra writable (cho testing)</td></tr>
<tr><td><code>--skip-command-checks</code></td><td>Không</td><td>bool</td><td>false</td><td>Bỏ qua kiểm tra command availability</td></tr>
</table>

<h3>Output JSON</h3>
<table>
<tr><th>Trường</th><th>Kiểu</th><th>Mô Tả</th></tr>
<tr><td><code>overall_status</code></td><td>string</td><td>"healthy", "degraded", hoặc "unhealthy"</td></tr>
<tr><td><code>service_status</code></td><td>string</td><td>"active" hoặc "inactive"</td></tr>
<tr><td><code>preflight</code></td><td>object</td><td>Kết quả kiểm tra điều kiện tiên quyết</td></tr>
</table>

<h3>Ví Dụ</h3>
<pre><code>$ kiro-cli health
{
  "overall_status": "healthy",
  "service_status": "active",
  "preflight": {
    "os_compatible": true,
    "root_access": true,
    "commands_available": {
      "nft": true,
      "nginx": true,
      "systemctl": true
    }
  },
  "version": "1.0.0"
}</code></pre>

<h3>Mã Thoát</h3>
<table>
<tr><th>Mã</th><th>Ý Nghĩa</th></tr>
<tr><td>0</td><td>Thành công — kết quả health check được hiển thị (bất kể overall_status)</td></tr>
<tr><td>1</td><td>Lỗi — không thể thực hiện health check</td></tr>
<tr><td>2</td><td>Lỗi đọc cấu hình</td></tr>
</table>

<h3>Liên Quan</h3>
<ul>
<li><a href="/docs/vi/cli/preflight">kiro-cli preflight</a> — chỉ kiểm tra điều kiện tiên quyết</li>
<li><a href="/docs/vi/cli/status">kiro-cli status</a> — trạng thái runtime</li>
</ul>
`

var viCLIPreflight template.HTML = `<h2>kiro-cli preflight</h2>
<p>Kiểm tra điều kiện tiên quyết trước khi triển khai: OS compatibility, quyền root, và command availability.</p>

<h3>Cú Pháp</h3>
<pre><code>kiro-cli preflight [--config &lt;path&gt;] [--os-release &lt;path&gt;] [--preflight-writable-root &lt;path&gt;] [--skip-command-checks]</code></pre>

<h3>Tham Số</h3>
<table>
<tr><th>Tham Số</th><th>Bắt Buộc</th><th>Kiểu</th><th>Mặc Định</th><th>Mô Tả</th></tr>
<tr><td><code>--config</code></td><td>Không</td><td>string</td><td>/etc/kiro/kiro.yaml</td><td>Đường dẫn file cấu hình (mặc định: /etc/kiro/kiro.yaml)</td></tr>
<tr><td><code>--os-release</code></td><td>Không</td><td>string</td><td>/etc/os-release</td><td>Đường dẫn file os-release</td></tr>
<tr><td><code>--preflight-writable-root</code></td><td>Không</td><td>string</td><td>/</td><td>Thư mục gốc cho kiểm tra writable</td></tr>
<tr><td><code>--skip-command-checks</code></td><td>Không</td><td>bool</td><td>false</td><td>Bỏ qua kiểm tra command availability</td></tr>
</table>

<h3>Output JSON</h3>
<table>
<tr><th>Trường</th><th>Kiểu</th><th>Mô Tả</th></tr>
<tr><td><code>os_compatible</code></td><td>bool</td><td>OS có được hỗ trợ (Ubuntu 22.04/24.04)</td></tr>
<tr><td><code>root_access</code></td><td>bool</td><td>Đang chạy với UID 0</td></tr>
<tr><td><code>commands_available</code></td><td>object</td><td>Trạng thái từng command (nft, nginx, systemctl)</td></tr>
</table>

<h3>Ví Dụ</h3>
<pre><code>$ sudo kiro-cli preflight
{
  "os_compatible": true,
  "os_id": "ubuntu",
  "os_version": "22.04",
  "root_access": true,
  "commands_available": {
    "nft": true,
    "nginx": true,
    "systemctl": true
  },
  "writable_paths": {
    "/usr/local/bin": true,
    "/etc/kiro": true
  }
}</code></pre>

<h3>Mã Thoát</h3>
<table>
<tr><th>Mã</th><th>Ý Nghĩa</th></tr>
<tr><td>0</td><td>Thành công — kết quả preflight được hiển thị</td></tr>
<tr><td>1</td><td>Lỗi — không thể thực hiện kiểm tra</td></tr>
<tr><td>2</td><td>Lỗi đọc cấu hình</td></tr>
</table>

<h3>Liên Quan</h3>
<ul>
<li><a href="/docs/vi/cli/health">kiro-cli health</a> — health check bao gồm preflight</li>
<li><a href="/docs/vi/cli/install">kiro-cli install</a> — cài đặt sau khi preflight pass</li>
</ul>
`

var viCLIMode template.HTML = `<h2>kiro-cli mode</h2>
<p>Hiển thị hoặc thay đổi chế độ hoạt động của Kiro WAF. Hai chế độ: <code>server</code> (chỉ WAF proxy) và <code>full</code> (WAF proxy + XDP filter).</p>

<h3>Cú Pháp</h3>
<pre><code>kiro-cli mode show
kiro-cli mode set --mode &lt;value&gt;</code></pre>

<h3>Tham Số</h3>
<table>
<tr><th>Tham Số</th><th>Bắt Buộc</th><th>Kiểu</th><th>Mặc Định</th><th>Mô Tả</th></tr>
<tr><td><code>--mode</code></td><td>Có (cho set)</td><td>string</td><td>—</td><td>Chế độ mới: "server" hoặc "full"</td></tr>
</table>

<h3>Giá Trị Hợp Lệ</h3>
<table>
<tr><th>Giá Trị</th><th>Mô Tả</th></tr>
<tr><td><code>server</code></td><td>Chỉ WAF reverse proxy (Golang HTTP), không XDP</td></tr>
<tr><td><code>full</code></td><td>WAF reverse proxy + XDP/eBPF packet filter</td></tr>
</table>

<h3>Ví Dụ</h3>
<pre><code># Xem mode hiện tại
$ kiro-cli mode show
server

# Chuyển sang full mode
$ kiro-cli mode set --mode full
Mode changed to: full

# Giá trị không hợp lệ
$ kiro-cli mode set --mode turbo
Error: invalid mode "turbo", must be "server" or "full"</code></pre>

<h3>Mã Thoát</h3>
<table>
<tr><th>Mã</th><th>Ý Nghĩa</th></tr>
<tr><td>0</td><td>Thành công — mode được hiển thị hoặc thay đổi</td></tr>
<tr><td>1</td><td>Lỗi — giá trị --mode không hợp lệ (không phải "server" hoặc "full")</td></tr>
<tr><td>2</td><td>Thiếu sub-command (show/set) hoặc thiếu --mode cho set</td></tr>
</table>

<h3>Liên Quan</h3>
<ul>
<li><a href="/docs/vi/cli/status">kiro-cli status</a> — xem mode trong trạng thái tổng quan</li>
</ul>
`

var viCLIInstall template.HTML = `<h2>kiro-cli install</h2>
<p>Quản lý cài đặt Kiro WAF client trên máy chủ. Bao gồm ba sub-command: plan, stage-lab, và apply-lab.</p>

<h3>Cú Pháp</h3>
<pre><code>kiro-cli install plan [--config &lt;path&gt;]
kiro-cli install stage-lab --install-root &lt;path&gt; [--config &lt;path&gt;]
kiro-cli install apply-lab --ack KIRO_LAB_INSTALL_APPLY [--install-root &lt;path&gt;]</code></pre>

<h3>Sub-Commands</h3>

<h4>install plan</h4>
<p>Hiển thị kế hoạch cài đặt dưới dạng JSON mà không thực hiện thay đổi.</p>
<table>
<tr><th>Tham Số</th><th>Bắt Buộc</th><th>Kiểu</th><th>Mặc Định</th><th>Mô Tả</th></tr>
<tr><td><code>--config</code></td><td>Không</td><td>string</td><td>/etc/kiro/kiro.yaml</td><td>Đường dẫn file cấu hình</td></tr>
</table>

<h4>install stage-lab</h4>
<p>Staging cài đặt vào thư mục chỉ định (dry-run an toàn).</p>
<table>
<tr><th>Tham Số</th><th>Bắt Buộc</th><th>Kiểu</th><th>Mặc Định</th><th>Mô Tả</th></tr>
<tr><td><code>--install-root</code></td><td>Có</td><td>string</td><td>—</td><td>Thư mục đích cho staging</td></tr>
<tr><td><code>--config</code></td><td>Không</td><td>string</td><td>/etc/kiro/kiro.yaml</td><td>Đường dẫn file cấu hình</td></tr>
</table>

<h4>install apply-lab</h4>
<p>Áp dụng cài đặt thực tế. Yêu cầu xác nhận và quyền root.</p>
<table>
<tr><th>Tham Số</th><th>Bắt Buộc</th><th>Kiểu</th><th>Mặc Định</th><th>Mô Tả</th></tr>
<tr><td><code>--ack</code></td><td>Có</td><td>string</td><td>—</td><td>Phải là "KIRO_LAB_INSTALL_APPLY" để xác nhận</td></tr>
<tr><td><code>--install-root</code></td><td>Không</td><td>string</td><td>/</td><td>Thư mục gốc cài đặt</td></tr>
</table>

<h3>Ví Dụ</h3>
<pre><code># Xem kế hoạch cài đặt
$ kiro-cli install plan
{
  "binary_path": "/usr/local/bin/kiro-client-waf",
  "config_dir": "/etc/kiro",
  "service_name": "kiro-client-waf",
  "mode": "full"
}

# Staging vào thư mục test
$ kiro-cli install stage-lab --install-root /tmp/kiro-test

# Áp dụng cài đặt (yêu cầu root)
$ sudo kiro-cli install apply-lab --ack KIRO_LAB_INSTALL_APPLY

# Lỗi: ack không đúng
$ sudo kiro-cli install apply-lab --ack wrong-value
Error: --ack must be "KIRO_LAB_INSTALL_APPLY"

# Lỗi: không có quyền root
$ kiro-cli install apply-lab --ack KIRO_LAB_INSTALL_APPLY
Error: install apply-lab requires root privileges (UID 0)</code></pre>

<h3>Mã Thoát</h3>
<table>
<tr><th>Mã</th><th>Ý Nghĩa</th></tr>
<tr><td>0</td><td>Thành công</td></tr>
<tr><td>1</td><td>Lỗi — ack không đúng, không có quyền root, hoặc cài đặt thất bại</td></tr>
<tr><td>2</td><td>Thiếu tham số bắt buộc</td></tr>
</table>

<h3>Liên Quan</h3>
<ul>
<li><a href="/docs/vi/cli/preflight">kiro-cli preflight</a> — kiểm tra trước khi cài đặt</li>
<li><a href="/docs/vi/cli/update">kiro-cli update</a> — cập nhật sau khi đã cài đặt</li>
</ul>
`

var viCLIUpdate template.HTML = `<h2>kiro-cli update</h2>
<p>Quản lý cập nhật OTA cho Kiro WAF client. Bao gồm ba sub-command: check, apply, và rollback.</p>

<h3>Cú Pháp</h3>
<pre><code>kiro-cli update check --master-url &lt;url&gt; [--component &lt;name&gt;] [--channel &lt;name&gt;]
kiro-cli update apply --master-url &lt;url&gt; --binary-path &lt;path&gt; --service &lt;name&gt; [--component &lt;name&gt;] [--channel &lt;name&gt;]
kiro-cli update rollback --binary-path &lt;path&gt; --service &lt;name&gt;</code></pre>

<h3>Sub-Commands</h3>

<h4>update check</h4>
<p>Kiểm tra bản cập nhật mới từ Master Server.</p>
<table>
<tr><th>Tham Số</th><th>Bắt Buộc</th><th>Kiểu</th><th>Mặc Định</th><th>Mô Tả</th></tr>
<tr><td><code>--master-url</code></td><td>Có</td><td>string</td><td>—</td><td>URL của Master Server</td></tr>
<tr><td><code>--component</code></td><td>Không</td><td>string</td><td>kiro-client-waf</td><td>Tên component cần kiểm tra</td></tr>
<tr><td><code>--channel</code></td><td>Không</td><td>string</td><td>stable</td><td>Kênh cập nhật (stable, beta)</td></tr>
</table>

<h4>update apply</h4>
<p>Tải và áp dụng bản cập nhật với xác minh SHA-256. Tự động rollback nếu health check thất bại trong 30 giây.</p>
<table>
<tr><th>Tham Số</th><th>Bắt Buộc</th><th>Kiểu</th><th>Mặc Định</th><th>Mô Tả</th></tr>
<tr><td><code>--master-url</code></td><td>Có</td><td>string</td><td>—</td><td>URL của Master Server</td></tr>
<tr><td><code>--binary-path</code></td><td>Có</td><td>string</td><td>—</td><td>Đường dẫn binary hiện tại</td></tr>
<tr><td><code>--service</code></td><td>Có</td><td>string</td><td>—</td><td>Tên systemd service</td></tr>
<tr><td><code>--component</code></td><td>Không</td><td>string</td><td>kiro-client-waf</td><td>Tên component</td></tr>
<tr><td><code>--channel</code></td><td>Không</td><td>string</td><td>stable</td><td>Kênh cập nhật</td></tr>
</table>

<h4>update rollback</h4>
<p>Khôi phục phiên bản trước từ file backup (.bak).</p>
<table>
<tr><th>Tham Số</th><th>Bắt Buộc</th><th>Kiểu</th><th>Mặc Định</th><th>Mô Tả</th></tr>
<tr><td><code>--binary-path</code></td><td>Có</td><td>string</td><td>—</td><td>Đường dẫn binary hiện tại</td></tr>
<tr><td><code>--service</code></td><td>Có</td><td>string</td><td>—</td><td>Tên systemd service</td></tr>
</table>

<h3>Ví Dụ</h3>
<pre><code># Kiểm tra cập nhật
$ kiro-cli update check --master-url https://firewall.vpsgen.com
{
  "update_available": true,
  "current_version": "1.0.0",
  "new_version": "1.1.0",
  "artifact_url": "https://firewall.vpsgen.com/releases/kiro-client-waf-1.1.0",
  "sha256": "abc123..."
}

# Áp dụng cập nhật
$ sudo kiro-cli update apply \
  --master-url https://firewall.vpsgen.com \
  --binary-path /usr/local/bin/kiro-client-waf \
  --service kiro-client-waf

# Rollback về phiên bản trước
$ sudo kiro-cli update rollback \
  --binary-path /usr/local/bin/kiro-client-waf \
  --service kiro-client-waf</code></pre>

<h3>Mã Thoát</h3>
<table>
<tr><th>Mã</th><th>Ý Nghĩa</th></tr>
<tr><td>0</td><td>Thành công</td></tr>
<tr><td>1</td><td>Lỗi — SHA-256 mismatch, health check thất bại (đã auto-rollback), hoặc rollback thất bại</td></tr>
<tr><td>2</td><td>Thiếu tham số bắt buộc (--master-url, --binary-path, --service)</td></tr>
</table>

<h3>Liên Quan</h3>
<ul>
<li><a href="/docs/vi/cli/version">kiro-cli version</a> — xem phiên bản hiện tại</li>
<li><a href="/docs/vi/cli/health">kiro-cli health</a> — kiểm tra sức khỏe sau cập nhật</li>
</ul>
`

var viCLIIncident template.HTML = `<h2>kiro-cli incident report</h2>
<p>Tạo báo cáo sự cố bảo mật dưới dạng JSON và Markdown. Lưu vào thư mục output chỉ định.</p>

<h3>Cú Pháp</h3>
<pre><code>kiro-cli incident report --type &lt;type&gt; --severity &lt;level&gt; --status &lt;status&gt; --summary &lt;text&gt; [--output-dir &lt;path&gt;]</code></pre>

<h3>Tham Số</h3>
<table>
<tr><th>Tham Số</th><th>Bắt Buộc</th><th>Kiểu</th><th>Mặc Định</th><th>Mô Tả</th></tr>
<tr><td><code>--type</code></td><td>Có</td><td>string</td><td>—</td><td>Loại sự cố (xem bảng bên dưới)</td></tr>
<tr><td><code>--severity</code></td><td>Có</td><td>string</td><td>—</td><td>Mức độ: critical, high, medium, low</td></tr>
<tr><td><code>--status</code></td><td>Có</td><td>string</td><td>—</td><td>Trạng thái: open, investigating, resolved, closed</td></tr>
<tr><td><code>--summary</code></td><td>Có</td><td>string</td><td>—</td><td>Mô tả ngắn gọn sự cố</td></tr>
<tr><td><code>--output-dir</code></td><td>Không</td><td>string</td><td>./incidents</td><td>Thư mục lưu báo cáo</td></tr>
</table>

<h4>Giá Trị --type Hợp Lệ</h4>
<table>
<tr><th>Giá Trị</th><th>Mô Tả</th></tr>
<tr><td><code>attack</code></td><td>Tấn công DDoS hoặc brute-force</td></tr>
<tr><td><code>lost_ssh</code></td><td>Mất kết nối SSH đến server</td></tr>
<tr><td><code>update_failed</code></td><td>Cập nhật OTA thất bại</td></tr>
<tr><td><code>origin_ip_leaked</code></td><td>IP gốc bị lộ</td></tr>
<tr><td><code>license_rebind</code></td><td>License bị rebind sang máy khác</td></tr>
<tr><td><code>runtime_security</code></td><td>Lỗi bảo mật runtime</td></tr>
<tr><td><code>other</code></td><td>Sự cố khác</td></tr>
</table>

<h3>Ví Dụ</h3>
<pre><code>$ kiro-cli incident report \
  --type attack \
  --severity high \
  --status investigating \
  --summary "DDoS attack detected, 50k rps from multiple IPs" \
  --output-dir /var/log/kiro/incidents

Created: /var/log/kiro/incidents/incident-2024-01-15T10-30-00Z.json
Created: /var/log/kiro/incidents/incident-2024-01-15T10-30-00Z.md</code></pre>

<h3>Mã Thoát</h3>
<table>
<tr><th>Mã</th><th>Ý Nghĩa</th></tr>
<tr><td>0</td><td>Thành công — báo cáo được tạo</td></tr>
<tr><td>1</td><td>Lỗi — không thể ghi file hoặc giá trị tham số không hợp lệ</td></tr>
<tr><td>2</td><td>Thiếu tham số bắt buộc (--type, --severity, --status, --summary)</td></tr>
</table>

<h3>Liên Quan</h3>
<ul>
<li><a href="/docs/vi/cli/pilot">kiro-cli pilot report</a> — tổng hợp incidents vào báo cáo pilot</li>
<li><a href="/docs/vi/cli/report">kiro-cli report</a> — báo cáo tổng hợp hệ thống</li>
</ul>
`

var viCLIPilot template.HTML = `<h2>kiro-cli pilot report</h2>
<p>Tạo báo cáo pilot go/no-go bằng cách tổng hợp evidence từ health check, benchmark, và incident reports.</p>

<h3>Cú Pháp</h3>
<pre><code>kiro-cli pilot report --server-count &lt;n&gt; --started-at &lt;RFC3339&gt; --ended-at &lt;RFC3339&gt; [--health-file &lt;path&gt;] [--benchmark-file &lt;path&gt;] [--incident-dir &lt;path&gt;] [--output-dir &lt;path&gt;]</code></pre>

<h3>Tham Số</h3>
<table>
<tr><th>Tham Số</th><th>Bắt Buộc</th><th>Kiểu</th><th>Mặc Định</th><th>Mô Tả</th></tr>
<tr><td><code>--server-count</code></td><td>Có</td><td>int</td><td>—</td><td>Số server trong pilot</td></tr>
<tr><td><code>--started-at</code></td><td>Có</td><td>string</td><td>—</td><td>Thời điểm bắt đầu pilot (RFC3339)</td></tr>
<tr><td><code>--ended-at</code></td><td>Có</td><td>string</td><td>—</td><td>Thời điểm kết thúc pilot (RFC3339)</td></tr>
<tr><td><code>--health-file</code></td><td>Không</td><td>string</td><td>—</td><td>File JSON kết quả health check</td></tr>
<tr><td><code>--benchmark-file</code></td><td>Không</td><td>string</td><td>—</td><td>File JSON kết quả benchmark</td></tr>
<tr><td><code>--incident-dir</code></td><td>Không</td><td>string</td><td>—</td><td>Thư mục chứa incident reports</td></tr>
<tr><td><code>--output-dir</code></td><td>Không</td><td>string</td><td>./pilot</td><td>Thư mục lưu báo cáo pilot</td></tr>
</table>

<h3>Output</h3>
<p>Tạo báo cáo JSON và Markdown chứa:</p>
<ul>
<li>Kết luận go/no-go dựa trên evidence</li>
<li>Tóm tắt health status</li>
<li>Kết quả benchmark (nếu có)</li>
<li>Danh sách incidents trong thời gian pilot</li>
<li>Thời gian pilot và số server</li>
</ul>

<h3>Ví Dụ</h3>
<pre><code>$ kiro-cli pilot report \
  --server-count 3 \
  --started-at "2024-01-01T00:00:00Z" \
  --ended-at "2024-01-15T00:00:00Z" \
  --health-file /tmp/health.json \
  --benchmark-file /tmp/bench.json \
  --incident-dir /var/log/kiro/incidents \
  --output-dir /tmp/pilot-report

{
  "decision": "go",
  "server_count": 3,
  "duration_days": 14,
  "health_summary": {"healthy": 3, "degraded": 0, "unhealthy": 0},
  "incidents_total": 1,
  "critical_incidents": 0
}</code></pre>

<h3>Mã Thoát</h3>
<table>
<tr><th>Mã</th><th>Ý Nghĩa</th></tr>
<tr><td>0</td><td>Thành công — báo cáo pilot được tạo</td></tr>
<tr><td>1</td><td>Lỗi — không thể đọc evidence files hoặc ghi output</td></tr>
<tr><td>2</td><td>Thiếu tham số bắt buộc (--server-count, --started-at, --ended-at)</td></tr>
</table>

<h3>Liên Quan</h3>
<ul>
<li><a href="/docs/vi/cli/incident">kiro-cli incident report</a> — tạo incident reports làm input</li>
<li><a href="/docs/vi/cli/health">kiro-cli health</a> — tạo health file làm input</li>
</ul>
`

var viCLIReport template.HTML = `<h2>kiro-cli report</h2>
<p>Tạo báo cáo tổng hợp hệ thống bao gồm thông tin phiên bản, cấu hình runtime, và trạng thái các thành phần.</p>

<h3>Cú Pháp</h3>
<pre><code>kiro-cli report [--config &lt;path&gt;]</code></pre>

<h3>Tham Số</h3>
<table>
<tr><th>Tham Số</th><th>Bắt Buộc</th><th>Kiểu</th><th>Mặc Định</th><th>Mô Tả</th></tr>
<tr><td><code>--config</code></td><td>Không</td><td>string</td><td>/etc/kiro/kiro.yaml</td><td>Đường dẫn file cấu hình (mặc định: /etc/kiro/kiro.yaml)</td></tr>
</table>

<h3>Output JSON</h3>
<table>
<tr><th>Trường</th><th>Kiểu</th><th>Mô Tả</th></tr>
<tr><td><code>version</code></td><td>string</td><td>Phiên bản Kiro WAF</td></tr>
<tr><td><code>go_version</code></td><td>string</td><td>Phiên bản Go runtime</td></tr>
<tr><td><code>os</code></td><td>string</td><td>Hệ điều hành (ví dụ: linux)</td></tr>
<tr><td><code>arch</code></td><td>string</td><td>Kiến trúc CPU (ví dụ: amd64)</td></tr>
<tr><td><code>cpu_cores</code></td><td>int</td><td>Số CPU cores</td></tr>
<tr><td><code>memory_mb</code></td><td>int</td><td>Bộ nhớ sử dụng (MB)</td></tr>
<tr><td><code>goroutines</code></td><td>int</td><td>Số goroutines đang chạy</td></tr>
<tr><td><code>mode</code></td><td>string</td><td>Chế độ hoạt động</td></tr>
<tr><td><code>uptime</code></td><td>string</td><td>Thời gian hoạt động</td></tr>
</table>

<h3>Ví Dụ</h3>
<pre><code>$ kiro-cli report
{
  "version": "1.0.0",
  "go_version": "go1.22.0",
  "os": "linux",
  "arch": "amd64",
  "cpu_cores": 4,
  "memory_mb": 128,
  "goroutines": 42,
  "mode": "full",
  "uptime": "72h15m",
  "config": {
    "license_key": "***masked***",
    "master_url": "https://firewall.vpsgen.com",
    "sites_count": 2
  }
}</code></pre>

<h3>Mã Thoát</h3>
<table>
<tr><th>Mã</th><th>Ý Nghĩa</th></tr>
<tr><td>0</td><td>Thành công — báo cáo được hiển thị</td></tr>
<tr><td>1</td><td>Lỗi — không thể đọc cấu hình hoặc thu thập thông tin</td></tr>
<tr><td>2</td><td>Lỗi đọc cấu hình</td></tr>
</table>

<h3>Liên Quan</h3>
<ul>
<li><a href="/docs/vi/cli/status">kiro-cli status</a> — trạng thái runtime ngắn gọn</li>
<li><a href="/docs/vi/cli/health">kiro-cli health</a> — kiểm tra sức khỏe</li>
</ul>
`
