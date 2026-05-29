# License Revocation Runbook

This completes the provider audit gate: activation, rebind, and revoke all have
records. Revocation uses a signed revocation list so the agent can reject a
revoked license when the local list is provided.

## Issue Test License

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

The provider writes:

```text
provider-data/revocations/revocations.json
provider-data/revocations/YYYY-MM.jsonl
```

`revocations.json` is signed with the provider Ed25519 key. JSONL is the audit
trail.

## Agent Check With Revocation List

```text
go run ./cmd/kiro-agent \
  --config configs/kiro.advanced.example.yaml \
  --check \
  --license-file /tmp/kiro-agent-license/license.json \
  --provider-public-key /tmp/kiro-agent-license/provider-public-key.pem \
  --machine-fingerprint sha256:test-fingerprint \
  --license-revocation-list provider-data/revocations/revocations.json
```

The agent fails when the `license_id` appears in the signed revocation list.

## Sync Revocation List

Phase 14 adds a provider download command:

```text
go run ./cmd/kiro-agent \
  --license-revocation-sync \
  --license-revocation-url https://provider.example.com/revocations/revocations.json \
  --license-revocation-output /var/lib/kiro/revocations.json
```

After sync, use the downloaded file during `--check`:

```text
go run ./cmd/kiro-agent \
  --config configs/kiro.advanced.example.yaml \
  --check \
  --license-file /etc/kiro/license.json \
  --provider-public-key /etc/kiro/provider-public-key.pem \
  --license-revocation-list /var/lib/kiro/revocations.json
```

## Rebind Is Not Revoke

Rebind is for legitimate server replacement. Revoke is for licenses that are no
longer allowed to run: cancellation, abuse, chargeback, AUP violation, or
security risk. Both workflows have separate JSONL audit trails.

## Limits

- The agent syncs on explicit command; there is no daemon scheduler yet.
- If a server stays offline, a revoked license is blocked only after the new
  revocation list reaches that server.
