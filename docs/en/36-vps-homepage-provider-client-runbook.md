# VPS Homepage, Provider/Client, And XDP Rate-Limit Runbook

This runbook follows Phase 21 for `vutrungocrong.fun`.

## Goals

- Install a small static homepage through the Nginx origin.
- Keep provider license-server responsibilities separate from protected-server
  responsibilities.
- Upgrade XDP from basic allow/block lists to kernel-level L3/L4 rate limits.
- Keep risky host changes gated by ACK flags and rollback.

## Current XDP

Source: `ebpf/xdp/kiro_xdp_drop.c`

Main maps:

- `kiro_config`: runtime drop and rate-limit config.
- `ipv4_allowlist`: IPv4/CIDR prefixes that always pass.
- `ipv4_blocklist`: IPv4/CIDR prefixes that always drop.
- `ipv4_rate_state`: LRU hash for source-IP and `/24` rate windows.
- `kiro_stats`: pass/drop counters by reason.

Thresholds come from YAML:

```yaml
server_protection:
  xdp:
    enabled: true
    drop_private_source_ip: true
    drop_malformed: true
  ddos:
    per_ip_pps: 3000
    per_subnet24_pps: 30000
    syn_per_ip_per_second: 200
    udp_per_ip_per_second: 500
    icmp_per_ip_per_second: 30
```

Map sync:

```bash
kiro-agent \
  --config /etc/kiro/kiro.yaml \
  --xdp-sync-maps-lab \
  --xdp-map-ack KIRO_LAB_XDP_MAP_SYNC
```

## Provider/Client Setup On The VPS

Script:

```bash
cd /opt/kiro_waf
sudo bash scripts/vps-provider-client-setup.sh /opt/kiro_waf
```

Results:

- `/usr/local/bin/kiro-provider`, `kiro-agent`, and `kiro-cli`.
- `/etc/kiro-provider/provider.yaml`.
- `/etc/kiro-provider/ed25519-private.key` stays on the provider side.
- `/etc/kiro/license.json` and `/etc/kiro/provider-public-key.pem` stay on the
  protected-server/client side.
- `kiro-agent --check` verifies the license against the VPS fingerprint.

Do not copy the provider private signing key to customer/protected servers.

## Homepage Install

Source:

```text
site/kiro-home/
```

Install through Nginx:

```bash
cd /opt/kiro_waf
sudo KIRO_HOME_DOMAIN=vutrungocrong.fun \
  KIRO_HOME_ALT_DOMAIN=www.vutrungocrong.fun \
  bash scripts/vps-install-homepage.sh /opt/kiro_waf
```

Origin checks:

```bash
curl -i -H 'Host: vutrungocrong.fun' http://127.0.0.1/health
curl -I -H 'Host: vutrungocrong.fun' http://127.0.0.1/
```

If Cloudflare proxying is enabled, the public response may be a Cloudflare
challenge depending on Cloudflare rules. The local origin check should still be
`200`.

## Safe XDP Replacement

`scripts/vps-xdp-generic-apply-lab.sh` detaches an existing XDP program before
attaching the new object by default:

```bash
cd /opt/kiro_waf
sudo KIRO_LAB_XDP_APPLY=1 \
  KIRO_XDP_INTERFACE=eth0 \
  KIRO_XDP_MODE=generic \
  KIRO_XDP_CONFIRM=0 \
  bash scripts/vps-xdp-generic-apply-lab.sh /opt/kiro_waf
```

If SSH and health checks remain healthy:

```bash
sudo /opt/kiro_waf/build/vps/kiro-agent \
  --config /opt/kiro_waf/configs/vutrungocrong.fun.example.yaml \
  --xdp-confirm \
  --xdp-state-dir /var/lib/kiro
```

## Production Limits

- Host-level XDP cannot help after upstream bandwidth is saturated.
- `generic` XDP is safer for the first VPS rollout, but slower than
  `native/offload`.
- Do not apply real nftables origin-lock rules before confirming the admin CIDR
  and Cloudflare IP ranges.
- Do not claim 50Gbps or 10M requests/s without CDN/upstream scrubbing and an
  independent traffic-generator lab.
