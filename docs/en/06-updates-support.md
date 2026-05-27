# Updates and Support

Updates are file-based and signed.

```text
provider-data/updates/manifests/kiro_1.0.0.json
provider-data/updates/artifacts/kiro-agent_1.0.0_linux_amd64.tar.gz
```

Update flow:

```text
1. Check license.
2. Download manifest.
3. Verify signature.
4. Download artifact.
5. Verify checksum.
6. Create rollback snapshot.
7. Apply update.
8. Run health check.
9. Roll back on failure.
```

Support bundles should include version, mode, license status, sanitized config,
health report, incident timeline, XDP/nftables counters, proxy/WAF summary, and
runtime alerts.

