# SETUP_MASTER

Tài liệu này cài Master Server cho domain `firewall.vpsgen.com` trên Ubuntu
22.04 LTS. Master chịu trách nhiệm trang chủ, Admin Dashboard, SQLite license
database, heartbeat/report API và phân phối artifact XDP cho client.

## 1. DNS

Tạo bản ghi:

```text
firewall.vpsgen.com.  A  103.77.246.198
```

Kiểm tra:

```bash
dig +short firewall.vpsgen.com
```

## 2. Chuẩn Bị Source

Trên VPS:

```bash
sudo mkdir -p /opt/kiro_waf
sudo chown "$USER":"$USER" /opt/kiro_waf
cd /opt/kiro_waf
```

Đưa source dự án vào thư mục này bằng `git clone`, `rsync` hoặc script upload
nội bộ của dự án.

## 3. Cài Tự Động

Không ghi admin key vào source. Truyền qua biến môi trường hoặc để script tự
tạo key mạnh và lưu trong `/etc/kiro-master/master.env`.

```bash
cd /opt/kiro_waf
sudo KIRO_MASTER_DOMAIN=firewall.vpsgen.com \
  KIRO_MASTER_INSTALL_CLIENT=true \
  KIRO_ADMIN_ALLOW_CIDRS='YOUR_ADMIN_IP/32' \
  KIRO_MASTER_ADMIN_KEY='REPLACE_WITH_PRIVATE_ADMIN_KEY' \
  bash ./deploy_master.sh /opt/kiro_waf
```

Script thực hiện:

- cài `nginx`, `clang`, `llvm`, `libelf-dev`, `sqlite3`, toolchain build;
- cài Go theo version trong `go.mod` nếu host chưa có Go;
- build `/usr/local/bin/kiro-master` từ `master-server/main.go`;
- build `/usr/local/bin/kiro-client-waf` nếu bật all-in-one client;
- build `/usr/lib/kiro/xdp/xdp_filter.o` từ `client-node/xdp_filter.c`;
- tạo user system `kiro-master`;
- tạo user system `kiro-client` và license local cho VPS nếu bật all-in-one;
- tạo SQLite database tại `/var/lib/kiro-master/master.db`;
- ghi env bảo mật tại `/etc/kiro-master/master.env`;
- ghi env client tại `/etc/kiro-client/client.env`;
- cài systemd unit `/etc/systemd/system/kiro-master.service`;
- cài systemd unit `/etc/systemd/system/kiro-client-waf.service`;
- cài Nginx site `/etc/nginx/sites-available/firewall.conf`.
- ghi `/etc/nginx/kiro-admin-allow.conf` để khóa Admin API
  `/api/v1/admin/` theo IP quản trị.

Mô hình all-in-one mặc định:

```text
Nginx public :80
  -> kiro-client-waf 127.0.0.1:8090 cho / và /admin/
  -> kiro-master     127.0.0.1:8080 cho /api/ và /healthz

kiro-client-waf
  -> backend kiro-master 127.0.0.1:8080
  -> heartbeat/update check trực tiếp 127.0.0.1:8080
```

## 4. Kiểm Tra Dịch Vụ

```bash
sudo systemctl status kiro-master --no-pager
sudo systemctl status kiro-client-waf --no-pager
sudo systemctl status nginx --no-pager
curl -fsS http://127.0.0.1:8080/healthz
curl -fsS -A 'Mozilla/5.0 KiroHealth' http://127.0.0.1:8090/healthz
curl -fsS -H 'Host: firewall.vpsgen.com' http://127.0.0.1/healthz
```

Trang chủ:

```text
http://firewall.vpsgen.com/
```

Admin Dashboard:

```text
http://firewall.vpsgen.com/admin/
```

Trang chủ không hiển thị link admin. Chỉ khi tự mở `/admin/` mới thấy form
login; sai admin key trả về `403 Forbidden`. Key hợp lệ nằm trong
`/etc/kiro-master/master.env`. Admin API `/api/v1/admin/` vẫn bị giới hạn bởi
`/etc/nginx/kiro-admin-allow.conf`.

Khi cần quản trị an toàn từ máy cá nhân, dùng một trong hai cách:

```bash
ssh -L 8081:127.0.0.1:80 root@103.77.246.198
```

Rồi mở `http://127.0.0.1:8081/admin/`, hoặc thêm IP quản trị vào:

```bash
sudoedit /etc/nginx/kiro-admin-allow.conf
sudo nginx -t && sudo systemctl reload nginx
```

## 5. HTTPS

Sau khi DNS đã trỏ đúng, bật TLS:

```bash
sudo apt-get install -y certbot python3-certbot-nginx
sudo certbot --nginx -d firewall.vpsgen.com
```

Kiểm tra gia hạn:

```bash
sudo certbot renew --dry-run
```

## 6. API Quản Trị License

Admin API dùng header `X-Admin-Key` hoặc `Authorization: Bearer`.

Tạo license:

```bash
ADMIN_KEY="$(sudo sed -n 's/^KIRO_MASTER_ADMIN_KEY=//p' /etc/kiro-master/master.env)"
curl -sS -X POST http://127.0.0.1:8080/api/v1/admin/licenses \
  -H "X-Admin-Key: ${ADMIN_KEY}" \
  -H 'Content-Type: application/json' \
  -d '{
    "customer_id": "school_001",
    "customer_name": "Truong THPT Mau",
    "client_ip": "203.0.113.10",
    "fingerprint_hash": "sha256:REPLACE_WITH_CLIENT_FINGERPRINT",
    "plan": "school_smb",
    "valid_days": 365
  }'
```

Kết quả trả về `license_key` một lần duy nhất. Master chỉ lưu HMAC hash nên nếu
mất key plaintext thì phải tạo key mới.

Liệt kê:

```bash
curl -sS http://127.0.0.1:8080/api/v1/admin/licenses \
  -H "X-Admin-Key: ${ADMIN_KEY}"
```

Gia hạn:

```bash
curl -sS -X POST http://127.0.0.1:8080/api/v1/admin/licenses/lic_xxx/renew \
  -H "X-Admin-Key: ${ADMIN_KEY}" \
  -H 'Content-Type: application/json' \
  -d '{"valid_days":365}'
```

Thu hồi:

```bash
curl -sS -X POST http://127.0.0.1:8080/api/v1/admin/licenses/lic_xxx/revoke \
  -H "X-Admin-Key: ${ADMIN_KEY}"
```

Xóa:

```bash
curl -sS -X DELETE http://127.0.0.1:8080/api/v1/admin/licenses/lic_xxx \
  -H "X-Admin-Key: ${ADMIN_KEY}"
```

Chỉnh sửa license:

```bash
curl -sS -X POST http://127.0.0.1:8080/api/v1/admin/licenses/lic_xxx/update \
  -H "X-Admin-Key: ${ADMIN_KEY}" \
  -H 'Content-Type: application/json' \
  -d '{
    "customer_id":"school_001",
    "customer_name":"Truong THPT Mau",
    "client_ip":"203.0.113.10",
    "fingerprint_hash":"sha256:REPLACE_WITH_CLIENT_FINGERPRINT",
    "plan":"enterprise",
    "expires_at":"2030-01-01",
    "revoked":false
  }'
```

Rotate key khi nghi bị lộ:

```bash
curl -sS -X POST http://127.0.0.1:8080/api/v1/admin/licenses/lic_xxx/rotate \
  -H "X-Admin-Key: ${ADMIN_KEY}"
```

Kết quả trả về key mới một lần duy nhất; client phải cập nhật
`KIRO_LICENSE_KEY`.

## 7. Release Và Auto-Update Metadata

Publish metadata bản mới:

```bash
curl -sS -X POST http://127.0.0.1:8080/api/v1/admin/releases \
  -H "X-Admin-Key: ${ADMIN_KEY}" \
  -H 'Content-Type: application/json' \
  -d '{
    "component":"kiro-client-waf",
    "channel":"stable",
    "version":"1.1.0",
    "artifact_url":"https://firewall.vpsgen.com/releases/kiro-client-waf_1.1.0.tar.gz",
    "sha256":"REPLACE_WITH_64_HEX_SHA256",
    "notes":"Bản ổn định mới"
  }'
```

Client gọi `/api/v1/update/check`. Mặc định client chỉ log bản mới. Nếu muốn tự
áp dụng update, cấu hình `KIRO_AUTO_UPDATE_COMMAND` trên client để tải artifact,
verify SHA256, thay binary và reload service theo runbook nội bộ.

## 8. API Cho Client Node

Client gửi heartbeat định kỳ:

```http
POST /api/v1/heartbeat
Content-Type: application/json
```

Payload:

```json
{
  "license_key": "KIRO-...",
  "client_ip": "203.0.113.10",
  "fingerprint_hash": "sha256:...",
  "node_id": "edge-01",
  "stats": {
    "locked": false,
    "timestamp": "2026-05-29T00:00:00Z"
  }
}
```

Master từ chối nếu key sai, hết hạn, bị thu hồi, IP khai báo khác IP đã bind,
IP quan sát qua Nginx khác IP đã bind, hoặc fingerprint không khớp.

## 9. Cấu Trúc Runtime

```text
master-server/
  main.go                         Master HTTP server + SQLite
  nginx/firewall.conf             Nginx reverse proxy
  systemd/kiro-master.service     systemd unit

/etc/kiro-master/master.env       secret runtime env
/var/lib/kiro-master/master.db     SQLite database
/usr/local/bin/kiro-master         compiled binary
/usr/local/bin/kiro-client-waf     all-in-one protected frontend
/usr/lib/kiro/xdp/xdp_filter.o     compiled XDP artifact
```

## 10. Ghi Chú Production

XDP giúp drop sớm ở NIC/kernel path, nhưng không thể bảo đảm một VPS đơn lẻ chịu
50Gbps nếu uplink hoặc provider đã nghẽn trước khi packet tới máy. L7 WAF cũng
không thể cam kết 10 triệu request/giây trên một node nếu không benchmark bằng
traffic generator, horizontal scaling và upstream DDoS scrubbing. Dùng các con
số này làm mục tiêu kiến trúc, không dùng làm cam kết vận hành khi chưa có số đo.
