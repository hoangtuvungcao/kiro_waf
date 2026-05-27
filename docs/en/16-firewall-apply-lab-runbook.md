# Firewall Apply Lab Runbook

Phase 4 is for Ubuntu 22.04 lab testing only. Do not run it directly on
production.

## Required preconditions

- Out-of-band console access is available.
- Admin CIDR in the config is correct.
- `nftables` and the `nft` command are installed.
- Dry-run and `nft -c` pass.
- The operator understands that a wrong rule can lock SSH.

## Dry-run

```text
go run ./cmd/kiro-agent --config configs/tenant.server-only.example.yaml --firewall-dry-run
```

## Lab apply

```text
sudo bash deployments/lab/firewall-apply-ubuntu-22.04.sh configs/tenant.server-only.example.yaml
```

Real apply requires this explicit acknowledgement:

```text
--firewall-lab-ack KIRO_LAB_FIREWALL_APPLY
```

Without that value, the agent refuses firewall apply and rollback operations.

## Confirm

After apply, verify SSH from the admin IP and expected web ports. If healthy:

```text
sudo go run ./cmd/kiro-agent \
  --config configs/tenant.server-only.example.yaml \
  --firewall-confirm \
  --firewall-state-dir /var/lib/kiro
```

## Manual rollback

```text
sudo go run ./cmd/kiro-agent \
  --config configs/tenant.server-only.example.yaml \
  --firewall-rollback \
  --firewall-lab-ack KIRO_LAB_FIREWALL_APPLY \
  --firewall-state-dir /var/lib/kiro
```

## Expired rollback check

This command can be called by a systemd timer:

```text
sudo go run ./cmd/kiro-agent \
  --config configs/tenant.server-only.example.yaml \
  --firewall-rollback-if-expired \
  --firewall-lab-ack KIRO_LAB_FIREWALL_APPLY \
  --firewall-state-dir /var/lib/kiro
```

## State files

```text
/var/lib/kiro/pending-firewall-apply.json
/var/lib/kiro/last-good-config/last-good-nftables.json
```

If SSH is locked and pending rollback is unavailable, use console access:

```text
sudo nft delete table inet kiro_waf
```
