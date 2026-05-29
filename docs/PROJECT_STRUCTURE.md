# Project Structure

Kiro WAF is split into two runtime roles:

- Management Server: `kiro-provider`, dashboard, license/update/revocation API.
- Client Node / Edge Shield: `kiro-agent`, XDP/nftables/proxy/WAF runtime.
- Standalone production path: `master-server/` and `client-node/` provide a
  compact deployable Master/Client split for `firewall.vpsgen.com`.

```text
master-server/
  main.go                       SQLite license DB, homepage, admin UI, API
  nginx/firewall.conf           /etc/nginx/sites-available/firewall.conf source
  systemd/kiro-master.service   Master systemd unit

client-node/
  client_waf.go                 Golang WAF reverse proxy + license heartbeat
  xdp_filter.c                  XDP/eBPF L3/L4 packet filter C source
  systemd/kiro-client-waf.service

deploy_master.sh                Master deploy wrapper for Ubuntu 22.04

cmd/
  kiro-agent/                 protected-server runtime CLI
  kiro-cli/                   local operator CLI
  kiro-provider/              management/license/update API server

internal/
  agent/
    benchmark/                lab benchmark evidence
    bot/                      JS/cookie challenge evaluation
    firewall/                 nftables plan/apply/rollback
    governor/                 resource governor and response overlays
    proxy/                    Nginx/WAF config generation and apply lab
    runtime/                  runtime support helpers
    waf/                      WAF dry-run/rule evaluation
    xdp/                      XDP build/apply/map sync manager
  provider/
    api.go                    management HTTP API
    license.go                issue/check/renew/rebind/revoke license logic
    update.go                 signed update manifest publishing
  shared/
    config/                   tenant/advanced/provider config parser
    diagnostics/              status/health/preflight
    installer/                install/uninstall lab
    licenseverify/            Ed25519 license and revocation verification
    update/                   signed update verification/apply

ebpf/
  xdp/
    kiro_xdp_drop.c           L3/L4 XDP/eBPF packet filter

configs/
  management.firewall.vpsgen.com.example.yaml
  provider.example.yaml
  vutrungocrong.fun.example.yaml
  kiro.advanced.example.yaml

deployments/
  systemd/
    kiro-agent.service
    kiro-provider.service
  nginx/
  nftables/
  sysctl/

site/
  kiro-home/                  public project homepage
  kiro-console/               management dashboard UI

scripts/
  build-xdp.sh
  ci-phase10-smoke.sh
  vps-management-server-setup.sh
  vps-provider-client-setup.sh
  vps-xdp-generic-apply-lab.sh
  vps-upload-smoke.sh

docs/
  README.md
  SETUP.md
  SETUP_MASTER.md
  SETUP_CLIENT.md
  PROJECT_STRUCTURE.md
  vi/
  en/
```

Import boundary:

- `kiro-agent` must not import `internal/provider`.
- `kiro-provider` must not import agent firewall/XDP packages.
- Provider private key stays only on the Management Server.
- Client nodes receive only `license.json` and `provider-public-key.pem`.
- `master-server` stores only HMAC hashes of plaintext license keys.
- `client-node` must not contain Master admin secrets.
