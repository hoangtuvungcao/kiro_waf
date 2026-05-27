# Architecture

## Full Mode Flow

```text
Internet
  |
Cloudflare Free, optional
  |
Origin firewall allowing only Cloudflare on 80/443
  |
XDP/eBPF fast drop
  |
TC/eBPF + nftables raw/notrack
  |
conntrack-protected firewall
  |
Nginx/HAProxy gateway
  |
Bot scoring + challenge + route quota
  |
WAF/API validation
  |
Application backends
  |
Runtime security + resource governor
```

## Server Mode Flow

```text
Internet
  |
XDP/eBPF fast drop
  |
TC/eBPF + nftables raw/notrack
  |
conntrack-protected firewall
  |
Protected server services
  |
Runtime security + resource governor
```

## Core Components

- `kiro-agent`: local server agent, enforcement, config, metrics, rollback.
- `kiro-cli`: local administration.
- `kiro-provider`: file-based provider management for customers, licenses,
  servers, updates, and support.

