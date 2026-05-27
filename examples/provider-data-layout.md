# Provider Data Layout

MVP provider storage is file-based.

```text
provider-data/
├── customers/
│   └── cus_000001.json
├── licenses/
│   └── lic_000001.json
├── servers/
│   └── srv_000001.json
├── activations/
│   └── 2026-05.jsonl
├── health/
│   └── srv_000001.jsonl
├── incidents/
│   └── srv_000001.jsonl
├── updates/
│   ├── manifests/
│   │   └── kiro_1.0.0.json
│   └── artifacts/
│       └── kiro-agent_linux_amd64.tar.gz
└── revocations/
    └── revocations.json
```

Important files should be signed with the provider Ed25519 private key.

The customer server only needs the provider public key to verify:

```text
/etc/kiro/provider-public-key.pem
```

