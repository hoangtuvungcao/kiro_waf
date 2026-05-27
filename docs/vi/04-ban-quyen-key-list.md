# Quản Lý Bản Quyền Và Key List

## Mục tiêu

Nhà cung cấp cần biết rõ:

- Khách hàng nào đang dùng.
- Server nào đã kích hoạt.
- License nào còn hạn.
- Server nào được phép chạy `server` hoặc `full`.
- Server nào được update.
- Lịch sử bảo hành, incident và rebind.

## Không dùng SQL

Bản đơn giản dùng file:

```text
provider-data/
  customers/
    cus_000001.json
  licenses/
    lic_000001.json
  servers/
    srv_000001.json
  activations/
    2026-05.jsonl
  health/
    srv_000001.jsonl
  incidents/
    srv_000001.jsonl
  updates/
    manifests/
      kiro_1.0.0.json
  revocations/
    revocations.json
```

File quan trọng phải có chữ ký số.

## Fingerprint server

Không nên khóa license chỉ bằng MAC. Nên dùng:

```text
/etc/machine-id
MAC của interface default route
hash toàn bộ MAC vật lý
hostname
os/kernel info
provider salt
```

Fingerprint:

```text
SHA256(machine_id + primary_mac + all_macs_hash + provider_salt)
```

## License

License nằm ở:

```text
/etc/kiro/license.json
```

License chứa:

- `license_id`.
- `customer_id`.
- `server_id`.
- `plan`.
- `allowed_modes`.
- `features`.
- `machine_binding`.
- `expires_at`.
- `grace_days`.
- `signature`.

Agent chỉ cần public key của nhà cung cấp để verify:

```text
/etc/kiro/provider-public-key.pem
```

Định dạng đã chốt cho MVP:

- Thuật toán ký: Ed25519.
- Public key: PEM `PUBLIC KEY` hoặc chuỗi `ed25519:<base64 raw public key>`.
- Signature trong `license.json`: `ed25519:<base64 raw signature>`.
- Dữ liệu được ký: JSON canonical của object `payload`, không bao gồm field
  `signature`.
- Agent chỉ verify bằng public key, không có private key và không có quyền issue
  license.

Fingerprint binding:

```text
fingerprint_hash =
  SHA256(machine_id + primary_mac + all_macs_hash + hostname + kernel_release + provider_salt)
```

Trong cấu hình nâng cao, `server_identity.fingerprint_salt_id` là salt do nhà
cung cấp phát hành. Khi cần debug hoặc kích hoạt offline:

```text
kiro-cli license fingerprint --salt default-provider-key-2026
```

## Kích hoạt online

```text
kiro license activate --key KIRO-XXXX-XXXX
```

Luồng:

```text
1. Agent tạo fingerprint.
2. Gửi key + fingerprint tới provider.
3. Provider kiểm tra key.
4. Provider sinh license đã ký số.
5. Agent lưu license.
6. Agent reload feature theo license.
```

## Kích hoạt offline

```text
kiro-cli license fingerprint --salt default-provider-key-2026 > fingerprint.txt
kiro license install license.json
```

Phù hợp server không cho kết nối ra ngoài.

## Rebind khi đổi máy/MAC

```text
kiro license rebind-request
```

Provider kiểm tra:

- License còn hạn.
- Khách hàng hợp lệ.
- Server cũ có còn active không.
- Số lần rebind trong tháng.
- Có dấu hiệu dùng một license cho nhiều máy không.
