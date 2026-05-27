# Cấu Trúc Code Và Module

## Nguyên tắc

Code phải đi theo tài liệu, không tạo module ngẫu nhiên. Mỗi module cần có:

- Interface rõ.
- Config rõ.
- Unit test.
- Error handling.
- Log/audit event.
- Cách rollback nếu module apply thay đổi hệ thống.

## Cấu trúc repo đề xuất

```text
kiro_waf/
├── cmd/
│   ├── kiro-agent/
│   ├── kiro-cli/
│   ├── kiro-provider/
│   └── kiro-gateway/
├── internal/
│   ├── agent/
│   │   ├── admission/
│   │   ├── bot/
│   │   ├── ddos/
│   │   ├── ebpf/
│   │   ├── firewall/
│   │   ├── governor/
│   │   ├── metrics/
│   │   ├── proxy/
│   │   ├── response/
│   │   ├── runtime/
│   │   └── waf/
│   ├── provider/
│   │   ├── activation/
│   │   ├── customer/
│   │   ├── license_issuer/
│   │   ├── revocation/
│   │   └── rollout/
│   └── shared/
│       ├── config/
│       ├── crypto/
│       ├── licenseverify/
│       ├── policy/
│       ├── storage/
│       ├── support/
│       └── update/
├── ebpf/
│   ├── xdp/
│   ├── tc/
│   └── runtime/
├── configs/
├── rules/
├── deployments/
├── examples/
├── docs/
└── tests/
    ├── unit/
    ├── integration/
    ├── lab/
    └── fixtures/
```

## Ranh giới code bắt buộc

Không chỉ đổi config để biến một process thành vai trò khác. Phải tách code theo
vai trò:

```text
cmd/kiro-agent
  được import:
    internal/agent/*
    internal/shared/*
  không được import:
    internal/provider/*

cmd/kiro-provider
  được import:
    internal/provider/*
    internal/shared/*
  không được import:
    internal/agent/firewall
    internal/agent/ebpf
    internal/agent/proxy

cmd/kiro-cli
  được import shared client API.
  command provider phải build/run có điều kiện hoặc gọi kiro-provider API.
```

Lý do:

- Server khách hàng không bao giờ có code ký license trong agent.
- Provider server không cần quyền root để load XDP/nftables.
- Rủi ro lộ private key giảm.
- Dễ audit bảo mật.

## Module bắt buộc cho phase đầu

### shared/config

Chức năng:

- Load YAML.
- Nhận diện simple config và advanced config.
- Expand simple config bằng provider profile thành runtime config đầy đủ.
- Validate schema.
- Validate mode.
- Validate domain/backend reference.
- Validate admin IP.
- Redact secret khi xuất support bundle.

Fail-safe:

- Config lỗi thì không apply.
- Trả lỗi chi tiết cho CLI.

### shared/storage

Chức năng:

- File storage JSON/YAML/JSONL.
- Atomic write.
- File lock.
- Rotate JSONL theo tháng.
- Backup last known good.

Không dùng SQL trong MVP.

### shared/licenseverify

Chức năng:

- Tạo local fingerprint.
- Verify Ed25519 signature.
- Check allowed modes/features.
- Check expiry/grace.
- Emit event khi license sắp hết hạn.

### agent/firewall

Chức năng:

- Generate nftables từ config.
- Dry-run.
- Apply atomic nếu có thể.
- Rollback timer.
- Dynamic sets: temp ban, admin allow, Cloudflare ranges.

### agent/proxy

Chức năng:

- Generate Nginx/HAProxy config từ `sites` và `backend_pools`.
- Validate config.
- Reload proxy an toàn.
- Rollback config cũ nếu reload fail.

### agent/governor

Chức năng:

- Đọc CPU/RAM/load/conntrack/backend latency.
- Tính defense level.
- Hysteresis/cooldown.
- Gọi response action.

### shared/update

Chức năng:

- Download manifest.
- Verify signature.
- Verify checksum.
- Snapshot.
- Apply.
- Health check.
- Rollback.

## Module sau phase đầu

- `agent/ebpf`: loader/maps/events.
- `agent/waf`: Coraza/ModSecurity integration.
- `agent/bot`: scoring/challenge.
- `agent/runtime`: process/file/network monitor.
- `provider/license_issuer`: issue/rebind/revoke license.
- `shared/support`: support bundle, incident report.
- `agent/metrics`: Prometheus/local JSON metrics.

## Quy tắc dependency

```text
cmd/kiro-agent -> internal/agent + internal/shared
cmd/kiro-provider -> internal/provider + internal/shared
internal/shared/config -> không phụ thuộc agent/provider runtime
internal/shared/licenseverify -> storage + crypto, không phụ thuộc firewall/proxy
internal/agent/firewall -> shared/config + shared/storage + support event
internal/agent/proxy -> shared/config + shared/storage
internal/agent/governor -> agent/metrics + agent/response
internal/shared/update -> licenseverify + storage + config
```

Không để module tầng thấp gọi ngược CLI hoặc provider. Không để agent import
provider package.

## Definition of Done cho module

Một module chỉ được coi là xong khi:

- Có config mẫu.
- Có unit test.
- Có test lỗi.
- Có log/audit event.
- Có tài liệu ngắn.
- Có command CLI hoặc integration path.
- Có rollback/fail-safe nếu thay đổi hệ thống.
