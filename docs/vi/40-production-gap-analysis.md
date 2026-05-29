# Phân Tích Gap Production

Tài liệu này chốt trạng thái hiện tại sau Phase 19 để chuẩn bị test trên VPS và
không nhầm giữa "đã có lab path" với "đã sẵn sàng bán production".

## Kết Luận Ngắn

Trạng thái hiện tại: **pilot/lab ready**. Dự án đã có nền tảng kỹ thuật đủ để
chạy thử có kiểm soát trên VPS Ubuntu 22.04, gồm config, license, update ký số,
firewall/proxy/WAF/bot generator, runtime diagnostics, installer lab, benchmark
lab, provider file API và XDP/eBPF lab attach.

Chưa nên claim **production enterprise/school ready** cho khách hàng thật trước
khi hoàn thành các gate còn lại: VPS proof, benchmark tải thật, vận hành update
theo systemd, quản lý BPF maps production, RBAC/rate-limit provider và quy trình
pilot tối thiểu 30 ngày.

## Đã Hoàn Thành

- Config runtime đọc được mode `server`/`full`, protection, paths, license,
  update và XDP.
- Firewall nftables có dry-run, apply lab, rollback timer và confirm.
- Proxy/Nginx có generator, WAF/Bot artifacts, challenge cookie, apply lab,
  validate/reload và rollback.
- Update manifest/artifact có checksum, chữ ký Ed25519, download qua file/HTTP,
  apply lab và rollback theo health command.
- License có issue, verify, rebind, signed revocation list và sync command.
- Provider file API có health/update/revocation/license/incident endpoint, bearer
  token tùy chọn, audit log và retention purge.
- Runtime diagnostics có status, health, preflight, support bundle, incident
  report và provider inbox export dạng file.
- Installer có plan, stage-lab, apply-lab, uninstall apply-lab với guard Ubuntu
  và ACK.
- XDP/eBPF có source C, build script, plan, apply/detach/confirm lab, root guard
  và health rollback.
- CI smoke Phase 10 bao phủ các luồng chính, không apply firewall/proxy/XDP thật.

## Thiếu Trước Khi Production Thật

| Nhóm | Trạng thái | Việc cần làm |
| --- | --- | --- |
| VPS proof | Thiếu bằng chứng host thật | Upload repo lên VPS, build, test, preflight, benchmark local, lưu artifact |
| Benchmark tải | Chưa đo PPS/conntrack/CPU-RAM dưới traffic thật | Dùng lab cô lập có traffic generator, không claim public capacity trước khi đo |
| XDP production | Có source/object/apply lab và map sync CLI | Thêm daemon watch/scheduler, pinned-map strategy và stats export liên tục |
| Installer production | Có lab-gated apply | Thêm wizard/upgrade path, package dependency install, backup/rollback tự động |
| Update production | Có apply lab | Thêm service update runner, systemd restart/health thật và scheduler |
| Revocation | Có sync command | Thêm timer/daemon định kỳ, retry/backoff và alert khi sync lỗi |
| Provider API | Có file-backed API, scoped token và rate limit | Thêm portal, staged rollout dashboard và vận hành token/retention nhiều tenant |
| Runtime security | Đọc JSONL chuẩn hóa | Thêm collector auditd/eBPF thực tế hoặc tích hợp agent tail trực tiếp |
| Operations | Có runbook | Thêm backup, restore drill, incident rota, SLA và hardening checklist theo môi trường |

## Gate Cho Doanh Nghiệp/Trường Học

1. VPS smoke pass trên Ubuntu 22.04/24.04 với artifact lưu lại.
2. Preflight không có lỗi `fail`; warning phải có owner và mitigation.
3. Benchmark lab local pass và benchmark tải thật có số liệu riêng theo size VPS.
4. Firewall/proxy/XDP apply thật chỉ chạy trong lab có console fallback.
5. Update/revocation chạy theo timer hoặc service có log, retry và alert.
6. Provider key/private data có backup, phân quyền file và quy trình rotate.
7. Pilot 3-5 VPS tối thiểu 30 ngày có go/no-go report.

## Bước Tiếp Theo

- Dùng `scripts/vps-upload-smoke.sh` để upload và chạy smoke an toàn trên VPS.
- Nếu smoke pass, mở Phase 21 cho scheduler/update/revocation production.
- Nếu cần đo DDoS/XDP thật, chuẩn bị lab riêng với traffic generator và console
  ngoài băng trước khi attach XDP native/offload.
