# Build and Test Plan

This project should be built in small validated phases.

## Phase Order

1. Repository bootstrap: Go module, `kiro-agent`, `kiro-cli`.
2. Add `kiro-provider` skeleton with strict import boundaries.
3. Config and file storage.
4. Local license verification.
5. Provider license issue flow.
6. Firewall generation and dry-run.
7. Safe firewall apply with rollback.
8. Proxy config generator.
9. Resource governor and overload modes.
10. WAF and bot defense.
11. Provider file storage, signed updates, and rebind workflow.
12. Runtime security and support bundle.
13. Production gate checks: preflight/status/health and lab benchmark reports.

## Required Gates

Each phase must have:

- Unit tests.
- Error-path tests.
- Config validation.
- Safe rollback if it changes the system.
- Clear CLI path.
- Documentation update.
- Import-boundary tests: agent must not import provider packages, and provider
  must not import agent firewall/eBPF packages.

## Safety Rules

- Do not test attack traffic on public targets.
- Do not apply firewall rules without admin allowlist and rollback timer.
- Do not enable full mode without validating backend and proxy config.
- Do not trust Cloudflare headers unless the source IP is a Cloudflare range.
- Do not claim strong DDoS protection without benchmarks.

Firewall dry-run command:

```text
kiro-agent --config configs/kiro.example.yaml --firewall-dry-run
```

Firewall lab apply runbook:

```text
docs/en/16-firewall-apply-lab-runbook.md
```

Proxy generator dry-run:

```text
kiro-agent --config configs/kiro.example.yaml --proxy-dry-run
```

Resource governor evaluate:

```text
kiro-agent --config configs/tenant.server-only.example.yaml --governor-evaluate \
  --sample-cpu-percent 86 --sample-ram-available-percent 50
```

WAF and bot lab evaluate:

```text
kiro-agent --config configs/tenant.full-cloudflare.example.yaml \
  --web-defense-evaluate --web-path /login \
  --web-query "user=admin' OR 1=1--"
```

Signed update verify:

```text
kiro-agent --config configs/kiro.advanced.example.yaml \
  --provider-public-key /tmp/provider-public-key.pem \
  --update-verify \
  --update-manifest-file /tmp/kiro_1.0.1.json \
  --update-artifact-file /tmp/kiro-agent_linux_amd64.tar.gz \
  --update-current-version 1.0.0
```

Runtime security and support bundle:

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

Preflight/status/health:

```text
kiro-cli status --config configs/tenant.full-cloudflare.example.yaml
kiro-cli preflight --config configs/tenant.full-cloudflare.example.yaml \
  --preflight-writable-root /tmp/kiro-preflight
kiro-cli health --config configs/tenant.full-cloudflare.example.yaml \
  --preflight-writable-root /tmp/kiro-preflight
```

Lab benchmark report:

```text
kiro-agent --config configs/tenant.full-cloudflare.example.yaml \
  --benchmark-lab \
  --benchmark-iterations 5000 \
  --benchmark-temp-ban-size 5000 \
  --benchmark-output-file /tmp/kiro-benchmark.json
```

The report must distinguish measured local metrics from `not_measured` items
such as XDP PPS, conntrack pressure, and CPU/RAM under attack.

Installer and uninstall lab:

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

The lab stage must not write outside `--install-root`; purge uninstall remains
opt-in.

Agent event log rate limit and rotation:

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

Repeated events within the same window must be suppressed, and files must rotate
when the byte limit is exceeded.

Release management:

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

Release verification must fail if release metadata, artifact signature, checksum,
or manifest signature is invalid.

Incident and support report:

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

The Markdown report must be redacted and include the checklist for the incident
type.

Commercial/legal policy gate:

```text
docs/en/28-commercial-legal-policies.md
docs/vi/35-commercial-legal-policies.md
SECURITY.md
```

The policy docs must cover service plans, realistic SLA/SLO, product limits,
privacy/data processing, vulnerability reporting, acceptable use, terms draft,
and refund/warranty rules.

License revocation:

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

The provider must write a signed revocation list and JSONL audit record; the
agent must reject licenses present in the signed list.
