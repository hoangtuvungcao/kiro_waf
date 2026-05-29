# Runbook Incident Và Support

Runbook này bổ sung commercial gate “support và bảo hành”: tạo incident report
chuẩn, gắn support bundle/health/alert, và có checklist cho các tình huống hỗ
trợ thường gặp.

## Tạo Health Và Support Bundle

```text
mkdir -p /tmp/kiro-incident

go run ./cmd/kiro-cli health \
  --config configs/tenant.full-cloudflare.example.yaml \
  --preflight-writable-root /tmp/kiro-incident/preflight \
  --skip-command-checks \
  > /tmp/kiro-incident/health.json

go run ./cmd/kiro-agent \
  --config configs/tenant.full-cloudflare.example.yaml \
  --support-bundle \
  --support-output-dir /tmp/kiro-incident/support-bundle \
  --support-health-file /tmp/kiro-incident/health.json
```

## Tạo Incident Report

```text
go run ./cmd/kiro-cli incident report \
  --config configs/tenant.full-cloudflare.example.yaml \
  --type attack \
  --severity high \
  --summary "High traffic spike on web/API paths; mitigation under review." \
  --output-dir /tmp/kiro-incident/reports \
  --support-bundle-dir /tmp/kiro-incident/support-bundle \
  --health-file /tmp/kiro-incident/health.json
```

Kết quả:

```text
/tmp/kiro-incident/reports/<incident_id>/incident-report.json
/tmp/kiro-incident/reports/<incident_id>/incident-report.md
```

Markdown report được redact bằng cùng logic support bundle.

## Loại Incident

`--type` hỗ trợ:

- `attack`.
- `lost_ssh`.
- `update_failed`.
- `origin_ip_leaked`.
- `license_rebind`.
- `runtime_security`.
- `other`.

## Checklist Nhanh

Attack:

- Không test attack traffic trên public target.
- Giữ support bundle, health report, event log.
- Kiểm tra governor level, WAF/bot, firewall snapshot, Cloudflare origin lock.
- Ghi rõ mitigation và thời điểm cooldown.

Mất SSH:

- Dùng console/rescue của VPS trước.
- Kiểm tra pending firewall rollback.
- Restore last-good config nếu cần.
- Xác nhận admin CIDR và SSH port trước khi apply lại.

Update lỗi:

- Không confirm pending update.
- Chạy rollback khi pending state còn tồn tại.
- Kiểm tra chữ ký manifest, checksum artifact, release metadata.

Lộ origin IP:

- Kiểm tra DNS có bật proxy Cloudflare.
- Bật/verify Cloudflare origin lock.
- Chặn direct origin HTTP/HTTPS nếu policy yêu cầu.
- Cân nhắc đổi origin IP nếu đã lộ rộng.

License/rebind lỗi:

- Lấy fingerprint mới bằng `kiro-cli license fingerprint`.
- Kiểm tra quota rebind và audit reason.
- Rebind bằng provider, chỉ export `license.json` và public key sang agent.
- Verify license cục bộ trên protected server.

## Giới Hạn

- Incident report là local file artifact, chưa sync lên provider portal.
- Timeline trong Markdown cần operator điền thêm diễn biến thực tế.
- Không thay thế quy trình pháp lý/thông báo khách hàng khi có sự cố nghiêm
  trọng.
