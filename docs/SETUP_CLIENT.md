# SETUP_CLIENT

Tài liệu này cài Client Node trên server được bảo vệ. Client nhận traffic thật,
chạy XDP/eBPF ở L3/L4, chạy Golang WAF ở L7, xác thực license với Master Server
và proxy traffic sạch về backend của khách hàng.

## 1. Mô Hình Luồng Traffic

```text
Internet
  -> NIC / XDP xdp_filter.o
  -> Nginx hoặc port public
  -> kiro-client-waf :8090
  -> backend khách hàng :3000/:8080/...
```

Master Server:

```text
https://firewall.vpsgen.com
```

## 2. Cài Package

Ubuntu 22.04:

```bash
sudo apt-get update
sudo apt-get install -y build-essential clang llvm libelf-dev curl ca-certificates nginx
```

Cần Go đúng version trong `go.mod`.

## 3. Build Binary Và XDP Object

Trong source dự án:

```bash
go build -trimpath -ldflags "-s -w" -o build/client/kiro-client-waf ./client-node
bash scripts/build-xdp.sh client-node/xdp_filter.c build/client/xdp_filter.o

sudo install -D -m 0755 build/client/kiro-client-waf /usr/local/bin/kiro-client-waf
sudo install -D -m 0644 build/client/xdp_filter.o /usr/lib/kiro/xdp/xdp_filter.o
sudo install -D -m 0644 client-node/systemd/kiro-client-waf.service /etc/systemd/system/kiro-client-waf.service
```

## 4. Tạo Fingerprint Client

Client WAF tự tính fingerprint từ machine-id, hostname và MAC. Để xem fingerprint
trước khi cấp license, có thể chạy tạm với key giả để đọc lỗi/log hoặc dùng CLI
cũ của dự án:

```bash
kiro-cli license fingerprint --salt default-provider-key-2026
```

Gửi fingerprint và IP public của client cho Master để tạo license.

## 5. Cấu Hình Client

Tạo file env:

```bash
sudo install -d -m 0750 /etc/kiro-client
sudo install -d -m 0755 /etc/kiro /var/lib/kiro /var/log/kiro
sudo tee /etc/kiro-client/client.env >/dev/null <<'EOF'
KIRO_CLIENT_LISTEN=127.0.0.1:8090
KIRO_BACKEND_URL=http://127.0.0.1:3000
KIRO_MASTER_URL=https://firewall.vpsgen.com
KIRO_LICENSE_KEY=REPLACE_WITH_LICENSE_KEY_FROM_MASTER
KIRO_PUBLIC_IP=REPLACE_WITH_CLIENT_PUBLIC_IP
KIRO_NODE_ID=edge-01
KIRO_CLIENT_COOKIE_SECRET=REPLACE_WITH_RANDOM_48_BYTES
KIRO_CLIENT_VERSION=1.0.0
KIRO_UPDATE_COMPONENT=kiro-client-waf
KIRO_UPDATE_CHANNEL=stable
KIRO_UPDATE_SECONDS=300
KIRO_AUTO_UPDATE_COMMAND=
KIRO_XDP_BLOCKLIST_FILE=/var/lib/kiro/xdp-blocklist.txt
KIRO_AGENT_BINARY=/usr/local/bin/kiro-agent
KIRO_CONFIG=/etc/kiro/kiro.yaml
KIRO_RPM_PER_IP=120
KIRO_SUBNET_RPM=1800
KIRO_HARD_BLOCK_AFTER=360
KIRO_BLOCK_TTL_SECONDS=900
KIRO_POW_DIFFICULTY=4
KIRO_HOLD_SECONDS=2
KIRO_HEARTBEAT_SECONDS=30
KIRO_XDP_SYNC=true
KIRO_CLIENT_LOCKDOWN_XDP=false
EOF
sudo chmod 0640 /etc/kiro-client/client.env
```

`KIRO_CLIENT_LOCKDOWN_XDP=false` là mặc định an toàn. Chỉ bật `true` sau khi đã
có console ngoài băng hoặc snapshot, vì license lỗi có thể block toàn bộ traffic.

## 6. Chạy WAF Reverse Proxy

```bash
sudo useradd --system --home-dir /var/lib/kiro --shell /usr/sbin/nologin kiro-client || true
sudo chown -R kiro-client:kiro-client /var/lib/kiro /var/log/kiro
sudo systemctl daemon-reload
sudo systemctl enable --now kiro-client-waf
sudo systemctl status kiro-client-waf --no-pager
curl -fsS -A 'Mozilla/5.0 KiroHealth' http://127.0.0.1:8090/
```

Nếu backend chưa chạy ở `KIRO_BACKEND_URL`, WAF trả `502 backend unavailable`.

## 7. Nginx Public Proxy

Ví dụ Nginx site để website đi qua WAF:

```nginx
server {
    listen 80;
    server_name example.com www.example.com;

    location / {
        proxy_pass http://127.0.0.1:8090;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Kiểm tra:

```bash
sudo nginx -t
sudo systemctl reload nginx
```

## 8. JS Challenge Và Hold Captcha

Client WAF xử lý nội bộ:

- request nghi vấn nhận `/__kiro/challenge` và phải giải Proof-of-Work bằng
  JavaScript trước khi nhận cookie `kiro_waf`;
- request vượt rate limit nhận `/__kiro/hold` và phải nhấn giữ đủ số giây;
- User-Agent automation rõ ràng như curl, python-requests, sqlmap bị block L7
  ngay; request lặp lại cùng IP bị ghi vào blocklist để XDP sync;
- IP vượt `KIRO_RPM_PER_IP` hoặc `KIRO_HARD_BLOCK_AFTER` bị ghi vào blocklist;
- subnet `/24` vượt `KIRO_SUBNET_RPM` bị ép challenge để giảm botnet nhỏ;
- sau khi xác thực, cookie HMAC theo IP có hiệu lực 20 phút;
- bot/HTTP flood vượt ngưỡng được ghi vào `KIRO_XDP_BLOCKLIST_FILE`.

Không dùng Cloudflare Turnstile, reCAPTCHA hoặc API chống bot bên ngoài.

## 9. Auto-Update

Client kiểm tra bản mới qua Master:

```text
POST https://firewall.vpsgen.com/api/v1/update/check
```

Payload gồm license, fingerprint, component, channel và version hiện tại. Nếu
Master có release mới hơn, client log artifact URL và SHA256.

Mặc định client không tự thay binary. Để bật tự động, đặt:

```bash
KIRO_AUTO_UPDATE_COMMAND=/usr/local/lib/kiro/client-auto-update.sh
```

Lệnh CLI cài sẵn:

```bash
sudo kiro-client-update
```

CLI này gọi Master, tải artifact nếu có bản mới, kiểm SHA256, thay binary
`kiro-client-waf`, restart service và rollback nếu health check fail. Biến
`KIRO_AUTO_UPDATE_COMMAND` chỉ dùng khi muốn client tự chạy lệnh update định kỳ
sau khi phát hiện bản mới.

## 10. XDP L3/L4

Object XDP nằm tại:

```text
/usr/lib/kiro/xdp/xdp_filter.o
```

XDP hiện có:

- IPv4 allowlist/blocklist bằng BPF LPM trie;
- rate-limit theo source IP và subnet `/24`;
- SYN/UDP/ICMP per-source thresholds;
- drop malformed TCP flags, IPv4 fragment khi bật config;
- validate IPv4 total length và UDP length;
- map `udp_src_port_blocklist` cho UDP reflection/amplification source ports.

Attach qua agent hiện có của dự án:

```bash
sudo KIRO_LAB_XDP_APPLY=1 \
  KIRO_CONFIG=/etc/kiro/kiro.yaml \
  KIRO_XDP_INTERFACE=eth0 \
  KIRO_XDP_MODE=generic \
  KIRO_XDP_CONFIRM=0 \
  bash scripts/vps-xdp-generic-apply-lab.sh /opt/kiro_waf
```

Sau khi SSH và health check ổn định:

```bash
sudo kiro-agent --config /etc/kiro/kiro.yaml --xdp-confirm --xdp-state-dir /var/lib/kiro
```

Sync blocklist sang BPF maps khi đã chuẩn bị quyền root và ACK:

```bash
sudo KIRO_LAB_XDP_MAP_SYNC=1 kiro-agent \
  --config /etc/kiro/kiro.yaml \
  --xdp-sync-maps-lab \
  --xdp-map-blocklist-file /var/lib/kiro/xdp-blocklist.txt \
  --xdp-map-ack KIRO_LAB_XDP_MAP_SYNC
```

## 11. License Heartbeat

Client gửi heartbeat mỗi `KIRO_HEARTBEAT_SECONDS`:

```text
POST https://firewall.vpsgen.com/api/v1/heartbeat
```

Nếu Master trả invalid, client khóa L7 proxy và trả `503`. Nếu bật
`KIRO_CLIENT_LOCKDOWN_XDP=true`, client còn thêm `0.0.0.0/0` vào blocklist để
đóng traffic ở XDP.

## 12. Troubleshooting

Log WAF:

```bash
sudo journalctl -u kiro-client-waf -f
```

Kiểm tra license từ client:

```bash
curl -sS -X POST https://firewall.vpsgen.com/api/v1/license/check \
  -H 'Content-Type: application/json' \
  -d '{
    "license_key":"KIRO-...",
    "client_ip":"CLIENT_PUBLIC_IP",
    "fingerprint_hash":"sha256:...",
    "node_id":"edge-01"
  }'
```

Các lỗi thường gặp:

- `license_not_found`: key sai hoặc Master dùng signing secret khác.
- `client_ip_mismatch`: `KIRO_PUBLIC_IP` khác IP bind trong license.
- `observed_ip_mismatch`: request heartbeat đi ra bằng IP khác IP license.
- `fingerprint_mismatch`: client đã đổi máy, NIC hoặc machine-id.
- `backend unavailable`: backend app phía sau WAF chưa chạy.
