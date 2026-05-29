# Kế Hoạch Khởi Tạo Và Test Từng Bước

## Nguyên tắc

Không khởi tạo tất cả cùng lúc. Làm từng phase nhỏ, mỗi phase phải chạy được,
test được, rollback được rồi mới sang phase tiếp theo.

Không test DDoS trên mạng public hoặc IP không sở hữu. Traffic attack chỉ được
test trong lab/local.

## Phase 0: Khởi tạo repo code

Mục tiêu:

- Tạo project Go.
- Tạo `cmd/kiro-agent`, `cmd/kiro-cli`, `cmd/kiro-provider`.
- Tạo package `internal/shared/config`, `internal/shared/storage`,
  `internal/shared/licenseverify`.
- Tạo skeleton `internal/agent` và `internal/provider`.
- Thêm test đảm bảo `cmd/kiro-agent` không import `internal/provider`.
- CI local bằng `go test ./...`.

Test bắt buộc:

```text
go test ./...
kiro-agent --config configs/kiro.example.yaml --check
kiro-cli version
```

Done khi:

- Build được binary.
- Parse config pass.
- Config lỗi trả lỗi rõ.
- Ranh giới import agent/provider được kiểm tra.

## Phase 1: Config và file storage

Mục tiêu:

- Validate `mode`.
- Parse simple tenant config.
- Expand simple tenant config sang runtime config.
- Validate `sites/backend_pools`.
- Validate route reference.
- Atomic write state file.
- JSONL event writer.

Test bắt buộc:

- Config hợp lệ pass.
- Simple config hợp lệ pass.
- Simple config thiếu domain/backend fail rõ ràng.
- Simple config profile không tồn tại fail rõ ràng.
- Thiếu backend pool fail.
- Domain trùng fail.
- YAML sai fail.
- Atomic write không tạo file rỗng.

Done khi:

- Không module nào apply system change.
- Test pass ổn định.

## Phase 2: License local

Mục tiêu:

- Fingerprint server.
- Verify license JSON.
- Check signature.
- Check feature gate.
- Check expiry/grace.

Test bắt buộc:

- License hợp lệ pass.
- Signature sai fail.
- Machine binding sai fail.
- Mode không được license cho phép fail.
- License hết hạn vào grace đúng.
- Nếu tắt grace period, license hết hạn phải fail ngay.

Done khi:

- Agent không bật feature thương mại nếu license không hợp lệ.
- Agent chỉ verify license, không có code issue/sign license.
- `kiro-cli license fingerprint --salt <salt_id>` tạo được fingerprint hash cho
  kích hoạt offline.
- Agent đọc `license.file`, `license.provider_public_key` và
  `server_identity.fingerprint_salt_id` từ config nâng cao; flag CLI chỉ dùng
  để override/debug.

## Phase 2.5: Provider license server skeleton

Mục tiêu:

- `kiro-provider` đọc `configs/provider.example.yaml`.
- Sinh key dev/test.
- Issue license test.
- Lưu provider-data bằng file storage.
- Không import module agent firewall/ebpf/proxy.
- Export `license.json` và `provider-public-key.pem` cho agent test, không export
  private key.

Test bắt buộc:

- Provider issue license hợp lệ.
- Agent verify license do provider sinh.
- Provider private key không xuất hiện trong thư mục agent test fixture.
- `cmd/kiro-provider` không yêu cầu root/network firewall capabilities.
- CLI provider có lệnh `gen-dev-keys` và `issue-test-license`.

Done khi:

- Provider và agent trao đổi license qua file/API giả lập.
- Vai trò không bị gộp bằng config.

## Phase 3: Firewall manager dry-run

Mục tiêu:

- Generate nftables từ config.
- Dry-run.
- Không apply thật ở phase này.
- Kiểm tra admin allowlist.

Test bắt buộc:

- Không có admin IP thì fail nếu `ssh_admin_only=true`.
- Generate server mode.
- Generate full mode.
- Cloudflare IPv4/IPv6 được đưa vào full mode.
- Direct origin drop rule tồn tại khi bật Cloudflare.

Done khi:

- Output nftables deterministic.
- Có snapshot last good giả lập.
- CLI dry-run:

```text
kiro-agent --config configs/kiro.example.yaml --firewall-dry-run
kiro-agent --config configs/tenant.server-only.example.yaml --firewall-dry-run \
  --firewall-snapshot-dir /tmp/kiro-firewall-snapshot
```

## Phase 4: Firewall apply an toàn

Mục tiêu:

- Apply nftables thật trên lab Ubuntu.
- Rollback timer.
- Confirm apply.
- Restore last good.

Test bắt buộc:

- Apply rule hợp lệ.
- Apply rule lỗi rollback.
- Không khóa SSH admin trong lab.
- Dynamic temp ban hoạt động.

Done khi:

- Có script lab.
- Có runbook recovery.
- CLI apply thật bị khóa bằng `--firewall-lab-ack KIRO_LAB_FIREWALL_APPLY`.
- Có pending rollback state và lệnh confirm/rollback.

## Phase 5: Proxy generator

Mục tiêu:

- Generate Nginx config từ `sites/backend_pools/routes`.
- Validate bằng `nginx -t`.
- Chỉ dry-run/validate ở phase này, chưa reload thật.
- Reload an toàn và rollback reload chuyển sang phase sau.

Test bắt buộc:

- 1 domain nhiều backend.
- Nhiều domain một backend.
- 1 domain nhiều route/backend.
- Route quota xuất hiện đúng.
- Cloudflare real IP include đúng.
- `flexible_http` generate Nginx listen 80, không yêu cầu cert/key.
- `full_strict` generate Nginx listen 443 ssl và yêu cầu cert/key.

Done khi:

- Proxy config sinh deterministic.
- Có validate hook bằng `nginx -t`.
- Có runbook lab.

## Phase 6: Governor và overload mode

Mục tiêu:

- Thu thập CPU/RAM/load/conntrack.
- Tính level.
- Hysteresis/cooldown.
- Emit event.

Test bắt buộc:

- CPU/RAM giả lập đẩy lên `ELEVATED`.
- Conntrack giả lập đẩy lên `ATTACK`.
- Recovery sample hạ mode đúng.
- Không nhảy mode liên tục.

Done khi:

- Mode switch có log và lý do rõ.
- CLI evaluate:

```text
kiro-agent --config configs/tenant.server-only.example.yaml --governor-evaluate \
  --sample-cpu-percent 86 --sample-ram-available-percent 50
```

## Phase 7: WAF và bot defense

Mục tiêu:

- Tích hợp Coraza/ModSecurity.
- Load OWASP CRS.
- Bot score.
- Cookie challenge.

Test bắt buộc:

- SQLi/XSS/path traversal bị detect.
- False positive route được exclude bằng policy.
- Bot không giữ cookie bị challenge/block.
- Client allowlist không bị challenge.

Done khi:

- Có test WAF/bot trong lab HTTP.
- CLI lab evaluate:

```text
kiro-agent --config configs/tenant.full-cloudflare.example.yaml \
  --web-defense-evaluate --web-path /login \
  --web-query "user=admin' OR 1=1--"
```

## Phase 8: Update và provider file storage

Mục tiêu:

- Provider issue license.
- Provider tạo update manifest.
- Agent verify update.
- Rollback update.

Test bắt buộc:

- Manifest sai chữ ký fail.
- Artifact sai checksum fail.
- Update crash rollback.
- Rebind license hoạt động.

Done khi:

- Có end-to-end activation + update trong lab.
- CLI lab verify/apply:

```text
kiro-agent --config configs/kiro.advanced.example.yaml \
  --provider-public-key /tmp/provider-public-key.pem \
  --update-verify \
  --update-manifest-file /tmp/kiro_1.0.1.json \
  --update-artifact-file /tmp/kiro-agent_linux_amd64.tar.gz \
  --update-current-version 1.0.0
```

## Phase 9: Runtime security và support bundle

Mục tiêu:

- File integrity.
- Process alert.
- Support bundle redact secret.
- Incident report.

Test bắt buộc:

- Web user chạy shell tạo alert.
- Sửa file config tạo alert.
- Support bundle không chứa token/password.

Done khi:

- Support bundle đủ để bảo hành mà không lộ secret.
- CLI lab scan/bundle:

```text
kiro-agent --config configs/kiro.advanced.example.yaml \
  --runtime-security-scan \
  --runtime-process-user www-data \
  --runtime-process-command bash \
  --runtime-alert-file /tmp/kiro-runtime-alerts.jsonl

kiro-agent --config configs/kiro.advanced.example.yaml \
  --support-bundle \
  --support-output-dir /tmp/kiro-support-bundle \
  --support-alert-file /tmp/kiro-runtime-alerts.jsonl
```

## Production gate bước đầu: preflight/status/health

Mục tiêu:

- CLI status phản hồi nhanh.
- Preflight kiểm tra môi trường lab trước khi apply thật.
- Health gộp config và preflight checks.
- Mode show/set thao tác được với file config lab.

CLI lab:

```text
kiro-cli status --config configs/tenant.full-cloudflare.example.yaml
kiro-cli preflight --config configs/tenant.full-cloudflare.example.yaml \
  --preflight-writable-root /tmp/kiro-preflight
kiro-cli health --config configs/tenant.full-cloudflare.example.yaml \
  --preflight-writable-root /tmp/kiro-preflight
```

## Production gate bước đầu: benchmark lab

Mục tiêu:

- Có report JSON cục bộ cho các khối đã triển khai.
- Phân biệt metric đã đo và metric chưa được claim.
- Không tạo traffic tấn công public.

CLI lab:

```text
kiro-agent --config configs/tenant.full-cloudflare.example.yaml \
  --benchmark-lab \
  --benchmark-iterations 5000 \
  --benchmark-temp-ban-size 5000 \
  --benchmark-output-file /tmp/kiro-benchmark.json
```

Done khi:

- Report có metric WAF, bot, nftables, temp-ban và proxy.
- `xdp_pps_drop`, `conntrack_pressure`, `cpu_ram_under_attack` vẫn là
  `not_measured` nếu chưa chạy lab cô lập riêng.
- Không dùng kết quả local làm claim chống DDoS public.

## Production gate bước đầu: installer/uninstall lab

Mục tiêu:

- Sinh install plan JSON.
- Stage file vào root lab thay vì ghi thẳng vào hệ thống thật.
- Có uninstall plan mặc định giữ dữ liệu và tùy chọn purge phá hủy.
- Kiểm tra systemd service artifact và install manifest.

CLI lab:

```text
mkdir -p /tmp/kiro-build
go build -o /tmp/kiro-build/kiro-agent ./cmd/kiro-agent
go build -o /tmp/kiro-build/kiro-cli ./cmd/kiro-cli

kiro-cli install plan \
  --config configs/tenant.full-cloudflare.example.yaml \
  --agent-binary /tmp/kiro-build/kiro-agent \
  --cli-binary /tmp/kiro-build/kiro-cli

kiro-cli install stage-lab \
  --config configs/tenant.full-cloudflare.example.yaml \
  --install-root /tmp/kiro-install-root \
  --agent-binary /tmp/kiro-build/kiro-agent \
  --cli-binary /tmp/kiro-build/kiro-cli

kiro-cli install uninstall-plan \
  --config configs/tenant.full-cloudflare.example.yaml
```

Done khi:

- Manifest có checksum cho config, binary và systemd service.
- Stage không ghi ra ngoài `--install-root`.
- Uninstall plan không purge config/state/log nếu chưa truyền `--purge`.

## Production gate bước đầu: agent event log

Mục tiêu:

- Governor/runtime event log ghi JSONL có rotation.
- Event lặp có rate limit để tránh spam log.
- Rate-limit state nằm cạnh event file hoặc theo `--event-rate-limit-state-file`.

CLI lab:

```text
kiro-agent --config configs/kiro.advanced.example.yaml \
  --runtime-security-scan \
  --runtime-process-user www-data \
  --runtime-process-command bash \
  --runtime-alert-file /tmp/kiro-runtime-alerts.jsonl \
  --event-log-max-bytes 1024 \
  --event-log-max-backups 2 \
  --event-rate-limit-window-seconds 60 \
  --event-rate-limit-max 1

kiro-agent --config configs/tenant.server-only.example.yaml \
  --governor-evaluate \
  --sample-cpu-percent 86 \
  --sample-ram-available-percent 50 \
  --governor-event-file /tmp/kiro-governor-events.jsonl \
  --event-log-max-bytes 1024 \
  --event-log-max-backups 2 \
  --event-rate-limit-window-seconds 60 \
  --event-rate-limit-max 1
```

Done khi:

- Event lặp trong cùng window không ghi thêm dòng JSONL.
- File rotate khi vượt giới hạn bytes.
- Support bundle vẫn đọc được alert JSONL đã redact.

## Commercial gate bước đầu: release management

Mục tiêu:

- Release manifest có changelog, migration note, rollback note.
- Artifact có checksum và chữ ký metadata artifact.
- Manifest ký số và có compatibility matrix.
- Agent có thể bật chế độ verify bắt buộc release metadata/artifact signature.

CLI lab:

```text
mkdir -p /tmp/kiro-release
go build -o /tmp/kiro-release/kiro-agent ./cmd/kiro-agent
tar -C /tmp/kiro-release -czf /tmp/kiro-release/kiro-agent_linux_amd64.tar.gz kiro-agent

kiro-provider --config configs/provider.example.yaml publish-release \
  --version 1.0.2 \
  --channel stable \
  --artifact-file /tmp/kiro-release/kiro-agent_linux_amd64.tar.gz \
  --min-agent-version 1.0.0 \
  --changelog "release metadata;artifact signature" \
  --migration-note "No migration required." \
  --rollback-note "Use kiro-agent --update-rollback before confirm." \
  --compatibility-file /tmp/kiro-release/compatibility.json

kiro-agent --config configs/kiro.advanced.example.yaml \
  --provider-public-key provider-data/keys/provider-public-key.pem \
  --update-verify \
  --update-manifest-file provider-data/updates/manifests/kiro_1.0.2.json \
  --update-artifact-file /tmp/kiro-release/kiro-agent_linux_amd64.tar.gz \
  --update-current-version 1.0.0 \
  --update-require-release-metadata \
  --update-require-artifact-signature
```

Done khi:

- Thiếu changelog/migration/rollback/compatibility thì verify release fail.
- Artifact sai checksum hoặc sai chữ ký thì verify fail.
- Release publish không đưa provider private key sang agent.

## Commercial gate bước đầu: incident/support

Mục tiêu:

- Tạo incident report JSON/Markdown.
- Gắn health report, runtime alerts và support bundle.
- Có checklist cho attack, mất SSH, update lỗi, lộ origin IP, license/rebind lỗi.

CLI lab:

```text
kiro-cli health --config configs/tenant.full-cloudflare.example.yaml \
  --preflight-writable-root /tmp/kiro-incident/preflight \
  --skip-command-checks > /tmp/kiro-incident/health.json

kiro-agent --config configs/tenant.full-cloudflare.example.yaml \
  --support-bundle \
  --support-output-dir /tmp/kiro-incident/support-bundle \
  --support-health-file /tmp/kiro-incident/health.json

kiro-cli incident report \
  --config configs/tenant.full-cloudflare.example.yaml \
  --type attack \
  --severity high \
  --summary "High traffic spike; mitigation under review." \
  --output-dir /tmp/kiro-incident/reports \
  --support-bundle-dir /tmp/kiro-incident/support-bundle \
  --health-file /tmp/kiro-incident/health.json
```

Done khi:

- Report markdown không lộ secret trong summary.
- Checklist đúng theo loại incident.
- Report link được evidence/support bundle cục bộ.

## Commercial gate bước đầu: chính sách thương mại/pháp lý

Mục tiêu:

- Có service plan rõ ràng cho Community, School/SMB, Professional, Enterprise-lite.
- Có SLA/SLO thực tế, không claim chống mọi DDoS.
- Có privacy statement, data processing note và retention.
- Có security vulnerability policy.
- Có AUP/ToS khung, DDoS limitation disclaimer, refund/warranty policy.

Done khi:

- README link tới chính sách song ngữ.
- Có `SECURITY.md` ở root repo.
- Tài liệu bán hàng không cam kết vượt quá khả năng sản phẩm/lab benchmark.

## Production gate bước đầu: license revocation

Mục tiêu:

- Provider revoke license bằng signed revocation list.
- Revoke có JSONL audit record.
- Agent check có thể từ chối license bị revoke khi có revocation list cục bộ.

CLI lab:

```text
kiro-provider --config configs/provider.example.yaml revoke-license \
  --license-id lic_revoke_000001 \
  --reason "customer cancellation"

kiro-agent --config configs/kiro.advanced.example.yaml \
  --check \
  --license-file /tmp/kiro-agent-license/license.json \
  --provider-public-key /tmp/kiro-agent-license/provider-public-key.pem \
  --machine-fingerprint sha256:test-fingerprint \
  --license-revocation-list provider-data/revocations/revocations.json
```

Done khi:

- `revocations/revocations.json` có chữ ký hợp lệ.
- `revocations/YYYY-MM.jsonl` có audit line.
- License trong list bị agent reject.
