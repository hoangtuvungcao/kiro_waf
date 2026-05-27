# Bảo Mật Hệ Thống

## Hardening Ubuntu 22.04 LTS

Nên bật:

- SSH key, hạn chế password login.
- SSH chỉ allow admin IP nếu có thể.
- AppArmor.
- auditd hoặc eBPF runtime monitor.
- nftables mặc định drop.
- journald/logrotate giới hạn dung lượng.
- systemd hardening cho service.

## systemd hardening

```ini
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictSUIDSGID=true
LockPersonality=true
ReadWritePaths=/var/lib/kiro /var/log/kiro /run/kiro
```

Service cần eBPF/nftables có thể phải giữ một số capability cần thiết, nhưng app
khách hàng không nên chạy root.

## Runtime alert

Cảnh báo khi:

- Web user chạy `sh`, `bash`, `curl`, `wget`, `nc`, `python`, `perl`.
- Process web mở outbound connection lạ.
- File thực thi mới xuất hiện trong webroot.
- `.env`, private key hoặc backup bị đọc bất thường.
- `sudoers`, `passwd`, `shadow`, SSH config bị sửa.
- Cron hoặc systemd unit mới xuất hiện.

## Bảo vệ dữ liệu ứng dụng

`kiro_waf` không dùng SQL cho state của chính nó, nhưng vẫn cần bảo vệ database
của ứng dụng khách hàng:

- Database không public internet.
- App không dùng user database quyền admin.
- Giới hạn connection pool.
- Query timeout.
- Không log password/token/session.
- Backup nằm ngoài webroot và được mã hóa.
- Chặn outbound lạ từ web process nếu không cần.

