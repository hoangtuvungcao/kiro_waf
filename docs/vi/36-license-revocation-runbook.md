# Runbook License Revocation

Runbook này hoàn thiện phần provider audit trong production gate:
activation/rebind/revoke đều có record. Revocation dùng signed revocation list
để agent có thể từ chối license bị thu hồi khi có file list cục bộ.

## Issue License Test

```text
go run ./cmd/kiro-provider --config configs/provider.example.yaml gen-dev-keys

go run ./cmd/kiro-provider --config configs/provider.example.yaml \
  issue-test-license \
  --license-id lic_revoke_000001 \
  --customer-id cus_revoke_000001 \
  --server-id srv_revoke_000001 \
  --fingerprint-hash sha256:test-fingerprint \
  --agent-out-dir /tmp/kiro-agent-license
```

## Revoke License

```text
go run ./cmd/kiro-provider --config configs/provider.example.yaml \
  revoke-license \
  --license-id lic_revoke_000001 \
  --reason "customer cancellation"
```

Provider ghi:

```text
provider-data/revocations/revocations.json
provider-data/revocations/YYYY-MM.jsonl
```

`revocations.json` được ký Ed25519 bằng provider key. JSONL là audit trail.

## Agent Check Với Revocation List

```text
go run ./cmd/kiro-agent \
  --config configs/kiro.advanced.example.yaml \
  --check \
  --license-file /tmp/kiro-agent-license/license.json \
  --provider-public-key /tmp/kiro-agent-license/provider-public-key.pem \
  --machine-fingerprint sha256:test-fingerprint \
  --license-revocation-list provider-data/revocations/revocations.json
```

Agent sẽ fail nếu `license_id` nằm trong revocation list đã ký.

## Sync Revocation List

Phase 14 thêm lệnh tải revocation list từ provider:

```text
go run ./cmd/kiro-agent \
  --license-revocation-sync \
  --license-revocation-url https://provider.example.com/revocations/revocations.json \
  --license-revocation-output /var/lib/kiro/revocations.json
```

Sau khi sync, dùng file đó trong `--check`:

```text
go run ./cmd/kiro-agent \
  --config configs/kiro.advanced.example.yaml \
  --check \
  --license-file /etc/kiro/license.json \
  --provider-public-key /etc/kiro/provider-public-key.pem \
  --license-revocation-list /var/lib/kiro/revocations.json
```

## Rebind Không Thay Thế Revoke

Rebind dùng khi khách hàng đổi máy hợp lệ. Revoke dùng khi license không còn
được phép chạy: hủy dịch vụ, gian lận, chargeback, vi phạm AUP hoặc security
risk. Hai luồng đều có audit JSONL riêng.

## Giới Hạn

- Agent hiện sync bằng lệnh chủ động; chưa có daemon scheduler tự chạy định kỳ.
- Nếu server offline lâu, license đã revoke chỉ bị chặn sau khi revocation list
  mới được đưa tới server.
