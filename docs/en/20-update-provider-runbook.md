# Update And Provider File Storage Runbook

Phase 8 adds signed update manifests and license rebinds using file storage.
Phase 14 adds manifest/artifact downloads over `http(s)`, `file://`, or local
paths, verification before apply, a lab health command, and revocation-list sync.

## Provider Keys And License

```text
go run ./cmd/kiro-provider --config configs/provider.example.yaml gen-dev-keys
```

Issue a test license:

```text
go run ./cmd/kiro-provider --config configs/provider.example.yaml \
  issue-test-license \
  --license-id lic_dev_000001 \
  --customer-id cus_dev_000001 \
  --server-id srv_dev_000001 \
  --fingerprint-hash sha256:test-fingerprint \
  --agent-out-dir /tmp/kiro-agent-license
```

## Publish An Update Manifest

```text
go run ./cmd/kiro-provider --config configs/provider.example.yaml \
  publish-test-update \
  --version 1.0.1 \
  --channel stable \
  --artifact-file /tmp/kiro-agent_linux_amd64.tar.gz
```

The provider writes:

```text
provider-data/updates/manifests/kiro_1.0.1.json
provider-data/updates/manifests/stable/manifest.json
```

The manifest is signed with the provider Ed25519 private key. The agent only
needs the provider public key.

## Agent Verify

```text
go run ./cmd/kiro-agent --config configs/kiro.advanced.example.yaml \
  --provider-public-key /tmp/kiro-agent-license/provider-public-key.pem \
  --update-verify \
  --update-manifest-file /var/lib/kiro-provider/updates/manifests/kiro_1.0.1.json \
  --update-artifact-file /tmp/kiro-agent_linux_amd64.tar.gz \
  --update-current-version 1.0.0
```

The agent rejects bad signatures, checksum mismatches, unexpected channels, and
downgrades unless `--update-allow-downgrade` is set.

## Download Manifest And Artifacts

```text
go run ./cmd/kiro-agent --config configs/kiro.advanced.example.yaml \
  --update-download \
  --update-manifest-url https://provider.example.com/updates/stable/manifest.json \
  --update-download-dir /var/lib/kiro/downloads
```

The agent downloads `manifest.json`, reads artifact URLs from the manifest, and
writes all downloaded files to the output directory. The JSON result includes
path, size, and SHA256 for each file.

`--update-verify` and `--update-apply-lab` can also use `--update-manifest-url`.
When `--update-artifact-file` is omitted, the agent uses the first downloaded
artifact.

## Lab Apply With Rollback

```text
go run ./cmd/kiro-agent --config configs/kiro.advanced.example.yaml \
  --provider-public-key /tmp/kiro-agent-license/provider-public-key.pem \
  --update-apply-lab \
  --update-manifest-file /var/lib/kiro-provider/updates/manifests/kiro_1.0.1.json \
  --update-artifact-file /tmp/kiro-agent_linux_amd64.tar.gz \
  --update-target-file /tmp/kiro-agent-target \
  --update-state-dir /tmp/kiro-update-state \
  --update-current-version 1.0.0
```

Lab health command:

```text
go run ./cmd/kiro-agent --config configs/kiro.advanced.example.yaml \
  --provider-public-key /tmp/kiro-agent-license/provider-public-key.pem \
  --update-apply-lab \
  --update-manifest-url https://provider.example.com/updates/stable/manifest.json \
  --update-target-file /tmp/kiro-agent-target \
  --update-state-dir /tmp/kiro-update-state \
  --update-health-command "test -x /tmp/kiro-agent-target"
```

Simulate a failed health check:

```text
go run ./cmd/kiro-agent --config configs/kiro.advanced.example.yaml \
  --provider-public-key /tmp/kiro-agent-license/provider-public-key.pem \
  --update-apply-lab \
  --update-manifest-file /var/lib/kiro-provider/updates/manifests/kiro_1.0.1.json \
  --update-artifact-file /tmp/kiro-agent_linux_amd64.tar.gz \
  --update-target-file /tmp/kiro-agent-target \
  --update-state-dir /tmp/kiro-update-state \
  --update-current-version 1.0.0 \
  --update-health-fail
```

The agent restores the target from the rollback snapshot and removes pending
update state.

## Rebind License

```text
go run ./cmd/kiro-provider --config configs/provider.example.yaml \
  rebind-license \
  --license-id lic_dev_000001 \
  --server-id srv_dev_000002 \
  --fingerprint-hash sha256:new-fingerprint \
  --reason "replace server" \
  --agent-out-dir /tmp/kiro-agent-license-rebound
```

The provider writes a JSONL audit record under
`provider-data/rebindings/YYYY-MM.jsonl`.

## Phase 8 Limits

- Phase 14 supports URL download and a lab health command, but apply remains a
  lab-gated file replacement.
- It does not restart systemd or replace `/usr/local/bin/kiro-agent` directly
  through the production installer yet.
- Binary/systemd production apply still needs root guards, service health
  checks, and a rollback timer before non-lab use.
