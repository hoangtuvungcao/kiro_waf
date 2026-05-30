# Implementation Plan: WAF UI & Experience Overhaul (Đợt 2)

## Overview

Implement 6 modules: Homepage Vietnamese text, dynamic plan config from DB, improved challenge pages, modern CLI output with colors, enhanced license anti-crack, and flexible expiry. Each task builds incrementally, starting with the database layer, then handlers, then UI/CLI, then validation/security enhancements.

## Tasks

- [ ] 1. Plan Config database layer and migration
  - [ ] 1.1 Create `master-server/db/plan_configs.go` with PlanConfigDB struct and DB methods
    - Define `PlanConfigDB` struct with all fields (id, name, display_name, price_usd, price_vnd, rpm_per_ip, subnet_rpm, max_domains, xdp_enabled, ota_enabled, default_days, challenge_mode, description, created_at, updated_at)
    - Implement `ListPlanConfigs()`, `GetPlanConfig(name)`, `UpsertPlanConfig(pc)`, `SeedDefaultPlanConfigs()` methods on `*DB`
    - Add `CREATE TABLE IF NOT EXISTS plan_configs` migration to the DB initialization
    - Seed default plans (community, pro, enterprise) with values from design
    - _Requirements: 2.2, 2.3, 2.5_

  - [ ]* 1.2 Write property test for PlanConfig persistence round-trip
    - **Property 1: PlanConfig Persistence Round-Trip**
    - Use `pgregory.net/rapid` to generate random valid PlanConfig values, upsert to DB, read back by name, assert all fields match
    - Test file: `master-server/db/plan_configs_test.go`
    - **Validates: Requirements 2.2, 2.3, 3.5**

- [ ] 2. Admin Plan Config handlers and templates
  - [ ] 2.1 Create `master-server/handlers/admin_plans.go` with plan CRUD handlers
    - Implement `handleAdminPlans` (GET /admin/plans — list all plans)
    - Implement `handleAdminPlanEdit` (GET /admin/plans/{name} — edit form)
    - Implement `handleAdminPlanUpdate` (POST /admin/plans/{name} — save changes)
    - Register routes in `RegisterAdminRoutes` with session protection
    - _Requirements: 2.1, 2.3_

  - [ ] 2.2 Create admin plan config templates
    - Create `master-server/templates/admin/plans.html` — list view with table of all plans
    - Create `master-server/templates/admin/plan_edit.html` — edit form with all configurable fields
    - Add template render functions in admin templates package
    - Include challenge_mode dropdown (pow, hold, both)
    - _Requirements: 2.1, 2.2, 2.3, 3.5_

  - [ ]* 2.3 Write unit tests for admin plan handlers
    - Test GET /admin/plans returns 200 with plan list
    - Test POST /admin/plans/{name} updates config in DB
    - Test invalid plan name returns 404
    - Test file: `master-server/handlers/admin_plans_test.go`
    - _Requirements: 2.1, 2.3_

- [ ] 3. Heartbeat API enhancement — read plan config from DB
  - [ ] 3.1 Modify `master-server/handlers/api.go` to read plan config from database
    - Replace `models.PlanConfigs[planName]` lookup with `database.GetPlanConfig(planName)` call
    - Add `ChallengeMode` field to `planConfigResponse` struct
    - Remove dependency on hardcoded `models.PlanConfigs` map for heartbeat responses
    - _Requirements: 2.4, 2.5, 5.1_

  - [ ]* 3.2 Write property test for heartbeat returning updated plan config
    - **Property 2: Heartbeat Returns Updated Plan Config**
    - Generate random PlanConfig, store in DB, send heartbeat for license on that plan, assert response plan_config matches DB values
    - Test file: `master-server/handlers/api_test.go`
    - **Validates: Requirements 2.4**

- [ ] 4. Checkpoint — Plan config DB and heartbeat integration
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 5. Heartbeat security enhancements — IP validation and field enforcement
  - [ ] 5.1 Add IP mismatch validation to heartbeat handler
    - In `HandleHeartbeat`, after license lookup, check if `license.ClientIP` is non-empty and differs from request IP
    - If mismatch, respond with `{"valid": false, "lock": true, "reason": "ip_mismatch"}`
    - _Requirements: 5.5_

  - [ ] 5.2 Add mandatory field validation to heartbeat handler
    - Update `heartbeatRequest` struct to include `BinaryHash string` field
    - Validate that `license_key`, `node_id`, and `fingerprint_hash` are all non-empty (fingerprint_hash now mandatory)
    - Return 400 with `{"error": "invalid payload"}` if any mandatory field is missing
    - _Requirements: 5.6_

  - [ ] 5.3 Add request count audit logging
    - Implement `auditRequestCount` function that checks `stats["request_count"]` against plan's `rpm_per_ip`
    - If request_count exceeds limit, log warning with license ID and counts
    - Call from heartbeat handler after successful validation
    - _Requirements: 5.4_

  - [ ]* 5.4 Write property test for IP mismatch rejection
    - **Property 8: IP Mismatch Rejection**
    - Generate license with non-empty client_ip, send heartbeat from different IP, assert valid=false, lock=true, reason="ip_mismatch"
    - Test file: `master-server/handlers/api_test.go`
    - **Validates: Requirements 5.5**

  - [ ]* 5.5 Write property test for heartbeat field validation
    - **Property 9: Heartbeat Request Field Validation**
    - Generate heartbeat requests with one or more mandatory fields empty, assert 400 response
    - Test file: `master-server/handlers/api_test.go`
    - **Validates: Requirements 5.6**

  - [ ]* 5.6 Write property test for request count over-limit warning
    - **Property 7: Request Count Over-Limit Warning**
    - Generate stats with request_count and plan with non-zero rpm_per_ip, assert warning logged iff count > limit
    - Test file: `master-server/handlers/api_test.go`
    - **Validates: Requirements 5.4**

- [ ] 6. Valid days validation and relative expiry formatting
  - [ ] 6.1 Add `ValidateValidDays` function to admin handler
    - Implement validation that accepts only integers in [1, 3650]
    - Integrate into license create and update flows in `master-server/handlers/admin.go`
    - Return flash error if out of range
    - _Requirements: 6.1_

  - [ ] 6.2 Create `FormatRelativeExpiry` helper for admin templates
    - Create `master-server/templates/admin/helpers.go` with `FormatRelativeExpiry(expiresAt, now time.Time) string`
    - Return "còn X ngày" for future dates, "hết hạn Y ngày trước" for past dates
    - Integrate into license list template rendering
    - _Requirements: 6.2_

  - [ ]* 6.3 Write property test for valid days range validation
    - **Property 10: Valid Days Range Validation**
    - Generate random integers, assert accepted iff in [1, 3650]
    - Test file: `master-server/handlers/admin_test.go`
    - **Validates: Requirements 6.1**

  - [ ]* 6.4 Write property test for relative expiry formatting
    - **Property 11: License Expiry Relative Time Formatting**
    - Generate random expiry dates and current times, assert correct "còn X ngày" / "hết hạn Y ngày trước" output
    - Test file: `master-server/templates/admin/helpers_test.go`
    - **Validates: Requirements 6.2**

- [ ] 7. Checkpoint — Security and validation
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 8. CLI terminal color package
  - [ ] 8.1 Create `internal/shared/termcolor` package
    - Create `internal/shared/termcolor/termcolor.go` with `Writer` struct, `ColorMode` enum (Auto, Always, Never)
    - Implement `New(out io.Writer) *Writer` with auto-detection (check NO_COLOR env, terminal isatty)
    - Implement `IsColorEnabled()`, `Color(color, text string) string`
    - Implement `Box(title string, rows [][]string) string` for box-drawing table output
    - Implement `ProgressBar(percent float64, width int) string` for text progress bars
    - Define ANSI constants: Reset, Teal, Green, Red, Yellow, Bold, Dim
    - _Requirements: 4.1, 4.2, 4.3, 4.5_

  - [ ]* 8.2 Write property test for NO_COLOR disabling ANSI
    - **Property 5: NO_COLOR Disables ANSI Escape Sequences**
    - Generate random text content, create Writer with ColorNever mode, assert output contains no `\x1b[` sequences
    - Test file: `internal/shared/termcolor/termcolor_test.go`
    - **Validates: Requirements 4.5**

  - [ ]* 8.3 Write property test for CLI color correctness
    - **Property 4: CLI Formatter Color Correctness**
    - Generate status entries with state in {ok, error, warning}, assert correct ANSI codes (green/red/yellow) and box drawing chars present when color enabled
    - Test file: `internal/shared/termcolor/termcolor_test.go`
    - **Validates: Requirements 4.1, 4.2, 4.3, 4.4**

- [ ] 9. CLI formatters package
  - [ ] 9.1 Create `internal/shared/clifmt` package with output formatters
    - Create `internal/shared/clifmt/clifmt.go` with `OutputMode` enum (OutputText, OutputJSON)
    - Implement `FormatVersion(version string, mode OutputMode, color *termcolor.Writer) string` — product name in teal
    - Implement `FormatStatus(status, mode, color)` — table with colored states (green=OK, red=error, yellow=warning)
    - Implement `FormatReport(report, mode, color)` — CPU, RAM with progress bars, goroutines, uptime
    - Implement `FormatUpdateCheck(result, mode, color)` — green if latest, yellow if update available
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.6_

  - [ ]* 9.2 Write property test for JSON output mode validity
    - **Property 6: JSON Output Mode Produces Valid JSON**
    - Generate random CLI data, format with OutputJSON mode, assert output is valid parseable JSON
    - Test file: `internal/shared/clifmt/clifmt_test.go`
    - **Validates: Requirements 4.6**

- [ ] 10. Integrate CLI formatters into kiro-cli
  - [ ] 10.1 Update `cmd/kiro-cli/main.go` to use termcolor and clifmt
    - Add `--json` flag to all commands (version, status, report, update check)
    - Replace plain `fmt.Println(buildinfo.Version)` with `FormatVersion` call
    - Replace `writeJSON` in `runStatus` with `FormatStatus` (text mode) or keep JSON (--json mode)
    - Replace `writeJSON` in `runReport` with `FormatReport` (text mode) or keep JSON (--json mode)
    - Update `runUpdate` check subcommand to use `FormatUpdateCheck`
    - Detect NO_COLOR env and terminal capability for auto color mode
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6_

- [ ] 11. Checkpoint — CLI output
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 12. Homepage Vietnamese text update
  - [ ] 12.1 Update `master-server/templates/homepage.html` with full Vietnamese diacritics
    - Update all text content to proper Vietnamese with diacritics (e.g., "Bảo vệ", "Giải pháp")
    - Update navigation bar: "Trang chủ", "Tài liệu", "Hướng dẫn cài đặt", "Bảng giá", "Admin"
    - Update pricing section: "Miễn phí", "Không giới hạn", "Hỗ trợ ưu tiên", prices in USD and VND
    - Update install section: "Hướng dẫn cài đặt" with copy button
    - Update footer with proper Vietnamese diacritics
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5_

  - [ ] 12.2 Update homepage to read pricing data from plan_configs DB
    - Modify homepage handler to query `ListPlanConfigs()` from database
    - Pass plan config data to template for dynamic pricing display
    - Display default_days per plan: "30 ngày", "365 ngày", "3650 ngày"
    - _Requirements: 1.3, 6.3_

  - [ ]* 12.3 Write unit tests for homepage Vietnamese content
    - Test homepage response contains Vietnamese diacritics strings
    - Test navigation bar text content
    - Test pricing section renders plan data from DB
    - Test file: `master-server/handlers/homepage_test.go`
    - _Requirements: 1.1, 1.2, 1.3_

- [ ] 13. Challenge pages improvement
  - [ ] 13.1 Update Hold challenge page with progress bar and Vietnamese text
    - Modify `client-node/challenge/hold.go` HTML template
    - Add circular or horizontal progress bar showing hold duration
    - Add Vietnamese text: "Vui lòng bấm và giữ nút bên dưới để xác thực"
    - Make hold duration configurable (2-5 seconds from plan_config)
    - Ensure responsive design and dark theme
    - _Requirements: 3.1, 3.2, 3.6_

  - [ ] 13.2 Update PoW challenge page with progress animation and Vietnamese text
    - Modify `client-node/challenge/pow.go` HTML template
    - Add loading animation and text: "Đang xác thực truy cập..."
    - Add progress bar showing computation progress (0% → 100%)
    - Auto-redirect when complete
    - Ensure responsive design and dark theme
    - _Requirements: 3.3, 3.4, 3.6_

  - [ ] 13.3 Implement challenge mode selection from plan_config
    - Update client WAF to read `challenge_mode` from heartbeat response plan_config
    - Route to appropriate challenge handler based on mode: "pow", "hold", or "both"
    - For "both" mode: PoW first, then Hold if suspicious
    - Default to "pow" if invalid mode received
    - _Requirements: 3.5_

  - [ ]* 13.4 Write property test for ValidHold duration
    - **Property 3: ValidHold Duration Validation**
    - Generate holdSeconds in [2,5] and elapsed durations, assert ValidHold returns true iff elapsed >= holdSeconds
    - Test file: `client-node/challenge/hold_test.go`
    - **Validates: Requirements 3.1**

- [ ] 14. License creation uses plan from DB
  - [ ] 14.1 Update license create form and handler to use DB plans
    - Modify `handleAdminLicenses` create form to populate plan dropdown from `ListPlanConfigs()`
    - Update `adminLicenseCreate` to validate selected plan exists in DB
    - Update license edit form plan dropdown similarly
    - _Requirements: 2.6_

- [ ] 15. Final checkpoint — Full integration
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties from the design document
- Unit tests validate specific examples and edge cases
- The project uses `pgregory.net/rapid` for property-based testing
- Build: `go build ./master-server/ ./cmd/kiro-master/ ./cmd/kiro-client/ ./cmd/kiro-cli/`
- Test: `go test ./...`

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1"] },
    { "id": 1, "tasks": ["1.2", "2.1", "8.1"] },
    { "id": 2, "tasks": ["2.2", "2.3", "3.1", "6.1", "6.2", "8.2", "8.3", "9.1"] },
    { "id": 3, "tasks": ["3.2", "5.1", "5.2", "5.3", "6.3", "6.4", "9.2", "10.1"] },
    { "id": 4, "tasks": ["5.4", "5.5", "5.6", "12.1", "13.1", "13.2"] },
    { "id": 5, "tasks": ["12.2", "12.3", "13.3", "13.4", "14.1"] }
  ]
}
```
