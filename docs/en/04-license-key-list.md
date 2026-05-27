# License and Key List

The MVP uses file-based provider data, not SQL.

```text
provider-data/
  customers/
  licenses/
  servers/
  activations/
  health/
  incidents/
  updates/
  revocations/
```

Licenses are bound to a server fingerprint:

```text
SHA256(machine_id + primary_mac + all_macs_hash + provider_salt)
```

Do not bind only to a MAC address because virtual NICs and server migrations can
change it.

The customer server stores:

```text
/etc/kiro/license.json
/etc/kiro/provider-public-key.pem
```

The provider signs license files with Ed25519. The agent verifies licenses with
the provider public key.

