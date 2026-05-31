# Implementation Plan: Unified YAML Config

## Overview

Unify configuration loading for `kiro-client-waf` to read from `/etc/kiro/kiro.yaml` (the same file `kiro-cli` uses), while preserving backward compatibility with environment variables. Implementation proceeds from data structures → loader logic → wiring → install script → docs.

## Tasks

- [x] 1. Define YAML config struct and field mapping
  - [x] 1.1 Create `internal/client/config_yaml.go` with `ClientYAMLConfig` and `ClientSection` structs
    - Define `ClientYAMLConfig` struct with embedded tenant fields (`mode`, `plan`, `license_key`, `admin`, `website`, `protection`) using `yaml` struct tags
    - Define `ClientSection` struct with all WAF-specific runtime fields (`cookie_secret`, `master_url`, `listen_addr`, `node_id`, `pow_difficulty`, `hold_seconds`, `rpm_per_ip`, `subnet_rpm`, `hard_block_after`, `block_ttl_seconds`, `blocklist_file`, `xdp_sync_command`, `heartbeat_seconds`, `update_seconds`, `challenge_all_new`, `transparent_ttl`, `cookie_short_ttl`, `escalation_threshold`, `escalation_cooldown`, `cookie_rate_limit`, `cf_trust_mode`, `xdp_blocked_countries`, `geoip_csv_path`)
    - Import shared config types from `internal/shared/config` for `TenantAdmin`, `TenantServer`, `TenantWebsite`, `TenantProtection`
    - _Requirements: 3.5, 3.6_

- [x] 2. Implement config loader functions
  - [x] 2.1 Create `internal/client/config_load.go` with `LoadClientConfig()`, `loadFromYAML()`, `applyEnvOverrides()`, `applyDefaults()`, and `validateClientConfig()`
    - `LoadClientConfig(yamlPath string) (clientConfig, error)`: orchestrates the full loading pipeline (YAML → env overrides → defaults → validate)
    - `loadFromYAML(path string) (clientConfig, error)`: reads YAML file, unmarshals into `ClientYAMLConfig`, maps fields to `clientConfig` using the documented mapping table
    - Handle `website.sites[0].backend` → `BackendURL` mapping
    - Handle `protection.profile` → rate-limit defaults mapping (`light`/`balanced`/`strict`/`lockdown` → RPMPerIP/SubnetRPM/HardBlockAfter)
    - `applyEnvOverrides(cfg *clientConfig)`: for each field, check corresponding env var; if non-empty, override the config value
    - `applyDefaults(cfg *clientConfig)`: fill zero-value fields with built-in defaults (`:8090`, hostname, `4`, `2`, `900`, etc.)
    - Implement fallback logic: if YAML file doesn't exist but env vars are present, load from env vars only (legacy mode) with a log warning
    - If neither YAML nor env vars exist, return fatal error listing both expected paths
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 2.1, 2.2, 2.3, 2.4, 3.1, 3.2, 3.3, 3.4, 3.5, 8.1, 8.2, 8.4_

  - [x] 2.2 Implement `ConfigValidationError` type and `validateClientConfig()` logic
    - Define `ConfigValidationError` struct with `[]FieldError` collecting ALL validation failures
    - Define `FieldError` struct with `Field` and `Reason` strings
    - Validate required fields: `license_key`, `cookie_secret`, `master_url`, `backend_url` — list all missing in one error
    - Validate URL format for `master_url` and `backend_url` (must be `http://` or `https://` with non-empty host)
    - Validate CIDR format for each entry in `admin_ips`
    - Validate positive integers for rate-limit fields
    - Output errors to stderr with field name and reason
    - _Requirements: 6.1, 6.2, 6.3, 6.4_

  - [ ]* 2.3 Write property tests for config loading in `internal/client/config_load_property_test.go`
    - **Property 1: YAML-to-clientConfig field mapping preserves all values**
    - **Validates: Requirements 1.3, 3.5**
    - Generate random valid `ClientYAMLConfig` with `rapid`, write to temp file, parse via `loadFromYAML`, compare field-by-field

  - [ ]* 2.4 Write property test for config precedence
    - **Property 2: Config precedence — env vars override YAML, YAML overrides defaults**
    - **Validates: Requirements 2.1, 2.3, 2.4**
    - Generate random YAML + random env vars, verify highest-priority non-empty source wins for each field

  - [ ]* 2.5 Write property test for YAML/env-var equivalence
    - **Property 3: YAML/env-var equivalence — identical inputs produce identical configs**
    - **Validates: Requirements 8.2, 8.3**
    - Generate random config values, load via YAML (no env vars) and via env vars (no YAML), assert field-by-field equality

  - [ ]* 2.6 Write property test for validation completeness
    - **Property 4: Validation reports all missing required fields**
    - **Validates: Requirements 6.1, 6.4**
    - Generate random subsets of required fields to omit, verify all missing fields are listed in the error

  - [ ]* 2.7 Write property test for invalid value rejection
    - **Property 5: Validation rejects invalid field values with descriptive errors**
    - **Validates: Requirements 6.2, 6.4**
    - Generate random invalid URLs/CIDRs/integers, verify error contains field name and reason

  - [ ]* 2.8 Write property test for type conversion
    - **Property 6: Type conversion correctness**
    - **Validates: Requirements 8.4**
    - Generate random valid int/bool string representations, verify correct Go typed output

  - [ ]* 2.9 Write unit tests in `internal/client/config_load_test.go`
    - `TestLoadClientConfig_DefaultPath` — no `--config` flag uses default path
    - `TestLoadClientConfig_CustomPath` — reads from specified path
    - `TestLoadClientConfig_LegacyEnvOnly` — no YAML, only env vars → works as before
    - `TestLoadClientConfig_NoConfigSource` — no YAML, no env vars → fatal error
    - `TestLoadClientConfig_ProfileDefaults` — each profile maps to correct RPM values
    - `TestLoadClientConfig_EmptyEnvVarIgnored` — empty env var doesn't override YAML
    - `TestLoadClientConfig_AdminIPsParsing` — comma-separated string and YAML array both work
    - _Requirements: 1.1, 1.2, 1.4, 2.2, 2.3, 3.4, 8.2_

- [x] 3. Checkpoint - Ensure config loader compiles and tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 4. Wire config loader into binary entry point
  - [x] 4.1 Add `--config` flag to `cmd/kiro-client/main.go` and call `RunWithConfig()`
    - Add `flag.String("config", "/etc/kiro/kiro.yaml", "path to YAML config file")` before `flag.Parse()`
    - Replace `os.Exit(client.Run())` with `os.Exit(client.RunWithConfig(*configPath))`
    - Keep `client.Run()` as a wrapper that calls `RunWithConfig("/etc/kiro/kiro.yaml")` for backward compatibility
    - _Requirements: 1.1, 1.2_

  - [x] 4.2 Add `RunWithConfig(configPath string) int` to `internal/client/client_waf.go`
    - Create `RunWithConfig` function that calls `LoadClientConfig(configPath)` instead of `loadConfig()`
    - On error from `LoadClientConfig`, log the error to stderr and return exit code 1
    - On success, proceed with existing WAF startup logic using the returned `clientConfig`
    - Preserve existing `Run()` function as `func Run() int { return RunWithConfig("/etc/kiro/kiro.yaml") }`
    - _Requirements: 1.1, 1.2, 6.4_

- [x] 5. Checkpoint - Ensure binary compiles with new flag and starts correctly
  - Ensure all tests pass, ask the user if questions arise.

- [x] 6. Update install script and systemd service
  - [x] 6.1 Update `internal/master/handlers/install_script_embed.sh` to generate YAML config
    - Replace the `create_config()` function to generate `/etc/kiro/kiro.yaml` instead of `.env` file
    - Generate random `cookie_secret` (40 chars from `/dev/urandom | base64`)
    - Include `license_key`, `master_url`, `backend_url` from install parameters
    - If existing YAML file found, preserve it and skip creation
    - If legacy `.env` file found but no YAML, log warning recommending migration
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5_

  - [x] 6.2 Update `deployments/systemd/kiro-client-waf.service`
    - Change `ExecStart` to `/usr/local/bin/kiro-client-waf --config /etc/kiro/kiro.yaml`
    - Remove `EnvironmentFile=/etc/kiro/client-waf.env` directive
    - Retain all security hardening directives unchanged
    - _Requirements: 5.1, 5.2, 5.3, 5.4_

- [x] 7. Update documentation
  - [x] 7.1 Update `docs/configuration.md` with unified YAML config documentation
    - Describe YAML as the primary configuration method for both `kiro-client-waf` and `kiro-cli`
    - Document the `client` YAML section with all WAF-specific fields
    - Add migration guide section (`.env` → YAML conversion steps)
    - Note that environment variables still override YAML values
    - Update "File Paths Summary" table: mark `/etc/kiro/kiro-client.env` as legacy/deprecated
    - _Requirements: 7.1, 7.2, 7.3, 7.4, 7.5_

  - [x] 7.2 Update `docs/installation.md` to reference YAML config
    - Update install instructions to mention YAML config generation
    - Remove references to `.env` file as primary config
    - _Requirements: 7.1_

  - [x] 7.3 Update `configs/kiro.example.yaml` to include `client` section
    - Add commented `client:` section with all WAF-specific fields and their defaults
    - Add inline comments explaining each field
    - _Requirements: 3.6, 7.3_

- [x] 8. Final checkpoint - Ensure all tests pass and build succeeds
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties from the design document using `pgregory.net/rapid`
- Unit tests validate specific examples and edge cases
- The existing `clientConfig` struct remains unchanged — only the loading mechanism changes
- The `Run()` function is preserved as a backward-compatible wrapper

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1"] },
    { "id": 1, "tasks": ["2.1", "2.2"] },
    { "id": 2, "tasks": ["2.3", "2.4", "2.5", "2.6", "2.7", "2.8", "2.9"] },
    { "id": 3, "tasks": ["4.1", "4.2"] },
    { "id": 4, "tasks": ["6.1", "6.2", "7.1", "7.2", "7.3"] }
  ]
}
```
