# Runbook Release Management

Runbook này bổ sung commercial gate “release management”: mỗi release có version,
changelog, signed artifact metadata, signed manifest, checksum, migration note,
rollback note và compatibility matrix.

## Chuẩn Bị Artifact

```text
mkdir -p /tmp/kiro-release
go build -o /tmp/kiro-release/kiro-agent ./cmd/kiro-agent
tar -C /tmp/kiro-release -czf /tmp/kiro-release/kiro-agent_linux_amd64.tar.gz kiro-agent
```

## Compatibility Matrix

Tạo file JSON:

```text
cat > /tmp/kiro-release/compatibility.json <<'JSON'
[
  {
    "target": "ubuntu_22.04_amd64",
    "status": "pass",
    "notes": "config check, preflight, benchmark smoke passed"
  },
  {
    "target": "ubuntu_24.04_amd64",
    "status": "warn",
    "notes": "roadmap target; requires extra lab verification before production"
  }
]
JSON
```

Status hợp lệ: `pass`, `warn`, `unsupported`, `not_tested`.

## Publish Release

```text
go run ./cmd/kiro-provider --config configs/provider.example.yaml gen-dev-keys

go run ./cmd/kiro-provider --config configs/provider.example.yaml \
  publish-release \
  --version 1.0.2 \
  --channel stable \
  --artifact-file /tmp/kiro-release/kiro-agent_linux_amd64.tar.gz \
  --min-agent-version 1.0.0 \
  --changelog "add release metadata;sign artifact metadata;add compatibility matrix" \
  --migration-note "No migration required for file-storage MVP." \
  --rollback-note "Use kiro-agent --update-rollback while pending update state exists." \
  --compatibility-file /tmp/kiro-release/compatibility.json
```

Provider ghi:

```text
provider-data/updates/manifests/kiro_1.0.2.json
provider-data/updates/manifests/stable/manifest.json
```

Manifest được ký Ed25519. Artifact có SHA256 và chữ ký metadata artifact trong
manifest; chữ ký artifact bind `name` và `sha256`.

## Agent Verify Release

```text
go run ./cmd/kiro-agent --config configs/kiro.advanced.example.yaml \
  --provider-public-key provider-data/keys/provider-public-key.pem \
  --update-verify \
  --update-manifest-file provider-data/updates/manifests/kiro_1.0.2.json \
  --update-artifact-file /tmp/kiro-release/kiro-agent_linux_amd64.tar.gz \
  --update-current-version 1.0.0 \
  --update-require-release-metadata \
  --update-require-artifact-signature
```

Agent từ chối release nếu:

- Manifest sai chữ ký.
- Artifact sai checksum.
- Artifact thiếu hoặc sai chữ ký khi bật `--update-require-artifact-signature`.
- Thiếu changelog/migration note/rollback note/compatibility matrix khi bật
  `--update-require-release-metadata`.
- Downgrade ngoài policy.

## Giới Hạn

- Chữ ký artifact hiện nằm trong manifest, chưa xuất file `.sig` riêng.
- Lệnh publish chỉ ghi file storage provider, chưa upload CDN.
- Compatibility matrix là bằng chứng lab do provider nhập, chưa tự chạy toàn bộ
  matrix VPS.
