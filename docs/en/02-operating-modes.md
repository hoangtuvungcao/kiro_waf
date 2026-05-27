# Operating Modes

## server

Protects the host and network services only.

Enabled:

- XDP/eBPF.
- TC/eBPF.
- nftables.
- L3/L4 anti-DDoS.
- Conntrack protection.
- Runtime security.
- Resource governor.

Disabled:

- Website reverse proxy.
- HTTP WAF.
- Website bot challenge.
- HTTP route quota.
- Cloudflare origin lock.

## full

Protects the host and public websites/APIs.

Enabled:

- Everything in `server`.
- Nginx/HAProxy gateway.
- WAF/API validation.
- Bot scoring.
- Cookie/JS challenge.
- Route quota.
- Optional Cloudflare Free origin lock.

Runtime defense levels are separate from product mode:

```text
NORMAL
ELEVATED
ATTACK
LOCKDOWN
```

