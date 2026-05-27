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
- Reload an toàn.
- Rollback nếu reload fail.

Test bắt buộc:

- 1 domain nhiều backend.
- Nhiều domain một backend.
- 1 domain nhiều route/backend.
- Route quota xuất hiện đúng.
- Cloudflare real IP include đúng.
- `flexible_http` generate Nginx listen 80, không yêu cầu cert/key.
- `full_strict` generate Nginx listen 443 ssl và yêu cầu cert/key.

Done khi:

- Proxy config chạy trong lab.
- Backend health check cơ bản hoạt động.

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
