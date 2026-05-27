# Runbook Lab Firewall Apply

Phase 4 chỉ dành cho lab Ubuntu 22.04. Không chạy trực tiếp trên production.

## Điều kiện bắt buộc

- Có console ngoài SSH.
- Có admin CIDR đúng trong config.
- Có `nftables` và lệnh `nft`.
- Đã chạy dry-run và `nft -c` pass.
- Hiểu rằng rule sai có thể khóa SSH.

## Dry-run

```text
go run ./cmd/kiro-agent --config configs/tenant.server-only.example.yaml --firewall-dry-run
```

## Apply trong lab

```text
sudo bash deployments/lab/firewall-apply-ubuntu-22.04.sh configs/tenant.server-only.example.yaml
```

Lệnh apply thật yêu cầu flag xác nhận:

```text
--firewall-lab-ack KIRO_LAB_FIREWALL_APPLY
```

Nếu không có flag này, agent không apply hoặc rollback firewall.

## Confirm

Sau khi apply, kiểm tra:

- SSH từ admin IP còn vào được.
- Web port đúng như mong muốn.
- Không có service quan trọng bị chặn nhầm.

Nếu ổn:

```text
sudo go run ./cmd/kiro-agent \
  --config configs/tenant.server-only.example.yaml \
  --firewall-confirm \
  --firewall-state-dir /var/lib/kiro
```

Confirm sẽ xóa pending rollback state.

## Rollback thủ công

Nếu có lỗi:

```text
sudo go run ./cmd/kiro-agent \
  --config configs/tenant.server-only.example.yaml \
  --firewall-rollback \
  --firewall-lab-ack KIRO_LAB_FIREWALL_APPLY \
  --firewall-state-dir /var/lib/kiro
```

## Rollback khi quá hạn

Agent có thể được gọi bởi systemd timer để rollback nếu pending apply quá hạn:

```text
sudo go run ./cmd/kiro-agent \
  --config configs/tenant.server-only.example.yaml \
  --firewall-rollback-if-expired \
  --firewall-lab-ack KIRO_LAB_FIREWALL_APPLY \
  --firewall-state-dir /var/lib/kiro
```

## File trạng thái

```text
/var/lib/kiro/pending-firewall-apply.json
/var/lib/kiro/last-good-config/last-good-nftables.json
```

`pending-firewall-apply.json` chứa ruleset cũ để rollback. File này phải được
bảo vệ quyền ghi, chỉ root/agent được sửa.

## Recovery nếu mất SSH

1. Mở console của VPS/VM.
2. Chạy rollback thủ công ở trên.
3. Nếu rollback state mất, tạm flush table của kiro:

```text
sudo nft delete table inet kiro_waf
```

4. Sửa admin CIDR trong config.
5. Chạy lại dry-run trước khi apply.
