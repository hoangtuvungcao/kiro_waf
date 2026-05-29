# SETUP

Đây là tài liệu cài đặt tổng hợp. Dự án hiện tách rõ hai runtime:

- **Master Server**: domain `firewall.vpsgen.com`, quản lý license, dashboard,
  heartbeat/report API, artifact update.
- **Client Node**: server được bảo vệ, chạy XDP/eBPF và Golang WAF, xác thực
  license với Master rồi proxy traffic sạch về backend.

## Đọc Theo Thứ Tự

1. [Project Structure](PROJECT_STRUCTURE.md)
2. [Setup Master](SETUP_MASTER.md)
3. [Setup Client](SETUP_CLIENT.md)
4. [Phase Roadmap](../PHASES.md)
5. [Production Gap Analysis](vi/40-production-gap-analysis.md)

## Quick Smoke Local

```bash
go test ./...
go build -o /tmp/kiro-master ./master-server
go build -o /tmp/kiro-client-waf ./client-node
bash scripts/build-xdp.sh client-node/xdp_filter.c /tmp/xdp_filter.o
bash scripts/ci-phase10-smoke.sh
```

## Quick Deploy Master

Trên VPS Ubuntu 22.04:

```bash
cd /opt/kiro_waf
sudo KIRO_MASTER_DOMAIN=firewall.vpsgen.com \
  KIRO_MASTER_INSTALL_CLIENT=true \
  KIRO_MASTER_ADMIN_KEY='REPLACE_WITH_PRIVATE_ADMIN_KEY' \
  bash ./deploy_master.sh /opt/kiro_waf
```

Script cài:

- `/usr/local/bin/kiro-master`
- `/etc/kiro-master/master.env`
- `/var/lib/kiro-master/master.db`
- `/etc/systemd/system/kiro-master.service`
- `/etc/systemd/system/kiro-client-waf.service` nếu bật all-in-one client
- `/etc/nginx/sites-available/firewall.conf`
- `/usr/lib/kiro/xdp/xdp_filter.o`

## Quick Deploy Client

Trên server được bảo vệ:

```bash
go build -o build/client/kiro-client-waf ./client-node
bash scripts/build-xdp.sh client-node/xdp_filter.c build/client/xdp_filter.o
sudo install -D -m 0755 build/client/kiro-client-waf /usr/local/bin/kiro-client-waf
sudo install -D -m 0644 build/client/xdp_filter.o /usr/lib/kiro/xdp/xdp_filter.o
sudo install -D -m 0644 client-node/systemd/kiro-client-waf.service /etc/systemd/system/kiro-client-waf.service
```

Sau đó tạo `/etc/kiro-client/client.env` theo [Setup Client](SETUP_CLIENT.md).
Master có thể publish release metadata để client kiểm tra update qua
`/api/v1/update/check`; tự động thay binary chỉ chạy khi client được cấu hình
`KIRO_AUTO_UPDATE_COMMAND`.

## Giới Hạn Cần Nói Rõ

Code đã có đường chạy thật cho XDP, WAF, license heartbeat và dashboard. Tuy
nhiên các mục tiêu như 50Gbps hoặc 10 triệu request/giây phải được chứng minh
bằng benchmark trên hạ tầng phù hợp. Một VPS không thể chống lưu lượng vượt
uplink/provider nếu không có upstream filtering hoặc scrubbing.
