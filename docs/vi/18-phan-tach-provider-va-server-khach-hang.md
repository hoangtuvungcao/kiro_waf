# Phân Tách Provider Server Và Server Khách Hàng

## Mục tiêu

Hệ thống phải tách rõ hai vai trò:

```text
Provider license server
  Server của nhà cung cấp, dùng để quản lý khách hàng, license, update và bảo
  hành.

Protected server
  Server của khách hàng/người dùng bình thường, nơi cài kiro-agent để bảo vệ
  website/server.
```

Không được để người dùng bình thường phải chạy phần provider nếu họ chỉ muốn bảo
vệ một server.

## Kết luận kiến trúc

Không được gộp code chạy runtime giữa provider server và protected server rồi
chỉ đổi cấu hình để biến vai trò. Cần tách binary và quyền hạn:

```text
kiro-provider
  Chỉ chạy ở server nhà cung cấp.
  Có quyền ký license/update.
  Có provider private key.

kiro-agent
  Chỉ chạy ở server khách hàng.
  Không có quyền ký license/update.
  Chỉ verify bằng provider public key.

kiro-cli
  Công cụ điều khiển local. Có thể có subcommand cho agent hoặc provider,
  nhưng command provider chỉ hoạt động khi chạy trên provider server và có
  config provider hợp lệ.
```

Không dùng `node_role` như cách bảo mật chính. `node_role` chỉ để validate
config và cảnh báo nhầm môi trường. Ranh giới bảo mật thật phải nằm ở:

- Binary khác nhau.
- Config path khác nhau.
- Private key chỉ tồn tại ở provider.
- Package provider không được import vào agent binary.
- Agent binary không chứa code ký license bằng private key.

## Provider license server

Chạy bởi nhà cung cấp.

Binary:

```text
kiro-provider
```

Config:

```text
configs/provider.example.yaml
```

Chức năng:

- Quản lý khách hàng.
- Tạo license/key.
- Ký license bằng private key.
- Quản lý server đã kích hoạt.
- Rebind khi đổi máy/MAC.
- Revoke license.
- Phát hành update manifest.
- Lưu health report và incident.
- Quản lý gói dịch vụ: community, school_smb, professional, enterprise_lite.

Dữ liệu:

```text
/var/lib/kiro-provider/
  customers/
  licenses/
  servers/
  activations/
  health/
  incidents/
  updates/
  revocations/
```

Private key của provider chỉ nằm ở provider server:

```text
/etc/kiro-provider/ed25519-private.key
```

Tuyệt đối không copy private key này sang server khách hàng.

Lệnh provider tối thiểu trong MVP:

```text
kiro-provider --config configs/provider.example.yaml --check
kiro-provider --config /etc/kiro-provider/provider.yaml gen-dev-keys
kiro-provider --config /etc/kiro-provider/provider.yaml issue-test-license \
  --license-id lic_000001 \
  --customer-id cus_000001 \
  --server-id srv_000001 \
  --plan school_smb \
  --fingerprint-hash sha256:... \
  --agent-out-dir ./agent-license
```

`agent-license/` chỉ được chứa:

```text
license.json
provider-public-key.pem
```

Không được có `ed25519-private.key` hoặc signing key trong thư mục này.

Không cài trên server khách hàng:

```text
kiro-provider
/etc/kiro-provider/ed25519-private.key
/var/lib/kiro-provider/
```

## Protected server

Chạy trên máy khách hàng.

Binary:

```text
kiro-agent
kiro-cli
```

Config:

```text
/etc/kiro/kiro.yaml
```

Chức năng:

- Verify license bằng provider public key.
- Load XDP/eBPF.
- Apply nftables.
- Generate Nginx/HAProxy config.
- WAF/bot defense.
- Runtime security.
- Resource governor.
- Gửi health report nếu telemetry bật.
- Update bằng manifest đã ký số.

Server khách hàng chỉ có:

```text
/etc/kiro/license.json
/etc/kiro/provider-public-key.pem
```

Không có provider private key.

Không có các chức năng:

- Issue license.
- Rebind license bằng quyền provider.
- Revoke license.
- Ký update manifest.
- Đọc provider private key.
- Quản lý customer list.

## Người dùng bình thường cần làm gì?

Người dùng bình thường chỉ cần:

```text
1. Cài kiro-agent.
2. Chọn mode server hoặc full.
3. Nhập license key hoặc dùng community mode.
4. Nếu full mode: nhập domain và backend.
5. Nếu dùng Cloudflare: bật proxied DNS và origin lock.
6. Chạy health check.
```

Họ không cần biết:

- Cách ký license.
- Cách quản lý key list provider.
- Cách phát hành update manifest.
- Cấu trúc provider-data.

## Luồng kích hoạt

```text
Protected server                  Provider license server
      |                                      |
      | -- activation key + fingerprint ---> |
      |                                      |
      | <--------- signed license ---------- |
      |                                      |
      | verify bằng public key               |
      | bật feature theo license             |
```

## Luồng update

```text
Protected server                  Provider update endpoint
      |                                      |
      | -------- check manifest -----------> |
      |                                      |
      | <------ signed manifest ------------ |
      |                                      |
      | verify signature + checksum          |
      | snapshot + apply + health check      |
```

## Quy tắc bảo mật

- Provider private key không được nằm trên protected server.
- `kiro-agent` không được import package chỉ dành cho provider.
- `kiro-provider` không được chạy như root firewall agent trên server khách hàng.
- Build artifact cho agent không được chứa private signing key, test key thật,
  hoặc provider data.
- Agent API không expose public internet.
- Protected server vẫn chạy được với license/policy cuối cùng nếu provider tạm
  thời offline.
- Revocation chỉ có hiệu lực khi agent nhận được revocation list mới hoặc khi
  license hết hạn/grace.
- Support bundle phải redact secret trước khi gửi provider.
