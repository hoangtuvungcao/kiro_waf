# Runbook Preflight, Status Và Health

Phần này bổ sung production gate cơ bản: kiểm tra môi trường trước khi apply,
xem trạng thái runtime config, health report cục bộ và đổi mode trong file
config. Các lệnh không apply firewall/proxy.

## Status

```text
go run ./cmd/kiro-cli status --config configs/tenant.full-cloudflare.example.yaml
```

Output là JSON gồm mode, plan, số site/backend pool và các module đang bật:
firewall, proxy, WAF, bot, governor, updates, runtime security.

## Preflight

```text
go run ./cmd/kiro-cli preflight \
  --config configs/tenant.full-cloudflare.example.yaml \
  --preflight-writable-root /tmp/kiro-preflight \
  --skip-command-checks
```

Preflight kiểm tra:

- Linux/Ubuntu target.
- Quyền root nếu cần apply thật.
- Admin allowlist.
- State dir ghi được.
- Tool `nft`, `nginx`, `systemctl` nếu không skip command checks.

Trên máy dev không phải Ubuntu/root có thể có cảnh báo. Trên lab production thật
không nên skip command checks.

## Health

```text
go run ./cmd/kiro-cli health \
  --config configs/tenant.full-cloudflare.example.yaml \
  --preflight-writable-root /tmp/kiro-preflight \
  --skip-command-checks
```

Health gộp config checks và preflight checks, trả `pass`, `warn` hoặc `fail`.

## Mode

Xem mode:

```text
go run ./cmd/kiro-cli mode show --config configs/tenant.server-only.example.yaml
```

Đổi mode trong file config lab:

```text
cp configs/tenant.server-only.example.yaml /tmp/kiro-mode-test.yaml
go run ./cmd/kiro-cli mode set --config /tmp/kiro-mode-test.yaml --mode full
```

Sau khi đổi mode phải chạy `config check` hoặc `health`; `full` mode vẫn cần
website config hợp lệ.

## Giới Hạn

- Preflight hiện là local/lab check, chưa phải installer wizard hoàn chỉnh.
- Không tự cài package thiếu.
- Không thay đổi service systemd/firewall/proxy.
