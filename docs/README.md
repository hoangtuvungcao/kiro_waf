# Kiro WAF Documentation Index

Index này gom lại tài liệu chính để README không phải giữ danh sách quá dài.
Nếu cần biết dự án đang ở đâu và bước tiếp theo là gì, đọc theo thứ tự dưới
đây.

## Start Here

- [Phase roadmap and progress](../PHASES.md): trạng thái phase/task và bằng
  chứng test gần nhất.
- [Setup guide](SETUP.md): cài Management Server, Client Node, XDP và API.
- [Setup Master](SETUP_MASTER.md): cài `firewall.vpsgen.com`, Nginx, systemd,
  SQLite license DB và Admin/API.
- [Setup Client](SETUP_CLIENT.md): cài WAF reverse proxy, heartbeat license và
  XDP object trên server được bảo vệ.
- [Project structure](PROJECT_STRUCTURE.md): cấu trúc thư mục và ranh giới
  Management Server/Client Node.
- [Production gap analysis](vi/40-production-gap-analysis.md): phần đã xong,
  phần còn thiếu trước khi claim production doanh nghiệp/trường học.
- [VPS Ubuntu 22.04 test runbook](vi/41-vps-ubuntu-2204-test-runbook.md): upload,
  build, smoke và benchmark an toàn trên VPS.
- [Production hardening runbook](vi/42-production-hardening-runbook.md): tách
  provider/protected server, XDP map sync, domain và giới hạn DDoS thực tế.
- [VPS homepage/provider-client runbook](vi/43-vps-homepage-provider-client-runbook.md):
  cài trang chủ, issue license cho VPS và replace XDP có rate-limit.
- [Management Server enterprise runbook](vi/44-enterprise-management-server-runbook.md):
  API license, dashboard và setup `firewall.vpsgen.com`.
- [XDP/eBPF VPS runbook](vi/39-xdp-ebpf-vps-runbook.md): build object, plan,
  attach lab có ACK và rollback.

## Core Vietnamese Docs

- [Tổng quan](vi/00-tong-quan.md)
- [Kiến trúc](vi/01-kien-truc.md)
- [Threat model và yêu cầu hệ thống](vi/13-threat-model-va-yeu-cau-he-thong.md)
- [Cấu trúc code và module](vi/14-cau-truc-code-va-module.md)
- [Production và thương mại readiness](vi/21-production-va-thuong-mai-readiness.md)
- [PRD/SRS sản phẩm](vi/22-prd-srs-san-pham.md)
- [Runbook preflight, status và health](vi/29-preflight-status-health-runbook.md)
- [Runbook benchmark lab](vi/30-benchmark-runbook.md)
- [Runbook installer và uninstall lab](vi/31-installer-runbook.md)
- [Runbook provider file API](vi/37-provider-file-api-runbook.md)
- [Runbook pilot go/no-go](vi/38-pilot-go-no-go-runbook.md)

## English Mirrors

- [Production gap analysis](en/33-production-gap-analysis.md)
- [VPS Ubuntu 22.04 test runbook](en/34-vps-ubuntu-2204-test-runbook.md)
- [Production hardening runbook](en/35-production-hardening-runbook.md)
- [VPS homepage/provider-client runbook](en/36-vps-homepage-provider-client-runbook.md)
- [XDP/eBPF VPS runbook](en/32-xdp-ebpf-vps-runbook.md)
- [Production and commercial readiness](en/15-production-commercial-readiness.md)
- [Provider file API runbook](en/30-provider-file-api-runbook.md)
- [Pilot go/no-go runbook](en/31-pilot-go-no-go-runbook.md)

## Notes

- Lab/apply commands that can change firewall, proxy, systemd or XDP state are
  intentionally guarded by explicit ACK flags.
- Current safe VPS smoke builds binaries, checks config, runs tests and local
  benchmark. It does not attach XDP or replace host firewall rules.
