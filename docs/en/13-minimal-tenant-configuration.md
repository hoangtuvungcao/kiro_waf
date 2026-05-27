# Minimal Tenant Configuration

For a hosted/rented security product, the full advanced configuration is too
long for normal tenants. The product should use three layers:

```text
tenant simple config
  edited by the customer or support technician

provider profiles
  light, balanced, strict, lockdown

advanced config
  internal or professional support use
```

Normal tenants should only edit:

- mode
- license key
- admin IPs
- interface
- domains
- backend URL
- TLS mode
- protection profile

Example:

```yaml
mode: full
plan: school_smb
license_key: KIRO-XXXX-XXXX

admin:
  allow_ips:
    - 203.0.113.10/32

server:
  interface: eth0
  ssh_port: 22

website:
  enabled: true
  cloudflare: true
  tls_mode: flexible_http
  sites:
    - domains:
        - example.com
      backend: http://127.0.0.1:3000

protection:
  profile: balanced
  waf: true
  bot: true
  auto_attack_mode: true
```

The agent expands this into a full runtime configuration using provider-managed
profiles and signed policy bundles.

