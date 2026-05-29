# Release Management Runbook

This adds the commercial gate for release management: each release includes a
version, changelog, signed artifact metadata, signed manifest, checksum,
migration note, rollback note, and compatibility matrix.

## Prepare Artifact

```text
mkdir -p /tmp/kiro-release
go build -o /tmp/kiro-release/kiro-agent ./cmd/kiro-agent
tar -C /tmp/kiro-release -czf /tmp/kiro-release/kiro-agent_linux_amd64.tar.gz kiro-agent
```

## Compatibility Matrix

Create a JSON file:

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

Valid statuses: `pass`, `warn`, `unsupported`, `not_tested`.

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

The provider writes:

```text
provider-data/updates/manifests/kiro_1.0.2.json
provider-data/updates/manifests/stable/manifest.json
```

The manifest is signed with Ed25519. The artifact has a SHA256 checksum and
signed artifact metadata inside the manifest; the artifact signature binds
`name` and `sha256`.

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

The agent rejects releases when:

- The manifest signature is invalid.
- The artifact checksum does not match.
- The artifact signature is missing or invalid when
  `--update-require-artifact-signature` is enabled.
- Changelog, migration note, rollback note, or compatibility matrix is missing
  when `--update-require-release-metadata` is enabled.
- The manifest is a downgrade outside policy.

## Limits

- The artifact signature currently lives in the manifest, not a separate `.sig`
  file.
- Publish writes provider file storage only; no CDN upload yet.
- The compatibility matrix is provider-supplied lab evidence, not a full
  automated VPS matrix runner yet.
