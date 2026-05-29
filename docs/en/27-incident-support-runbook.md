# Incident And Support Runbook

This adds the commercial support gate: standardized incident reports,
support-bundle/health/alert evidence links, and checklists for common support
cases.

## Build Health And Support Bundle

```text
mkdir -p /tmp/kiro-incident

go run ./cmd/kiro-cli health \
  --config configs/tenant.full-cloudflare.example.yaml \
  --preflight-writable-root /tmp/kiro-incident/preflight \
  --skip-command-checks \
  > /tmp/kiro-incident/health.json

go run ./cmd/kiro-agent \
  --config configs/tenant.full-cloudflare.example.yaml \
  --support-bundle \
  --support-output-dir /tmp/kiro-incident/support-bundle \
  --support-health-file /tmp/kiro-incident/health.json
```

## Create Incident Report

```text
go run ./cmd/kiro-cli incident report \
  --config configs/tenant.full-cloudflare.example.yaml \
  --type attack \
  --severity high \
  --summary "High traffic spike on web/API paths; mitigation under review." \
  --output-dir /tmp/kiro-incident/reports \
  --support-bundle-dir /tmp/kiro-incident/support-bundle \
  --health-file /tmp/kiro-incident/health.json
```

Outputs:

```text
/tmp/kiro-incident/reports/<incident_id>/incident-report.json
/tmp/kiro-incident/reports/<incident_id>/incident-report.md
```

The Markdown report is redacted using the same logic as support bundles.

## Incident Types

`--type` supports:

- `attack`.
- `lost_ssh`.
- `update_failed`.
- `origin_ip_leaked`.
- `license_rebind`.
- `runtime_security`.
- `other`.

## Quick Checklists

Attack:

- Do not test attack traffic against public targets.
- Preserve support bundle, health report, and event logs.
- Review governor level, WAF/bot, firewall snapshot, and Cloudflare origin lock.
- Record mitigation actions and cooldown timing.

Lost SSH:

- Use VPS console/rescue access first.
- Check pending firewall rollback state.
- Restore last-good config if needed.
- Confirm admin CIDR and SSH port before applying again.

Update failed:

- Do not confirm the pending update.
- Run rollback while pending state still exists.
- Check manifest signature, artifact checksum, and release metadata.

Origin IP leaked:

- Check that DNS records are proxied through Cloudflare.
- Enable or verify Cloudflare origin lock.
- Block direct origin HTTP/HTTPS when policy requires it.
- Consider rotating the origin IP if it was widely exposed.

License/rebind failed:

- Collect the new fingerprint with `kiro-cli license fingerprint`.
- Check rebind quota and audit reason.
- Rebind through the provider, exporting only `license.json` and the public key
  to the agent.
- Verify the license locally on the protected server.

## Limits

- Incident reports are local file artifacts, not synced to a provider portal yet.
- Operators still need to fill the real timeline in Markdown.
- This does not replace legal/customer notification workflows for serious
  incidents.
