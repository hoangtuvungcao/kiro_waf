# Preflight, Status, And Health Runbook

This adds the basic production gate checks: local environment preflight, runtime
config status, local health report, and config mode changes. These commands do
not apply firewall or proxy changes.

## Status

```text
go run ./cmd/kiro-cli status --config configs/tenant.full-cloudflare.example.yaml
```

The JSON output includes mode, plan, site/backend pool counts, and enabled
modules: firewall, proxy, WAF, bot, governor, updates, and runtime security.

## Preflight

```text
go run ./cmd/kiro-cli preflight \
  --config configs/tenant.full-cloudflare.example.yaml \
  --preflight-writable-root /tmp/kiro-preflight \
  --skip-command-checks
```

Preflight checks:

- Linux/Ubuntu target.
- Root privileges for real apply operations.
- Admin allowlist.
- Writable state directory.
- `nft`, `nginx`, and `systemctl` unless command checks are skipped.

On a non-Ubuntu/non-root dev machine, warnings are expected. On real production
labs, do not skip command checks.

## Health

```text
go run ./cmd/kiro-cli health \
  --config configs/tenant.full-cloudflare.example.yaml \
  --preflight-writable-root /tmp/kiro-preflight \
  --skip-command-checks
```

Health combines config checks and preflight checks, returning `pass`, `warn`, or
`fail`.

## Mode

Show mode:

```text
go run ./cmd/kiro-cli mode show --config configs/tenant.server-only.example.yaml
```

Change mode in a lab config file:

```text
cp configs/tenant.server-only.example.yaml /tmp/kiro-mode-test.yaml
go run ./cmd/kiro-cli mode set --config /tmp/kiro-mode-test.yaml --mode full
```

After changing mode, run config check or health. `full` mode still requires valid
website config.

## Limits

- Preflight is a local/lab check, not a full installer wizard.
- It does not install missing packages.
- It does not change systemd, firewall, or proxy state.
