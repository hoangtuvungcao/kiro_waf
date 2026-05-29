# Provider File API Lab Runbook

Phase 17 adds a lab HTTP API backed by the existing file storage layout. This is
the foundation for a provider portal, not a complete public SaaS portal.

## Run The Lab API

```text
go run ./cmd/kiro-provider --config configs/provider.example.yaml \
  serve-lab-api \
  --listen 127.0.0.1:8080 \
  --auth-token dev-token
```

`/healthz` is open for process checks. Other endpoints require:

```text
Authorization: Bearer dev-token
```

## File-backed Endpoints

- `GET /updates/{channel}/manifest.json`: reads
  `storage.root_dir/updates/manifests/{channel}/manifest.json`.
- `GET /revocations/revocations.json`: reads the signed revocation list.
- `GET /licenses/{license_id}.json`: reads a signed license file.
- `POST /health?server_id=srv_1`: writes a JSON health payload under
  `storage.root_dir/health/`.
- `POST /incidents?server_id=srv_1`: writes a JSON incident payload under
  `storage.root_dir/incidents/`.
- `POST /retention/purge?dir=health&days=180`: deletes old files under `health`,
  `incidents`, or `audit`.

Every request appends an audit JSONL record to
`storage.root_dir/audit/api-YYYY-MM.jsonl`.

## Limits

- Auth is a static lab bearer token.
- There is no customer portal UI, RBAC, rate limit, or staged rollout dashboard
  yet.
- File storage is suitable for MVP/lab use; multi-tenant production needs
  operated backup, locking, and retention policy.
