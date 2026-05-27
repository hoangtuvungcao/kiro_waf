# Production and Commercial Readiness

## Goal

This document defines when `kiro_waf` can move from lab/dev to production and
commercial use.

Documentation alone does not make the product production-ready. Code, tests,
benchmarks, release management, and support operations must pass.

## Status Model

```text
Documentation-ready: Yes.
Implementation-ready: after Phase 0-3.
Lab-ready: after Phase 4-5.
Pilot-ready: after Phase 6-8.
Production-ready: after production gate.
Commercial-ready: after production gate plus support/legal/business gate.
```

## Production Gate

Required:

- Installer with preflight checks.
- Safe firewall dry-run and rollback.
- No SSH lockout.
- Agent systemd stability.
- Provider and agent import boundaries.
- Signed license verification.
- Signed update manifests and checksum verification.
- nftables/XDP lab tests.
- Proxy generator tests.
- Cloudflare IPv4/IPv6 origin lock tests.
- Flexible HTTP and Full Strict tests.
- WAF/bot tests.
- Resource governor tests.
- Runtime alert tests.
- Support bundle secret redaction.
- Benchmarks for common VPS sizes.

## Commercial Gate

Required after production gate:

- Clear service plans.
- Realistic SLA/SLO.
- Product limitation statement.
- Privacy policy.
- Security vulnerability reporting policy.
- Signed release process.
- Support and warranty runbooks.
- Terms of service.
- Pilot rollout with stable results.

## Go/No-Go Rules

Mandatory before commercial release:

- Build/test pass.
- Firewall rollback pass.
- Proxy reload rollback pass.
- License/update signature pass.
- Support bundle redact pass.
- Benchmarks published.
- Pilot stability period completed.

Not mandatory for MVP:

- Full dashboard.
- Provider database.
- ML detection.
- Multi-node cluster.

