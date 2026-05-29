# Production Hardening And DDoS Runbook

This runbook describes the path from VPS smoke to a real pilot for
`vutrungocrong.fun`.

## Capacity Limits

Do not claim that a single VPS can absorb 50Gbps or 10,000,000 requests/s without
provider/CDN/lab evidence. That level requires:

- CDN/Anycast/WAF edge such as Cloudflare for L7 floods.
- Upstream DDoS scrubbing from the VPS provider before traffic reaches the NIC.
- Origin lock so only Cloudflare and admin IPs can reach the origin.
- Separate traffic-generator benchmarks, not a benchmark running on the same VPS.

The current code protects the host/origin: early XDP drops, L3/L4 rate limits
by source IP, `/24` subnet, TCP SYN, UDP, ICMP, nftables, Nginx/WAF/Bot/rate
limits, and governor actions that keep the backend alive longer.

## Provider And Protected Server Separation

Provider server:

- Runs `kiro-provider`.
- Keeps the private signing key under `/etc/kiro-provider/ed25519-private.key`.
- Issues licenses, updates, and revocations.
- Does not proxy customer traffic.
- Should use scoped API tokens:

```bash
kiro-provider --config configs/provider.example.yaml serve-lab-api \
  --listen 127.0.0.1:8080 \
  --auth-token-scope agent-read=updates:read,revocations:read,licenses:read \
  --auth-token-scope ingest=health:write,incidents:write \
  --rate-limit-per-minute 120
```

Protected server/client:

- Runs `kiro-agent` and `kiro-cli`.
- Keeps only `license.json` and `provider-public-key.pem`.
- Never receives the provider private key.
- Verifies signed license, machine binding, and revocation list.

There is no absolute anti-crack guarantee when an attacker controls the binary
and root. The goal is stronger resistance: signed license, machine binding,
revocation, audit, and signed updates.

## Domain `vutrungocrong.fun`

DNS currently resolves to Cloudflare IPs. This is expected when Cloudflare proxy
is enabled. In that model:

- Origin VPS should not expose HTTP/HTTPS directly except to Cloudflare.
- Firewall should allow SSH only from admin CIDRs and HTTP/HTTPS from Cloudflare
  ranges.
- Nginx should restore real client IP from `CF-Connecting-IP`.

Config template:

```text
configs/vutrungocrong.fun.example.yaml
```

Replace `203.0.113.10/32` with the real admin IP before real firewall apply.

## XDP Map Sync

After XDP attach, sync config/maps:

```bash
kiro-agent \
  --config /etc/kiro/kiro.yaml \
  --xdp-sync-maps-lab \
  --xdp-map-ack KIRO_LAB_XDP_MAP_SYNC
```

Input files:

```text
/etc/kiro/xdp-allowlist.txt
/etc/kiro/xdp-blocklist.txt
```

Each line is IPv4 or CIDR.

`kiro_config` also receives thresholds from `server_protection.ddos`:

- `per_ip_pps`
- `per_subnet24_pps`
- `syn_per_ip_per_second`
- `udp_per_ip_per_second`
- `icmp_per_ip_per_second`

When these thresholds are greater than `0`, XDP enables the `ipv4_rate_state`
LRU map and drops early in a 1-second window.

## Generic XDP Attach With Rollback

Run only with provider console fallback:

```bash
cd /opt/kiro_waf
KIRO_LAB_XDP_APPLY=1 \
KIRO_XDP_INTERFACE=eth0 \
KIRO_XDP_MODE=generic \
KIRO_XDP_CONFIRM=0 \
bash scripts/vps-xdp-generic-apply-lab.sh /opt/kiro_waf
```

Confirm only after SSH and logs look healthy. Without confirm, the rollback
watcher detaches after the rollback timer.

## Real Firewall Apply

Do not apply real firewall rules until admin CIDR, console fallback, Cloudflare
IP ranges, reviewed dry-run, and rollback command are all ready.
