# Provider and Protected Server Roles

`kiro_waf` has two separate roles.

## Architecture rule

Do not merge provider runtime and protected-server runtime into one process and
switch behavior only by configuration. Roles must be separated by binary,
permissions, and secrets.

```text
kiro-provider
  Runs only on the provider license/update server.
  Can sign licenses and update manifests.
  Has the provider private key.

kiro-agent
  Runs only on the protected customer server.
  Verifies licenses using the provider public key.
  Must not contain provider signing code.

kiro-cli
  Local command tool. Provider commands must call provider APIs or run only with
  provider config on the provider server.
```

## Provider license server

Run by the security provider.

Binary:

```text
kiro-provider
```

Responsibilities:

- Manage customers.
- Issue signed licenses.
- Rebind and revoke licenses.
- Publish signed update manifests.
- Store health reports and incidents.
- Manage service plans.

Provider private key stays only on this server:

```text
/etc/kiro-provider/ed25519-private.key
```

MVP provider commands:

```text
kiro-provider --config configs/provider.example.yaml --check
kiro-provider --config /etc/kiro-provider/provider.yaml gen-dev-keys
kiro-provider --config /etc/kiro-provider/provider.yaml issue-test-license \
  --license-id lic_000001 \
  --customer-id cus_000001 \
  --server-id srv_000001 \
  --plan school_smb \
  --fingerprint-hash sha256:... \
  --agent-out-dir ./agent-license
```

`agent-license/` must contain only:

```text
license.json
provider-public-key.pem
```

It must not contain `ed25519-private.key` or any signing key material.

## Protected server

Run on the customer server.

Binaries:

```text
kiro-agent
kiro-cli
```

Responsibilities:

- Verify license using provider public key.
- Apply XDP/eBPF and nftables.
- Generate proxy config.
- Enforce WAF/bot/resource policies.
- Send health reports if enabled.
- Apply signed updates.

The customer server only stores:

```text
/etc/kiro/license.json
/etc/kiro/provider-public-key.pem
```

It must never store the provider private signing key.

The agent binary must not import provider-only packages.
