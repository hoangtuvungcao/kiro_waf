# Readiness Cuối Trước Khi Code

## Kết luận

Tài liệu và cấu hình hiện tại đã đủ để bắt đầu hoàn thiện dự án theo từng phase.
Không nên tiếp tục mở rộng tài liệu trước khi khởi tạo code, vì phần còn thiếu
quan trọng nhất bây giờ là implementation và test thực tế.

Trạng thái hiện tại:

```text
Đủ để bắt đầu code: Có.
Đủ tài liệu định hướng production: Có.
Đủ tài liệu định hướng thương mại: Có.
Đủ để chạy production thật: Chưa, cần implementation + test gate.
Đủ để bán thương mại thật: Chưa, cần production gate + pilot + support/legal.
Đủ để triển khai lab/dev và hoàn thiện theo phase: Có.
```

## Những phần đã chốt

### Phân tách vai trò

Đã chốt rõ:

```text
kiro-provider
  Server nhà cung cấp.
  Quản lý customer, license, update, support.
  Có private signing key.

kiro-agent
  Server khách hàng.
  Bảo vệ server/website.
  Chỉ verify license bằng public key.
  Không có provider private key.

kiro-cli
  Công cụ điều khiển local/API.
```

Không gộp provider runtime và agent runtime rồi chỉ đổi config.

### Cấu hình

Đã có 3 lớp:

```text
configs/kiro.example.yaml
  Config tối giản cho người thuê.

configs/tenant.*.example.yaml
  Mẫu theo tình huống phổ biến.

configs/kiro.advanced.example.yaml
  Config đầy đủ cho provider/kỹ thuật.
```

Provider có config riêng:

```text
configs/provider.example.yaml
```

### Domain/backend

Đã hỗ trợ trong thiết kế:

- Một domain nhiều backend.
- Nhiều domain một backend.
- Một domain nhiều route/backend.
- Cloudflare Flexible HTTP không cần cert/key.
- Full Strict có cert/key cho production nhạy cảm.

### License/update

Đã chốt:

- License ký số.
- Machine fingerprint không chỉ dựa MAC.
- Provider private key chỉ ở provider server.
- Update manifest ký số.
- Artifact checksum.
- Rollback khi update lỗi.

### Test plan

Đã có kế hoạch test từng phase:

- Config/storage.
- License.
- Provider skeleton.
- Firewall dry-run.
- Firewall apply lab.
- Proxy generator.
- Governor.
- WAF/bot.
- Update.
- Runtime security/support bundle.

## Những phần chưa được coi là xong

Các phần sau chỉ mới là thiết kế, chưa có code:

- `kiro-agent`.
- `kiro-provider`.
- `kiro-cli`.
- XDP/eBPF program.
- nftables manager có rollback thật.
- Nginx generator thật.
- WAF integration thật.
- Bot challenge thật.
- Runtime security thật.
- Signed update workflow thật.
- Dashboard.

Vì vậy không được quảng cáo là hệ thống chống tấn công đã hoạt động tốt cho đến
khi có benchmark và test lab.

## Gate bắt buộc khi bắt đầu code

Phase đầu tiên phải làm:

```text
1. Tạo Go module.
2. Tạo cmd/kiro-agent.
3. Tạo cmd/kiro-cli.
4. Tạo cmd/kiro-provider.
5. Tạo internal/shared/config.
6. Tạo internal/shared/storage.
7. Tạo internal/shared/licenseverify.
8. Tạo internal/agent skeleton.
9. Tạo internal/provider skeleton.
10. Test import boundary:
    - agent không import provider
    - provider không import agent/firewall hoặc agent/ebpf
```

## Không làm trước

Không nên làm các phần này trước Phase 0-3:

- Dashboard.
- ML/anomaly phức tạp.
- Database provider.
- Multi-node cluster.
- Plugin marketplace.
- Giao diện quản trị lớn.

Các phần đó dễ làm dự án phình ra trước khi lõi bảo vệ chạy được.

## Definition of Ready

Dự án đã ready để code khi:

- YAML/JSON parse pass.
- README link pass.
- Có threat model.
- Có cấu trúc module.
- Có phase plan.
- Có checklist test.
- Có config tối giản và advanced.
- Có phân tách provider/agent.
- Có production/commercial gate.

Hiện tại các điều kiện này đã đạt.

## Bước tiếp theo

Bắt đầu Phase 0:

```text
go mod init
cmd/kiro-agent
cmd/kiro-cli
cmd/kiro-provider
internal/shared/config
internal/shared/storage
internal/shared/licenseverify
```

Test đầu tiên cần có:

```text
go test ./...
kiro-agent --config configs/kiro.example.yaml --check
kiro-agent --config configs/kiro.advanced.example.yaml --check --skip-license-check
kiro-provider --config configs/provider.example.yaml --check
```
