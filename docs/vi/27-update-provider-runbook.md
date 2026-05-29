# Runbook Update Và Provider File Storage

Phase 8 thêm luồng update ký số và rebind license bằng file storage. Phase 14
bổ sung download manifest/artifact qua URL `http(s)`, `file://` hoặc path cục
bộ, verify trước apply, health command cho lab apply và sync revocation list.

## Provider Sinh Key Và License

```text
go run ./cmd/kiro-provider --config configs/provider.example.yaml gen-dev-keys
```

Issue license test:

```text
go run ./cmd/kiro-provider --config configs/provider.example.yaml \
  issue-test-license \
  --license-id lic_dev_000001 \
  --customer-id cus_dev_000001 \
  --server-id srv_dev_000001 \
  --fingerprint-hash sha256:test-fingerprint \
  --agent-out-dir /tmp/kiro-agent-license
```

## Provider Publish Update Manifest

```text
go run ./cmd/kiro-provider --config configs/provider.example.yaml \
  publish-test-update \
  --version 1.0.1 \
  --channel stable \
  --artifact-file /tmp/kiro-agent_linux_amd64.tar.gz
```

Provider ghi manifest vào:

```text
provider-data/updates/manifests/kiro_1.0.1.json
provider-data/updates/manifests/stable/manifest.json
```

Manifest được ký Ed25519 bằng private key provider. Agent chỉ cần public key.

## Agent Verify Manifest Và Artifact

```text
go run ./cmd/kiro-agent --config configs/kiro.advanced.example.yaml \
  --provider-public-key /tmp/kiro-agent-license/provider-public-key.pem \
  --update-verify \
  --update-manifest-file /var/lib/kiro-provider/updates/manifests/kiro_1.0.1.json \
  --update-artifact-file /tmp/kiro-agent_linux_amd64.tar.gz \
  --update-current-version 1.0.0
```

Agent từ chối:

- Manifest sai chữ ký.
- Artifact sai checksum.
- Downgrade nếu không bật `--update-allow-downgrade`.
- Manifest không đúng channel.

## Download Manifest Và Artifact

```text
go run ./cmd/kiro-agent --config configs/kiro.advanced.example.yaml \
  --update-download \
  --update-manifest-url https://provider.example.com/updates/stable/manifest.json \
  --update-download-dir /var/lib/kiro/downloads
```

Agent tải `manifest.json`, đọc danh sách artifact trong manifest rồi tải artifact
vào cùng thư mục. Kết quả JSON có path, size và SHA256 của từng file.

`--update-verify` và `--update-apply-lab` cũng có thể dùng
`--update-manifest-url`; nếu không truyền `--update-artifact-file`, agent dùng
artifact đầu tiên vừa tải.

## Apply Lab Có Rollback

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

Health command lab:

```text
go run ./cmd/kiro-agent --config configs/kiro.advanced.example.yaml \
  --provider-public-key /tmp/kiro-agent-license/provider-public-key.pem \
  --update-apply-lab \
  --update-manifest-url https://provider.example.com/updates/stable/manifest.json \
  --update-target-file /tmp/kiro-agent-target \
  --update-state-dir /tmp/kiro-update-state \
  --update-health-command "test -x /tmp/kiro-agent-target"
```

Nếu health check fail trong lab:

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

Agent rollback file target về snapshot cũ và xóa pending update state.

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

Provider ghi audit JSONL vào `provider-data/rebindings/YYYY-MM.jsonl`.

## Giới hạn Phase 8

- Phase 14 đã có download URL và health command, nhưng apply vẫn là lab-gated
  file replacement.
- Chưa restart systemd service thật hoặc replace `/usr/local/bin/kiro-agent`
  trực tiếp trong production installer.
- Binary/systemd production apply cần thêm guard root, service health check và
  rollback timer riêng trước khi bật ngoài lab.
