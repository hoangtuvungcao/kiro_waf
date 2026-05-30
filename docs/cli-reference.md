# CLI Reference

## Overview

`kiro-cli` là công cụ dòng lệnh để quản trị Kiro WAF trên local server.

```bash
kiro-cli <command> [subcommand] [options]
```

## Commands

### version

Hiển thị phiên bản hiện tại.

```bash
kiro-cli version
```

**Output:** `0.1.0` (hoặc version hiện tại)

**Exit codes:**
- `0`: Thành công

---

### license

Quản lý license và machine fingerprint.

#### license fingerprint

Tạo machine fingerprint hash để đăng ký license.

```bash
kiro-cli license fingerprint [--salt SALT_ID]
```

**Parameters:**
| Flag | Type | Default | Mô tả |
|------|------|---------|--------|
| `--salt` | string | `""` | Provider fingerprint salt ID |

**Example:**
```bash
kiro-cli license fingerprint --salt default-provider-key-2026
# Output: a1b2c3d4e5f6...
```

**Exit codes:**
- `0`: Thành công
- `1`: Không thể thu thập fingerprint

---

### status

Hiển thị trạng thái hệ thống (JSON output).

```bash
kiro-cli status [--config PATH]
```

**Parameters:**
| Flag | Type | Default | Mô tả |
|------|------|---------|--------|
| `--config` | string | `configs/kiro.example.yaml` | Path đến config file |

**Example:**
```bash
kiro-cli status --config /etc/kiro/kiro.yaml
```

**Output (JSON):**
```json
{
  "mode": "full",
  "plan": "community",
  "services": {
    "nginx": "active",
    "kiro-client-waf": "active"
  },
  "domains": ["example.com"],
  "uptime": "3d 12h 45m"
}
```

**Exit codes:**
- `0`: Thành công
- `1`: Lỗi đọc config
- `2`: Lỗi tham số

---

### health

Kiểm tra sức khỏe toàn diện (status + preflight).

```bash
kiro-cli health [--config PATH] [--os-release PATH] [--skip-command-checks]
```

**Parameters:**
| Flag | Type | Default | Mô tả |
|------|------|---------|--------|
| `--config` | string | `configs/kiro.example.yaml` | Path đến config file |
| `--os-release` | string | `/etc/os-release` | Path đến os-release |
| `--preflight-writable-root` | string | `""` | Writable root cho state-dir check |
| `--skip-command-checks` | bool | `false` | Bỏ qua kiểm tra nft/nginx/systemctl |

**Example:**
```bash
kiro-cli health --config /etc/kiro/kiro.yaml
```

**Exit codes:**
- `0`: Healthy
- `1`: Unhealthy (có lỗi)
- `2`: Lỗi tham số

---

### preflight

Kiểm tra điều kiện tiên quyết trước khi cài đặt/áp dụng.

```bash
kiro-cli preflight [--config PATH] [--os-release PATH] [--skip-command-checks]
```

**Parameters:**
| Flag | Type | Default | Mô tả |
|------|------|---------|--------|
| `--config` | string | `configs/kiro.example.yaml` | Path đến config file |
| `--os-release` | string | `/etc/os-release` | Path đến os-release |
| `--preflight-writable-root` | string | `""` | Writable root cho state-dir check |
| `--skip-command-checks` | bool | `false` | Bỏ qua kiểm tra commands |

**Example:**
```bash
kiro-cli preflight --config /etc/kiro/kiro.yaml
```

**Output (JSON):** Danh sách checks với pass/fail status.

**Exit codes:**
- `0`: Tất cả checks pass
- `1`: Có check fail
- `2`: Lỗi tham số

---

### mode

Xem hoặc thay đổi chế độ hoạt động.

#### mode show

```bash
kiro-cli mode show [--config PATH]
```

**Output:** `full` hoặc `server`

#### mode set

```bash
kiro-cli mode set --mode MODE [--config PATH]
```

**Parameters:**
| Flag | Type | Required | Mô tả |
|------|------|----------|--------|
| `--mode` | string | Yes | Mode mới: `server` hoặc `full` |
| `--config` | string | No | Path đến config file |

**Example:**
```bash
# Xem mode hiện tại
kiro-cli mode show --config /etc/kiro/kiro.yaml

# Chuyển sang server-only mode
kiro-cli mode set --mode server --config /etc/kiro/kiro.yaml
```

**Exit codes:**
- `0`: Thành công
- `1`: Lỗi thay đổi mode
- `2`: Lỗi tham số

---

### install

Quản lý cài đặt và gỡ cài đặt.

#### install plan

Hiển thị kế hoạch cài đặt (dry-run).

```bash
kiro-cli install plan [--config PATH] [--install-root ROOT] [--agent-binary PATH] [--cli-binary PATH]
```

#### install stage-lab

Stage cài đặt vào thư mục lab (không áp dụng thật).

```bash
kiro-cli install stage-lab [--config PATH] --install-root ROOT [--agent-binary PATH] [--cli-binary PATH]
```

#### install apply-lab

Áp dụng cài đặt (yêu cầu xác nhận).

```bash
kiro-cli install apply-lab [--config PATH] --ack KIRO_LAB_INSTALL_APPLY [--install-root ROOT] [--run-steps]
```

**Parameters:**
| Flag | Type | Required | Mô tả |
|------|------|----------|--------|
| `--config` | string | No | Path đến config file |
| `--install-root` | string | No | Root prefix (lab/staging) |
| `--agent-binary` | string | No | Source binary kiro-agent |
| `--cli-binary` | string | No | Source binary kiro-cli |
| `--provider-binary` | string | No | Source binary kiro-provider |
| `--systemd-service` | string | No | Source systemd service file |
| `--ack` | string | Yes (apply) | Xác nhận: `KIRO_LAB_INSTALL_APPLY` |
| `--os-release` | string | No | Path os-release |
| `--skip-os-check` | bool | No | Bỏ qua Ubuntu check (test only) |
| `--run-steps` | bool | No | Chạy command steps |

#### install uninstall-plan

Hiển thị kế hoạch gỡ cài đặt.

```bash
kiro-cli install uninstall-plan [--config PATH] [--purge]
```

#### install uninstall-apply-lab

Gỡ cài đặt.

```bash
kiro-cli install uninstall-apply-lab [--config PATH] --ack KIRO_LAB_UNINSTALL_APPLY [--purge]
```

**Exit codes:**
- `0`: Thành công
- `1`: Lỗi thực thi
- `2`: Lỗi tham số / thiếu ack

---

### update

Quản lý cập nhật OTA.

#### update check

Kiểm tra phiên bản mới.

```bash
kiro-cli update check --master-url URL [--component NAME] [--channel CHANNEL] [--current-version VER]
```

#### update apply

Tải và áp dụng cập nhật.

```bash
kiro-cli update apply --master-url URL --binary-path PATH --service NAME [--component NAME] [--channel CHANNEL]
```

#### update rollback

Rollback về phiên bản trước.

```bash
kiro-cli update rollback --binary-path PATH --service NAME
```

**Parameters:**
| Flag | Type | Required | Mô tả |
|------|------|----------|--------|
| `--master-url` | string | Yes (check/apply) | URL master server |
| `--component` | string | No | Component: `kiro-client-waf` (default) |
| `--channel` | string | No | Channel: `stable` (default) |
| `--current-version` | string | No | Version hiện tại |
| `--binary-path` | string | Yes (apply/rollback) | Path binary cần update |
| `--service` | string | Yes (apply/rollback) | Systemd service name |

**Example:**
```bash
# Kiểm tra update
kiro-cli update check --master-url https://firewall.vpsgen.com

# Áp dụng update
kiro-cli update apply \
  --master-url https://firewall.vpsgen.com \
  --binary-path /usr/local/bin/kiro-client \
  --service kiro-client-waf

# Rollback
kiro-cli update rollback \
  --binary-path /usr/local/bin/kiro-client \
  --service kiro-client-waf
```

**Exit codes:**
- `0`: Thành công (hoặc đã là version mới nhất)
- `1`: Lỗi update/rollback
- `2`: Thiếu tham số bắt buộc

---

### incident

Tạo báo cáo sự cố.

#### incident report

```bash
kiro-cli incident report [--config PATH] [--output-dir DIR] [--type TYPE] [--severity SEV] [--summary TEXT]
```

**Parameters:**
| Flag | Type | Default | Mô tả |
|------|------|---------|--------|
| `--config` | string | `configs/kiro.example.yaml` | Config file |
| `--output-dir` | string | `$state_dir/incidents` | Output directory |
| `--incident-id` | string | auto-generated | ID sự cố |
| `--type` | string | `other` | Loại: `attack`, `lost_ssh`, `update_failed`, `origin_ip_leaked`, `license_rebind`, `runtime_security`, `other` |
| `--severity` | string | `medium` | Mức độ: `low`, `medium`, `high`, `critical` |
| `--status` | string | `open` | Trạng thái: `open`, `investigating`, `resolved` |
| `--summary` | string | `""` | Tóm tắt sự cố |
| `--started-at` | string | `""` | Thời gian bắt đầu (RFC3339) |
| `--detected-at` | string | `""` | Thời gian phát hiện (RFC3339) |
| `--health-file` | string | `""` | Health report JSON |
| `--alerts-file` | string | `""` | Runtime alerts JSONL |

**Example:**
```bash
kiro-cli incident report \
  --config /etc/kiro/kiro.yaml \
  --type attack \
  --severity high \
  --summary "DDoS attack detected from subnet 192.168.0.0/16"
```

**Exit codes:**
- `0`: Report tạo thành công
- `1`: Lỗi tạo report
- `2`: Lỗi tham số

---

### pilot

Tạo báo cáo pilot (thử nghiệm).

#### pilot report

```bash
kiro-cli pilot report [--config PATH] [--output-dir DIR] [--server-count N] [--started-at TIME] [--ended-at TIME]
```

**Parameters:**
| Flag | Type | Default | Mô tả |
|------|------|---------|--------|
| `--config` | string | `configs/kiro.example.yaml` | Config file |
| `--output-dir` | string | `$state_dir/pilot-reports` | Output directory |
| `--pilot-id` | string | auto-generated | Pilot ID |
| `--server-count` | int | `0` | Số server pilot |
| `--started-at` | string | `""` | Thời gian bắt đầu (RFC3339) |
| `--ended-at` | string | `""` | Thời gian kết thúc (RFC3339) |
| `--health-file` | string | `""` | Health report evidence |
| `--benchmark-file` | string | `""` | Benchmark report |
| `--incident-dir` | string | `""` | Incident drill directory |

**Exit codes:**
- `0`: Report tạo thành công
- `1`: Lỗi
- `2`: Lỗi tham số

---

### report

Tạo system report tổng hợp.

```bash
kiro-cli report [--config PATH]
```

**Example:**
```bash
kiro-cli report --config /etc/kiro/kiro.yaml | jq .
```

**Exit codes:**
- `0`: Thành công
- `1`: Lỗi

---

## Global Patterns

### JSON Output

Tất cả commands (trừ `version` và `mode show`) output JSON. Dùng `jq` để parse:

```bash
kiro-cli status --config /etc/kiro/kiro.yaml | jq '.services'
kiro-cli health --config /etc/kiro/kiro.yaml | jq '.checks[] | select(.pass == false)'
```

### Config Path

Mặc định dùng `configs/kiro.example.yaml`. Trong production luôn chỉ định:

```bash
kiro-cli <command> --config /etc/kiro/kiro.yaml
```

### Exit Codes Summary

| Code | Ý nghĩa |
|------|----------|
| `0` | Thành công |
| `1` | Lỗi runtime (config invalid, operation failed) |
| `2` | Lỗi usage (thiếu tham số, command không hợp lệ) |
