# Requirements Document

## Introduction

Hệ thống Kiro WAF hiện tại sử dụng hai định dạng cấu hình khác nhau: `.env` file cho `kiro-client-waf` binary và `.yaml` file cho `kiro-cli`. Điều này gây nhầm lẫn cho người dùng và khó quản lý. Feature này thống nhất cả hai tool đọc từ cùng một file YAML (`/etc/kiro/kiro.yaml`), đồng thời giữ backward compatibility với `.env` file hiện có.

## Glossary

- **Config_Loader**: Module trong `kiro-client-waf` chịu trách nhiệm đọc và parse cấu hình từ YAML file và environment variables
- **YAML_Config_File**: File cấu hình chính tại `/etc/kiro/kiro.yaml` theo cấu trúc đã được định nghĩa trong `configs/kiro.example.yaml`
- **Env_File**: File environment variables legacy tại `/etc/kiro/kiro-client.env` hoặc `/etc/kiro/client-waf.env`
- **Install_Script**: Script cài đặt tự động được serve từ master server, tạo cấu hình ban đầu cho client node
- **Systemd_Service**: File unit systemd tại `/etc/systemd/system/kiro-client-waf.service` quản lý lifecycle của WAF binary
- **Shared_Config_Package**: Package `internal/shared/config` đã có sẵn logic parse YAML, hiện được dùng bởi `kiro-cli`
- **Client_WAF_Binary**: Binary `kiro-client-waf` chạy reverse proxy, challenge pages, rate limiting, và XDP sync
- **Config_Flag**: Command-line flag `--config` chỉ định đường dẫn tới YAML config file

## Requirements

### Requirement 1: YAML Config Loading for Client WAF

**User Story:** As a system administrator, I want `kiro-client-waf` to read configuration from `/etc/kiro/kiro.yaml`, so that I only need to manage one config file for the entire Kiro system.

#### Acceptance Criteria

1. WHEN the `--config` flag is provided, THE Config_Loader SHALL read and parse the YAML file at the specified path
2. WHEN the `--config` flag is not provided, THE Config_Loader SHALL attempt to read `/etc/kiro/kiro.yaml` as the default config path
3. WHEN a valid YAML_Config_File is loaded, THE Config_Loader SHALL map YAML fields to the corresponding `clientConfig` struct fields
4. IF the YAML_Config_File does not exist and no Env_File is present, THEN THE Config_Loader SHALL exit with a fatal error describing both expected paths
5. WHEN the YAML_Config_File contains the `license_key` field, THE Config_Loader SHALL use it as the value for `LicenseKey` in the client config
6. WHEN the YAML_Config_File contains the `website.sites[0].backend` field, THE Config_Loader SHALL use it as the value for `BackendURL` in the client config

### Requirement 2: Environment Variable Override (Backward Compatibility)

**User Story:** As a system administrator with existing `.env` configuration, I want environment variables to still work and override YAML values, so that I can migrate gradually without downtime.

#### Acceptance Criteria

1. WHEN both YAML_Config_File and environment variables are present, THE Config_Loader SHALL use environment variable values to override corresponding YAML values
2. WHEN only the Env_File exists (no YAML_Config_File), THE Config_Loader SHALL load configuration from environment variables as before (full backward compatibility)
3. WHEN an environment variable is set to an empty string, THE Config_Loader SHALL treat it as unset and use the YAML value instead
4. THE Config_Loader SHALL apply the following precedence order: environment variables (highest) > YAML config file > built-in defaults (lowest)

### Requirement 3: YAML Field Mapping

**User Story:** As a developer, I want a clear mapping between YAML fields and the existing `clientConfig` struct, so that the YAML structure matches the documented examples in `configs/kiro.example.yaml`.

#### Acceptance Criteria

1. THE Config_Loader SHALL map `license_key` YAML field to `KIRO_LICENSE_KEY` equivalent
2. THE Config_Loader SHALL map `website.sites[0].backend` YAML field to `KIRO_BACKEND_URL` equivalent
3. THE Config_Loader SHALL map `admin.allow_ips` YAML field to `KIRO_ADMIN_IPS` equivalent
4. THE Config_Loader SHALL map `protection.profile` YAML field to determine rate-limit defaults (RPM per IP, subnet RPM, hard block threshold)
5. WHEN the YAML_Config_File contains a `client` section with WAF-specific fields (cookie_secret, master_url, listen_addr, node_id), THE Config_Loader SHALL map them to the corresponding clientConfig fields
6. THE Config_Loader SHALL support a `client` top-level YAML section for WAF-specific runtime settings not present in the tenant config schema

### Requirement 4: Install Script YAML Generation

**User Story:** As a system administrator running the install script, I want the script to create `/etc/kiro/kiro.yaml` instead of a `.env` file, so that new installations use the unified config from the start.

#### Acceptance Criteria

1. WHEN the Install_Script runs on a fresh system, THE Install_Script SHALL create `/etc/kiro/kiro.yaml` with the required configuration values
2. THE Install_Script SHALL generate a random `cookie_secret` value and include it in the `client` section of the YAML file
3. THE Install_Script SHALL include `license_key`, `master_url`, and `backend_url` values provided during installation in the YAML file
4. WHEN an existing `/etc/kiro/kiro.yaml` file is present, THE Install_Script SHALL preserve the existing file and skip config creation
5. WHEN an existing Env_File is present but no YAML_Config_File exists, THE Install_Script SHALL inform the user that the legacy config is still supported but recommend migrating to YAML

### Requirement 5: Systemd Service Update

**User Story:** As a system administrator, I want the systemd service to pass the config file path via command-line flag, so that the service no longer depends on `EnvironmentFile` directive.

#### Acceptance Criteria

1. THE Systemd_Service SHALL use `ExecStart=/usr/local/bin/kiro-client-waf --config /etc/kiro/kiro.yaml` instead of relying on `EnvironmentFile`
2. THE Systemd_Service SHALL remove the `EnvironmentFile=/etc/kiro/client-waf.env` directive
3. THE Systemd_Service SHALL retain all existing security hardening directives (NoNewPrivileges, ProtectSystem, capabilities)
4. WHEN the systemd service is reloaded after update, THE Systemd_Service SHALL start the Client_WAF_Binary with the YAML config without manual intervention

### Requirement 6: Config Validation at Startup

**User Story:** As a system administrator, I want `kiro-client-waf` to validate the YAML config at startup, so that I get clear error messages for misconfiguration before the service starts serving traffic.

#### Acceptance Criteria

1. WHEN the YAML_Config_File is missing required fields (license_key, cookie_secret, master_url, backend_url), THE Config_Loader SHALL exit with a fatal error listing all missing fields
2. WHEN the YAML_Config_File contains invalid values (malformed URLs, invalid CIDR notation in admin IPs), THE Config_Loader SHALL exit with a descriptive validation error
3. THE Config_Loader SHALL reuse validation logic from the Shared_Config_Package where applicable
4. WHEN validation fails, THE Config_Loader SHALL output the error to stderr with the field name and reason for failure

### Requirement 7: Documentation Update

**User Story:** As a system administrator reading the docs, I want all documentation to reflect the unified YAML config approach, so that there is no confusion between `.env` and `.yaml` formats.

#### Acceptance Criteria

1. THE documentation in `docs/configuration.md` SHALL describe YAML as the primary configuration method for both `kiro-client-waf` and `kiro-cli`
2. THE documentation SHALL include a migration guide section explaining how to convert from `.env` to YAML format
3. THE documentation SHALL document the `client` YAML section with all WAF-specific fields (cookie_secret, master_url, listen_addr, node_id, etc.)
4. THE documentation SHALL note that environment variables still override YAML values for backward compatibility
5. THE documentation SHALL update the "File Paths Summary" table to remove `/etc/kiro/kiro-client.env` as primary config and mark it as legacy/deprecated

### Requirement 8: Config File Parser (YAML to clientConfig)

**User Story:** As a developer, I want a dedicated parser function that converts the YAML structure into the existing `clientConfig` struct, so that the mapping is testable and maintainable.

#### Acceptance Criteria

1. THE Config_Loader SHALL expose a function that accepts a YAML file path and returns a populated `clientConfig` struct or an error
2. THE Config_Loader SHALL produce identical `clientConfig` output whether the values come from YAML or environment variables (given equivalent input values)
3. FOR ALL valid YAML configurations, parsing the YAML into clientConfig and then comparing field-by-field with the equivalent env-var-based config SHALL produce identical results (round-trip equivalence property)
4. THE Config_Loader SHALL handle type conversions: string YAML values to int fields (e.g., `rpm_per_ip: 120`), boolean strings, and duration strings

