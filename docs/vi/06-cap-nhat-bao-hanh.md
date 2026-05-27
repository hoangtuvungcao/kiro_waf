# Cập Nhật Và Bảo Hành

## Cập nhật không dùng SQL

Provider phát hành update bằng file manifest đã ký số:

```text
provider-data/updates/manifests/kiro_1.0.0.json
provider-data/updates/artifacts/kiro-agent_1.0.0_linux_amd64.tar.gz
```

Manifest chứa:

- Version.
- Channel: `stable`, `security`, `beta`.
- Artifact URL hoặc đường dẫn SCP.
- Checksum.
- Migration note.
- Rollback artifact.
- Signature.

## Luồng update

```text
1. Agent kiểm tra license.
2. Agent tải manifest.
3. Verify chữ ký manifest.
4. Tải artifact.
5. Kiểm tra checksum.
6. Tạo rollback snapshot.
7. Apply update.
8. Health check.
9. Fail thì rollback.
```

## Bảo hành

Mỗi server nên có support bundle:

```text
kiro support bundle
```

Bundle gồm:

- Version.
- Mode hiện tại.
- License status.
- Config đã ẩn secret.
- Health report.
- Incident timeline.
- XDP/nftables counters.
- Nginx/WAF summary.
- Runtime alerts.

## Health report dạng file

```json
{
  "server_id": "srv_000001",
  "version": "1.0.0",
  "mode": "full",
  "defense_level": "ATTACK",
  "cpu_percent": 82,
  "ram_available_percent": 18,
  "conntrack_percent": 64,
  "xdp_drops": 120000,
  "nft_drops": 30000,
  "waf_blocks": 120,
  "bot_challenges": 900
}
```

