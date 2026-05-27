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
