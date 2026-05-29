# Installer And Uninstall Lab Runbook

This adds the first installer production gate: JSON install plans, lab-root file
staging, guarded lab apply, systemd artifact checks, and uninstall plan/apply.
`stage-lab` writes only under the `--install-root` directory and does not touch
the host `/etc`, `/usr`, `/var`, or `systemctl`.

## Build Lab Binaries

```text
mkdir -p /tmp/kiro-build
go build -o /tmp/kiro-build/kiro-agent ./cmd/kiro-agent
go build -o /tmp/kiro-build/kiro-cli ./cmd/kiro-cli
```

## Install Plan

```text
go run ./cmd/kiro-cli install plan \
  --config configs/tenant.full-cloudflare.example.yaml \
  --agent-binary /tmp/kiro-build/kiro-agent \
  --cli-binary /tmp/kiro-build/kiro-cli
```

The plan includes:

- Preflight before install.
- Creating `/etc/kiro`, `/var/lib/kiro`, `/var/log/kiro`, and `/run/kiro`.
- Copying config, binaries, and `kiro-agent.service`.
- Firewall dry-run with a last-good snapshot.
- Proxy dry-run.
- systemd reload and service enablement for real environments.

Each step marks `requires_root` so operators can distinguish real installs from
lab/staging plans.

## Lab Stage

```text
rm -rf /tmp/kiro-install-root
go run ./cmd/kiro-cli install stage-lab \
  --config configs/tenant.full-cloudflare.example.yaml \
  --install-root /tmp/kiro-install-root \
  --agent-binary /tmp/kiro-build/kiro-agent \
  --cli-binary /tmp/kiro-build/kiro-cli
```

Expected files:

```text
/tmp/kiro-install-root/etc/kiro/kiro.yaml
/tmp/kiro-install-root/etc/systemd/system/kiro-agent.service
/tmp/kiro-install-root/usr/local/bin/kiro-agent
/tmp/kiro-install-root/usr/local/bin/kiro-cli
/tmp/kiro-install-root/var/lib/kiro/install-manifest.json
```

Quick checks:

```text
test -f /tmp/kiro-install-root/var/lib/kiro/install-manifest.json
rg '"target": "/usr/local/bin/kiro-agent"' /tmp/kiro-install-root/var/lib/kiro/install-manifest.json
```

## Apply Lab Into Install Root

This copies files into `--install-root` and writes an apply manifest. When
`--install-root` is set, command steps such as preflight/systemd/firewall/proxy
are skipped by default so the current host is not touched.

```text
go run ./cmd/kiro-cli install apply-lab \
  --config configs/tenant.full-cloudflare.example.yaml \
  --install-root /tmp/kiro-install-root \
  --agent-binary /tmp/kiro-build/kiro-agent \
  --cli-binary /tmp/kiro-build/kiro-cli \
  --ack KIRO_LAB_INSTALL_APPLY
```

Additional result:

```text
/tmp/kiro-install-root/var/lib/kiro/install-apply-manifest.json
```

Use `--run-steps` only in a dedicated lab root where running the command steps is
intended. Do not use `--run-steps` with `--install-root` on a normal dev/CI host.

## Apply Lab Into Real Root

Run this only on an Ubuntu 22.04/24.04 lab VPS with console fallback and backup.
When `--install-root` is omitted, the command requires root and runs command
steps: preflight, firewall dry-run snapshot, proxy dry-run, `systemctl
daemon-reload`, and `systemctl enable --now kiro-agent.service`.

```text
sudo ./kiro-cli install apply-lab \
  --config /path/to/kiro.yaml \
  --agent-binary /tmp/kiro-build/kiro-agent \
  --cli-binary /tmp/kiro-build/kiro-cli \
  --ack KIRO_LAB_INSTALL_APPLY
```

Mandatory guards:

- Exact ACK: `KIRO_LAB_INSTALL_APPLY`.
- Root when applying into `/`.
- `/etc/os-release` must be Ubuntu 22.04 or 24.04.

## Uninstall Plan

Without data purge:

```text
go run ./cmd/kiro-cli install uninstall-plan \
  --config configs/tenant.full-cloudflare.example.yaml
```

By default, the plan removes service/binaries and keeps config, license, state,
and logs.

Destructive purge:

```text
go run ./cmd/kiro-cli install uninstall-plan \
  --config configs/tenant.full-cloudflare.example.yaml \
  --purge
```

Use purge only after backup and explicit confirmation that config/state/log data
should be removed.

## Uninstall Apply Lab

Without data purge:

```text
go run ./cmd/kiro-cli install uninstall-apply-lab \
  --config configs/tenant.full-cloudflare.example.yaml \
  --install-root /tmp/kiro-install-root \
  --ack KIRO_LAB_UNINSTALL_APPLY
```

This removes service/binaries and keeps config, state, and logs.

Destructive purge:

```text
go run ./cmd/kiro-cli install uninstall-apply-lab \
  --config configs/tenant.full-cloudflare.example.yaml \
  --install-root /tmp/kiro-install-root \
  --ack KIRO_LAB_UNINSTALL_APPLY \
  --purge
```

Without `--install-root`, uninstall apply has the same root and Ubuntu guards as
install apply.

## Limits

- `stage-lab` does not call `systemctl`.
- Missing system packages such as `nftables` or `nginx` are not installed.
- Real apply is lab-gated, not a production installer wizard yet.
- There is no automatic rollback for install apply yet; uninstall apply is the
  main manual recovery path.
