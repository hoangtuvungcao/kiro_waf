# Production Gap Analysis

This document freezes the current state after Phase 19 so the project can move
to VPS testing without confusing lab readiness with customer production
readiness.

## Short Conclusion

Current state: **pilot/lab ready**. The project has enough foundation for a
controlled Ubuntu 22.04 VPS trial: config, license, signed updates,
firewall/proxy/WAF/bot generation, runtime diagnostics, installer lab flow,
benchmark lab, provider file API, and XDP/eBPF lab attach.

It should not be marketed as **enterprise/school production ready** until the
remaining gates pass: real VPS proof, real load benchmarks, production update
operation, BPF map management, provider RBAC/rate limiting, and a 30-day pilot.

## Completed

- Runtime config expands `server`/`full`, protection, paths, license, update,
  and XDP settings.
- nftables firewall supports dry-run, lab apply, rollback timer, and confirm.
- Proxy/Nginx generator emits WAF/Bot artifacts, challenge cookie config, lab
  apply, validate/reload, and rollback.
- Update manifest/artifacts support checksum, Ed25519 signatures, file/HTTP
  download, lab apply, and health-command rollback.
- License workflows support issue, verify, rebind, signed revocation list, and
  sync command.
- Provider file API exposes health, updates, revocations, licenses, incidents,
  optional bearer auth, audit log, and retention purge.
- Runtime diagnostics include status, health, preflight, support bundle,
  incident report, and provider inbox export.
- Installer supports plan, stage-lab, apply-lab, and uninstall apply-lab with
  Ubuntu and ACK guards.
- XDP/eBPF includes C source, build script, plan, lab attach/detach/confirm, root
  guard, and health rollback.
- Phase 10 CI smoke covers the main workflows without real firewall/proxy/XDP
  mutation.

## Missing Before Real Production

| Area | Status | Required work |
| --- | --- | --- |
| VPS proof | No real-host evidence yet | Upload to VPS, build, test, preflight, local benchmark, save artifacts |
| Load benchmark | PPS/conntrack/CPU-RAM under traffic not measured | Use an isolated traffic-generator lab before any public capacity claim |
| XDP production | Source/object/lab apply and map sync CLI exist | Add daemon watch/scheduler, pinned-map strategy, and continuous stats export |
| Installer production | Lab-gated apply exists | Add wizard/upgrade flow, dependency install, automatic backup/rollback |
| Update production | Lab apply exists | Add service runner, systemd restart/health, and scheduler |
| Revocation | Sync command exists | Add timer/daemon, retry/backoff, and alerts on sync failure |
| Provider API | File-backed API, scoped tokens, and rate limiting exist | Add portal, staged rollout dashboard, and multi-tenant token/retention operations |
| Runtime security | Reads normalized JSONL | Add real auditd/eBPF collector or direct agent tail integration |
| Operations | Runbooks exist | Add backup, restore drills, incident rota, SLA, and environment hardening checklist |

## Enterprise/School Gate

1. VPS smoke passes on Ubuntu 22.04/24.04 with artifacts saved.
2. Preflight has no `fail`; each warning has an owner and mitigation.
3. Local lab benchmark passes and real load benchmark is measured per VPS size.
4. Real firewall/proxy/XDP apply runs only in a lab with fallback console access.
5. Update/revocation runs under timer or service with logs, retry, and alerting.
6. Provider private data has backup, file permissions, and key rotation process.
7. A 3-5 VPS pilot runs for at least 30 days and produces a go/no-go report.

## Next Step

- Use `scripts/vps-upload-smoke.sh` to upload and run a safe VPS smoke.
- If smoke passes, open Phase 21 for production scheduler/update/revocation.
- For real DDoS/XDP claims, prepare an isolated traffic-generator lab and
  out-of-band console before native/offload XDP attach.
