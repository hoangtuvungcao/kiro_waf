# Runbook Provider File API Lab

Phase 17 thêm HTTP API lab dùng file storage hiện có. API này phục vụ nền cho
portal/provider sau này, chưa phải public SaaS portal hoàn chỉnh.

## Chạy Lab API

```text
go run ./cmd/kiro-provider --config configs/provider.example.yaml \
  serve-lab-api \
  --listen 127.0.0.1:8080 \
  --auth-token dev-token
```

`/healthz` mở để kiểm tra process. Các endpoint khác yêu cầu:

```text
Authorization: Bearer dev-token
```

## Endpoint File-backed

- `GET /updates/{channel}/manifest.json`: đọc
  `storage.root_dir/updates/manifests/{channel}/manifest.json`.
- `GET /revocations/revocations.json`: đọc signed revocation list.
- `GET /licenses/{license_id}.json`: đọc signed license file.
- `POST /health?server_id=srv_1`: ghi health payload JSON vào
  `storage.root_dir/health/`.
- `POST /incidents?server_id=srv_1`: ghi incident payload JSON vào
  `storage.root_dir/incidents/`.
- `POST /retention/purge?dir=health&days=180`: xóa file cũ trong `health`,
  `incidents` hoặc `audit`.

Mọi request được append audit JSONL vào `storage.root_dir/audit/api-YYYY-MM.jsonl`.

## Giới Hạn

- Auth hiện là bearer token tĩnh cho lab.
- Chưa có customer portal UI, RBAC, rate limit, hoặc staged rollout dashboard.
- File storage phù hợp MVP/lab; production nhiều tenant cần backup, lock và
  retention policy được vận hành riêng.
