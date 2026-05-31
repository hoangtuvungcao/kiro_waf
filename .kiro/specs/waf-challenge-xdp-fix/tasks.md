# Implementation Plan: WAF Challenge & XDP Fix

## Overview

Fix six critical bugs across the WAF's challenge verification system and XDP kernel-level blocking pipeline. The challenge verification handlers set access cookies unconditionally before checking verification results, the loop detector bypasses without granting a cookie (infinite loop), XDP startup is gated on an empty env var, initial config is never written to BPF maps, ban sync errors are silently discarded, and the blocklist file grows unbounded with stale entries.

The fix follows the exploratory bugfix workflow: write tests BEFORE fix to confirm bugs exist, write preservation tests to capture correct baseline behavior, implement the fix, then verify all tests pass.

## Tasks

- [x] 1. Write bug condition exploration test
  - **Property 1: Bug Condition** - Challenge/Hold Verify Cookie Gating & Loop Bypass & XDP Pipeline
  - **CRITICAL**: This test MUST FAIL on unfixed code - failure confirms the bugs exist
  - **DO NOT attempt to fix the test or the code when it fails**
  - **NOTE**: This test encodes the expected behavior - it will validate the fix when it passes after implementation
  - **GOAL**: Surface counterexamples that demonstrate the bugs exist
  - **Scoped PBT Approach**: Scope properties to concrete failing cases for each bug condition
  - Test cases to implement in `internal/client/proxy_bugfix_property_test.go` and `internal/client/ban/engine_bugfix_property_test.go`:
    - Bug 1: Call `handleChallengeVerify` with an invalid/expired nonce → assert response does NOT contain `Set-Cookie` header (from Bug Condition: `input.type == "challenge_verify"` always sets cookie)
    - Bug 2: Call `handleHoldVerify` with insufficient hold duration → assert response does NOT contain `Set-Cookie` header (from Bug Condition: `input.type == "hold_verify"` always sets cookie)
    - Bug 3: Simulate loop bypass (same IP, same challenge >3 times in 10s) → assert response DOES contain `Set-Cookie` header (from Bug Condition: loop bypass proxies without cookie)
    - Bug 6a: Mock `SyncToXDP()` to fail → assert error is logged/returned (from Bug Condition: `_ = e.SyncToXDP()` discards error)
    - Bug 6b: Ban IP, expire it, call `CleanupExpired()` → assert blocklist file does NOT contain the expired IP (from Bug Condition: stale IPs remain in blocklist)
    - Bug 6c: Ban IP, call `Unban()` → assert blocklist file does NOT contain the unbanned IP
  - Run test on UNFIXED code
  - **EXPECTED OUTCOME**: Test FAILS (this is correct - it proves the bugs exist)
  - Document counterexamples found (e.g., "Set-Cookie header present despite failed verification", "No Set-Cookie on loop bypass", "Blocklist contains expired IP after cleanup")
  - Mark task complete when test is written, run, and failure is documented
  - _Requirements: 1.1, 1.2, 1.3, 1.6, 1.7, 1.8_

- [x] 2. Write preservation property tests (BEFORE implementing fix)
  - **Property 2: Preservation** - Existing Correct Behaviors Unchanged
  - **IMPORTANT**: Follow observation-first methodology
  - **Test file**: `internal/client/proxy_preservation_property_test.go` and `internal/client/ban/engine_preservation_property_test.go`
  - Observe on UNFIXED code:
    - `handleTransparentVerify` with valid token → sets cookie, returns HTTP 200 with `{"status":"ok"}`
    - Request with valid `kiro_access` cookie → proxied directly without challenge
    - `IsBanned(ip)` for active ban → returns true
    - `IsBanned(ip)` for expired ban → returns false
    - Challenge serving when loop count ≤ 3 → challenge page served normally
    - `appendToBlocklist(ip)` → writes `IP/32` format to file
    - `SyncToXDP()` with valid command that succeeds → returns nil
  - Write property-based tests capturing observed behavior:
    - Property: For all requests with valid cookies, the proxy serves the backend response without issuing challenges
    - Property: For all active bans, `IsBanned()` returns true; for all expired bans, returns false
    - Property: For all `appendToBlocklist` calls, the file contains the IP in `IP/32` CIDR format
    - Property: For all challenge requests where loop count ≤ 3, the challenge page is served (no bypass)
    - Property: `handleTransparentVerify` sets cookie only on successful verification (already correct)
  - Run tests on UNFIXED code
  - **EXPECTED OUTCOME**: Tests PASS (this confirms baseline behavior to preserve)
  - Mark task complete when tests are written, run, and passing on unfixed code
  - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 3.7_

- [x] 3. Fix Challenge/Hold Verify Cookie Gating in proxy.go

  - [x] 3.1 Fix handleChallengeVerify - move cookie after verification
    - Remove `h.setAccessCookieV2(w, r, ip)` from the top of `handleChallengeVerify`
    - Modify `challenge.VerifyChallenge()` call to capture boolean result (requires checking if it already returns bool or modifying the verify function to return success status)
    - Only set cookie and call `h.escalationEng.RecordSuccess(ip)` if verification succeeds
    - Follow the same pattern as `handleTransparentVerify` (verify first, then cookie on success)
    - _Bug_Condition: isBugCondition(input) where input.type == "challenge_verify" — cookie set unconditionally_
    - _Expected_Behavior: cookie set ONLY when VerifyChallenge returns true_
    - _Preservation: handleTransparentVerify pattern unchanged, valid cookie passthrough unchanged_
    - _Requirements: 1.1, 2.1, 3.1, 3.2_

  - [x] 3.2 Fix handleHoldVerify - move cookie after verification
    - Remove `h.setAccessCookieV2(w, r, ip)` from the top of `handleHoldVerify`
    - Modify `challenge.VerifyHold()` call to capture boolean result
    - Only set cookie and call `h.escalationEng.RecordSuccess(ip)` if verification succeeds
    - Follow the same pattern as `handleTransparentVerify`
    - _Bug_Condition: isBugCondition(input) where input.type == "hold_verify" — cookie set unconditionally_
    - _Expected_Behavior: cookie set ONLY when VerifyHold returns true_
    - _Preservation: handleTransparentVerify pattern unchanged_
    - _Requirements: 1.2, 2.2, 3.1, 3.2_

  - [x] 3.3 Fix Loop Bypass - set short-lived cookie before proxying
    - In `serveChallengeForLevel`, in each case where `h.shouldBypassLoop()` returns true, call `h.setAccessCookieV2(w, r, ip)` BEFORE `h.reverseProxy.ServeHTTP(w, r)`
    - This grants a valid cookie so the next request passes the cookie check and doesn't re-trigger the challenge
    - Apply to all three bypass locations (transparent level 1, pow level 2, hold level 3)
    - _Bug_Condition: isBugCondition(input) where loopDetector.ShouldBypass returns true — no cookie set_
    - _Expected_Behavior: short-lived access cookie set before proxy, preventing infinite loop_
    - _Preservation: Normal challenge serving (loop count ≤ 3) unchanged_
    - _Requirements: 1.3, 2.3, 3.6_

  - [x] 3.4 Verify bug condition exploration test (proxy bugs) now passes
    - **Property 1: Expected Behavior** - Challenge/Hold Verify Cookie Gating & Loop Bypass
    - **IMPORTANT**: Re-run the SAME test from task 1 (proxy-related assertions) - do NOT write a new test
    - The test from task 1 encodes the expected behavior for bugs 1, 2, 3
    - When this test passes, it confirms the expected behavior is satisfied
    - Run bug condition exploration test from step 1 (proxy portion)
    - **EXPECTED OUTCOME**: Test PASSES (confirms bugs 1, 2, 3 are fixed)
    - _Requirements: 2.1, 2.2, 2.3_

  - [x] 3.5 Verify preservation tests still pass (proxy)
    - **Property 2: Preservation** - Proxy Behaviors Unchanged
    - **IMPORTANT**: Re-run the SAME tests from task 2 (proxy-related assertions) - do NOT write new tests
    - Run preservation property tests from step 2 (proxy portion)
    - **EXPECTED OUTCOME**: Tests PASS (confirms no regressions in proxy behavior)
    - Confirm valid cookie passthrough, transparent verify, and challenge serving are unchanged

- [x] 4. Remove XDP Sync Command Gate in client_waf.go

  - [x] 4.1 Remove xdpCommandConfigured gate
    - In `client_waf.go` `Run()` function, remove the line `xdpCommandConfigured := cfg.XDPSyncCommand != ""`
    - In the `OnPlanConfig` callback, change the condition from `if xdpCommandConfigured && XDPStartupFunc != nil` to `if XDPStartupFunc != nil`
    - XDP components (GeoIP, botnet, config sync) use `bpftool` directly and don't need the sync command
    - The sync command is only needed for `ban/engine.go`'s `SyncToXDP()` which is separate
    - _Bug_Condition: isBugCondition(input) where env("KIRO_XDP_SYNC_COMMAND") == "" blocks XDP startup_
    - _Expected_Behavior: XDP starts when plan enables it, regardless of KIRO_XDP_SYNC_COMMAND_
    - _Preservation: When KIRO_XDP_SYNC_COMMAND is set, behavior unchanged (3.5)_
    - _Requirements: 1.4, 2.4, 3.5_

- [x] 5. Add Initial Config Sync in cmd/kiro-client/main.go

  - [x] 5.1 Add ConfigSync initialization and initial sync call
    - In `startXDPComponents`, create a `ConfigSync` instance: `configSync := xdp.NewConfigSync(xdp.ConfigSyncOptions{})`
    - Build an `XDPConfig` struct from environment variables (read `KIRO_XDP_*` env vars or use sensible defaults matching the existing config pattern)
    - Call `configSync.SyncConfig(cfg)` to write initial configuration to the `kiro_config` BPF map
    - Start SYN cookie key rotation: `configSync.StartKeyRotation(ctx)`
    - Log success/failure of initial config sync
    - _Bug_Condition: isBugCondition(input) where configSync.SyncConfig() never called — BPF map all zeros_
    - _Expected_Behavior: BPF map written with non-zero config on startup_
    - _Preservation: GeoIP loader and botnet controller startup unchanged_
    - _Requirements: 1.5, 2.5_

- [x] 6. Fix Ban Engine - Error Propagation & Blocklist Rebuild

  - [x] 6.1 Propagate SyncToXDP error in Ban()
    - Change `_ = e.SyncToXDP()` to `if err := e.SyncToXDP(); err != nil { log.Printf("ban: SyncToXDP failed for ip=%s reason=%s: %v", ip, reason, err) }`
    - Keep `Ban()` signature as `void` (callers don't handle errors currently) but ensure the error is logged
    - _Bug_Condition: isBugCondition(input) where syncToXDP() returns error — discarded with `_`_
    - _Expected_Behavior: error logged with IP and reason context_
    - _Preservation: Successful sync behavior unchanged (3.3)_
    - _Requirements: 1.6, 2.6, 3.3_

  - [x] 6.2 Add rebuildBlocklist helper method
    - Add new method `func (e *InMemoryBanEngine) rebuildBlocklist()` on InMemoryBanEngine
    - Must be called while holding the write lock (caller ensures lock is held)
    - Truncate the blocklist file and rewrite with all currently-banned (non-expired) IPs in `IP/32` format
    - Graceful degradation: log errors but don't crash
    - If `blocklistPath` is empty, return immediately (no-op)
    - _Preservation: appendToBlocklist format unchanged — still writes IP/32 (3.7)_
    - _Requirements: 2.7, 2.8, 3.7_

  - [x] 6.3 Rebuild blocklist on CleanupExpired
    - After the loop that deletes expired entries in `CleanupExpired()`, call `e.rebuildBlocklist()`
    - Call `e.SyncToXDP()` after rebuild (log error if it fails)
    - Note: `rebuildBlocklist` is called while lock is held (CleanupExpired already holds write lock)
    - _Bug_Condition: isBugCondition(input) where cleanup removes from memory but not from file_
    - _Expected_Behavior: blocklist file only contains currently-banned IPs after cleanup_
    - _Requirements: 1.7, 2.7_

  - [x] 6.4 Remove from blocklist on Unban
    - After `delete(e.store, ip)` in `Unban()`, call `e.rebuildBlocklist()`
    - Call `e.SyncToXDP()` after rebuild (log error if it fails)
    - Note: `Unban` already holds write lock when calling delete
    - _Bug_Condition: isBugCondition(input) where unban removes from memory but not from file_
    - _Expected_Behavior: blocklist file does not contain unbanned IP, XDP sync triggered_
    - _Requirements: 1.8, 2.8_

  - [x] 6.5 Verify bug condition exploration test (ban engine bugs) now passes
    - **Property 1: Expected Behavior** - Ban Engine Error Propagation & Blocklist Rebuild
    - **IMPORTANT**: Re-run the SAME test from task 1 (ban engine assertions) - do NOT write a new test
    - The test from task 1 encodes the expected behavior for bugs 6a, 6b, 6c
    - When this test passes, it confirms the expected behavior is satisfied
    - Run bug condition exploration test from step 1 (ban engine portion)
    - **EXPECTED OUTCOME**: Test PASSES (confirms bugs 6a, 6b, 6c are fixed)
    - _Requirements: 2.6, 2.7, 2.8_

  - [x] 6.6 Verify preservation tests still pass (ban engine)
    - **Property 2: Preservation** - Ban Engine Behaviors Unchanged
    - **IMPORTANT**: Re-run the SAME tests from task 2 (ban engine assertions) - do NOT write new tests
    - Run preservation property tests from step 2 (ban engine portion)
    - **EXPECTED OUTCOME**: Tests PASS (confirms no regressions in ban engine)
    - Confirm IsBanned, appendToBlocklist format, and successful sync are unchanged

- [x] 7. Checkpoint - Ensure all tests pass
  - Run full test suite: `go test ./internal/client/... ./cmd/kiro-client/... ./internal/client/ban/...`
  - Ensure all property-based tests (bug condition + preservation) pass
  - Ensure existing unit tests still pass (no regressions)
  - Verify no compile errors across the project: `go build ./...`
  - Ask the user if questions arise

## Notes

- Task 1 (exploration test) and Task 2 (preservation test) MUST be completed BEFORE any implementation tasks (3-6)
- Tasks 3, 4, 5, 6 can be implemented in parallel since they modify different files
- The exploration test in Task 1 is expected to FAIL on unfixed code — this confirms the bugs exist
- The preservation test in Task 2 is expected to PASS on unfixed code — this captures baseline behavior
- After implementation, re-running Task 1 tests should PASS (bugs fixed) and Task 2 tests should still PASS (no regressions)
- Use `pgregory.net/rapid` or `testing/quick` for property-based tests in Go
- Bugs 4 and 5 (XDP startup gate and initial config sync) are harder to unit test in isolation due to BPF map dependencies — consider integration-level testing or mocking the bpftool calls
- The `challenge.VerifyChallenge()` and `challenge.VerifyHold()` functions may need their return signatures checked — they may already return bool or may need modification to support the conditional cookie pattern

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1", "2"] },
    { "id": 1, "tasks": ["3.1", "3.2", "3.3", "4.1", "5.1", "6.1", "6.2"] },
    { "id": 2, "tasks": ["6.3", "6.4"] },
    { "id": 3, "tasks": ["3.4", "3.5", "6.5", "6.6"] },
    { "id": 4, "tasks": ["7"] }
  ]
}
```
