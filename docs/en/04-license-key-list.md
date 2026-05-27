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

MVP license format:

- Public key: PEM `PUBLIC KEY` or `ed25519:<base64 raw public key>`.
- License signature: `ed25519:<base64 raw signature>`.
- Signed bytes: canonical JSON of the `payload` object only.
- The protected-server agent verifies licenses only. It never stores the
  provider private key and cannot issue licenses.

Fingerprint binding:

```text
fingerprint_hash =
  SHA256(machine_id + primary_mac + all_macs_hash + hostname + kernel_release + provider_salt)
```

For offline activation or support debugging:

```text
kiro-cli license fingerprint --salt default-provider-key-2026
```
