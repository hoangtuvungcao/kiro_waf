# WAF Challenge & XDP Fix - Bugfix Design

## Overview

This design addresses six critical bugs across the WAF's challenge verification system and XDP kernel-level blocking pipeline. The challenge verification handlers (`handleChallengeVerify`, `handleHoldVerify`) set access cookies unconditionally before checking verification results, allowing bots to bypass protection. The loop detector bypasses challenges without granting a cookie, creating infinite redirect loops. On the XDP side, startup is gated on an empty env var, initial config is never written to BPF maps, ban sync errors are silently discarded, and the blocklist file grows unbounded with stale entries.

The fix strategy is minimal and targeted: reorder cookie-setting logic in verify handlers, grant a short-lived cookie on loop bypass, remove the env var gate for XDP startup, call `ConfigSync.SyncConfig()` during initialization, propagate `SyncToXDP()` errors, and rebuild the blocklist file on cleanup/unban.

## Glossary

- **Bug_Condition (C)**: The set of conditions that trigger any of the six bugs — premature cookie setting, missing cookie on loop bypass, XDP startup blocked by empty env var, missing initial config sync, silenced sync errors, or unbounded blocklist growth
- **Property (P)**: The desired correct behavior — cookies only granted on successful verification, loop bypass grants short-lived cookie, XDP starts when plan enables it regardless of env var, initial config is synced to BPF map, sync errors are logged/returned, blocklist is rebuilt on cleanup/unban
- **Preservation**: Existing correct behaviors that must remain unchanged — transparent verify flow, valid cookie passthrough, successful sync command execution, ban enforcement, normal challenge serving, blocklist append format
- **handleChallengeVerify**: Handler in `proxy.go` for `/__kiro/challenge/verify` that validates PoW solutions
- **handleHoldVerify**: Handler in `proxy.go` for `/__kiro/hold/verify` that validates hold-to-confirm duration
- **LoopDetector**: Component in `challenge/loopdetect.go` that detects redirect loops when the same challenge is issued >3 times in 10s
- **ConfigSync**: Component in `xdp/config_sync.go` that serializes and writes XDP configuration to BPF maps
- **InMemoryBanEngine**: Component in `ban/engine.go` that manages IP bans with in-memory store and blocklist file sync
- **startXDPComponents**: Function in `cmd/kiro-client/main.go` that initializes GeoIP loader and botnet controller

## Bug Details

### Bug Condition

The bugs manifest across two subsystems (challenge verification and XDP pipeline) with six distinct failure modes that collectively render the WAF's protection mechanisms non-functional.

**Formal Specification:**
```
FUNCTION isBugCondition(input)
  INPUT: input of type WAFOperation
  OUTPUT: boolean
  
  // Bug 1 & 2: Cookie set before verification check
  IF input.type == "challenge_verify" OR input.type == "hold_verify"
    RETURN true  // cookie is ALWAYS set regardless of verification outcome
  END IF
  
  // Bug 3: Loop bypass without cookie
  IF input.type == "request" AND loopDetector.ShouldBypass(input.ip, input.challengeType)
    RETURN true  // request proxied without cookie, next request re-triggers challenge
  END IF
  
  // Bug 4: XDP startup gated on empty env var
  IF input.type == "xdp_startup" AND env("KIRO_XDP_SYNC_COMMAND") == ""
    RETURN true  // xdpCommandConfigured = false, XDP never starts
  END IF
  
  // Bug 5: No initial config sync
  IF input.type == "xdp_startup" AND configSync.SyncConfig() never called
    RETURN true  // BPF map has all zeros, all XDP features disabled
  END IF
  
  // Bug 6a: SyncToXDP error silenced
  IF input.type == "ban" AND syncToXDP() returns error
    RETURN true  // error discarded with `_ = e.SyncToXDP()`
  END IF
  
  // Bug 6b: Blocklist never rebuilt
  IF input.type == "cleanup_expired" OR input.type == "unban"
    RETURN true  // stale IPs remain in blocklist file
  END IF
  
  RETURN false
END FUNCTION
```

### Examples

- **Bug 1**: Client sends invalid nonce to `/__kiro/challenge/verify` → `VerifyChallenge()` returns false → but cookie was already set → client has valid access cookie despite failed verification
- **Bug 2**: Client sends hold request with duration < 2s to `/__kiro/hold/verify` → `VerifyHold()` returns false → but cookie was already set → client bypasses hold requirement
- **Bug 3**: IP 1.2.3.4 triggers PoW challenge 4 times in 10s → LoopDetector.ShouldBypass returns true → request proxied to backend without cookie → next request has no cookie → challenge re-triggered → infinite loop
- **Bug 4**: Server starts with `KIRO_XDP_SYNC_COMMAND=""` and plan has `xdp_enabled=true` → `xdpCommandConfigured = false` → XDP components never start → no kernel-level protection
- **Bug 5**: XDP starts successfully → GeoIP loader and botnet controller run → but `kiro_config` BPF map is all zeros → rate limiting disabled, GeoIP disabled, all thresholds at 0
- **Bug 6a**: `Ban("1.2.3.4", 15m, "escalation")` → `appendToBlocklist` succeeds → `SyncToXDP()` fails (command not found) → error discarded → IP only blocked at L7, not at kernel level
- **Bug 6b**: IP 1.2.3.4 banned for 15 minutes → written to blocklist → ban expires → `CleanupExpired()` removes from memory → blocklist file still contains `1.2.3.4/32` → file grows indefinitely

## Expected Behavior

### Preservation Requirements

**Unchanged Behaviors:**
- Transparent challenge verify (`/__kiro/transparent/verify`) flow: verify first, then set cookie on success, return `{"status":"ok"}` — this is already correctly implemented
- Valid `kiro_access` cookie passthrough: requests with valid cookies are proxied directly without challenges
- Successful `SyncToXDP()` execution: when sync command is configured and succeeds, it continues to work as before
- `IsBanned()` check: banned IPs continue to receive HTTP 403
- Normal challenge serving: when LoopDetector has NOT detected a loop (≤3 times in 10s), challenges are served normally
- `appendToBlocklist` format: newly banned IPs continue to be written as `IP/32` CIDR format
- `KIRO_XDP_SYNC_COMMAND` when set: continues to be used for sync operations

**Scope:**
All inputs that do NOT involve the six bug conditions should be completely unaffected by this fix. This includes:
- Requests from IPs with valid cookies
- Requests to passthrough paths (`/install`, `/api/`, `/healthz`, etc.)
- Ban checks for currently-banned IPs
- Challenge page serving (GET endpoints)
- Rate limiting and escalation logic
- Cookie refresh logic
- GeoIP and botnet controller operation (once started)

## Hypothesized Root Cause

Based on the code analysis, the root causes are:

1. **Premature Cookie Setting (Bugs 1 & 2)**: In `proxy.go`, `handleChallengeVerify` and `handleHoldVerify` call `h.setAccessCookieV2(w, r, ip)` unconditionally at the top of the function, before calling `challenge.VerifyChallenge()` or `challenge.VerifyHold()`. The `handleTransparentVerify` handler is correctly implemented (checks result first), suggesting the other two were written without following the same pattern.

2. **Missing Cookie on Loop Bypass (Bug 3)**: In `serveChallengeForLevel`, when `h.shouldBypassLoop()` returns true, the code calls `h.reverseProxy.ServeHTTP(w, r)` directly without setting a cookie. The next request from the same IP will again lack a valid cookie, re-entering the challenge flow and creating an infinite loop.

3. **XDP Startup Gate (Bug 4)**: In `client_waf.go`, the variable `xdpCommandConfigured := cfg.XDPSyncCommand != ""` gates XDP startup. The `OnPlanConfig` callback requires BOTH `xdpCommandConfigured` AND `planCfg.XDPEnabled` to be true. Since `KIRO_XDP_SYNC_COMMAND` defaults to empty string, XDP never starts even when the plan enables it. The XDP components (GeoIP, botnet, config sync) use `bpftool` directly and don't actually need the sync command.

4. **Missing Initial Config Sync (Bug 5)**: `startXDPComponents` in `main.go` starts GeoIP loader and botnet controller but never instantiates `ConfigSync` or calls `SyncConfig()`. The BPF `kiro_config` map remains at all-zero values, effectively disabling all XDP features.

5. **Silenced Sync Error (Bug 6a)**: In `ban/engine.go`, the `Ban()` method calls `_ = e.SyncToXDP()`, explicitly discarding the error. This means XDP-level blocking silently fails while L7 blocking succeeds, creating an inconsistent security posture.

6. **Unbounded Blocklist Growth (Bug 6b)**: `appendToBlocklist` only appends to the file. Neither `CleanupExpired()` nor `Unban()` removes entries from the blocklist file. The file grows indefinitely with stale entries that may cause incorrect XDP blocking of previously-banned IPs.

## Correctness Properties

Property 1: Bug Condition - Challenge Verify Cookie Gating

_For any_ challenge verification request (PoW or Hold) where the verification function returns false (invalid nonce, expired token, insufficient hold duration), the fixed handlers SHALL NOT set an access cookie, ensuring that failed verifications do not grant access.

**Validates: Requirements 2.1, 2.2**

Property 2: Bug Condition - Loop Bypass Cookie Grant

_For any_ request where the LoopDetector determines a challenge should be bypassed (same challenge issued >3 times in 10s), the fixed code SHALL set a short-lived access cookie before proxying to the backend, preventing infinite re-triggering of the challenge on subsequent requests.

**Validates: Requirements 2.3**

Property 3: Bug Condition - XDP Startup Independence

_For any_ startup where the plan's `xdp_enabled` flag is true, the fixed code SHALL start XDP components (GeoIP loader, botnet controller, config sync) regardless of whether `KIRO_XDP_SYNC_COMMAND` is set, since XDP components use `bpftool` directly for BPF map operations.

**Validates: Requirements 2.4, 2.5**

Property 4: Bug Condition - Ban Sync Error Propagation

_For any_ `Ban()` call where `SyncToXDP()` returns a non-nil error, the fixed code SHALL log the error with sufficient context for debugging and return it to the caller, making the failure observable.

**Validates: Requirements 2.6**

Property 5: Bug Condition - Blocklist Rebuild on Cleanup

_For any_ `CleanupExpired()` or `Unban()` call that removes IPs from the in-memory store, the fixed code SHALL rebuild the blocklist file to contain only currently-banned IPs and trigger an XDP sync, ensuring stale entries are removed.

**Validates: Requirements 2.7, 2.8**

Property 6: Preservation - Existing Correct Behaviors

_For any_ input where none of the six bug conditions hold (valid cookie requests, transparent verify, successful syncs, normal challenge serving, ban checks), the fixed code SHALL produce exactly the same behavior as the original code, preserving all existing functionality.

**Validates: Requirements 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 3.7**

## Fix Implementation

### Changes Required

Assuming our root cause analysis is correct:

**File**: `internal/client/proxy.go`

**Functions**: `handleChallengeVerify`, `handleHoldVerify`, `serveChallengeForLevel`

**Specific Changes**:

1. **Fix handleChallengeVerify** — Move cookie setting after verification check:
   - Remove `h.setAccessCookieV2(w, r, ip)` from the top of the function
   - Call `challenge.VerifyChallenge()` and capture the boolean result
   - Only set cookie and record escalation success if verification returns true
   - Follow the same pattern as `handleTransparentVerify`

2. **Fix handleHoldVerify** — Move cookie setting after verification check:
   - Remove `h.setAccessCookieV2(w, r, ip)` from the top of the function
   - Call `challenge.VerifyHold()` and capture the boolean result
   - Only set cookie and record escalation success if verification returns true

3. **Fix Loop Bypass** — Set short-lived cookie on bypass:
   - In `serveChallengeForLevel`, when `h.shouldBypassLoop()` returns true, call `h.setAccessCookieV2(w, r, ip)` before proxying to backend
   - This grants a valid cookie so the next request passes the cookie check

---

**File**: `internal/client/client_waf.go`

**Function**: `Run()` (OnPlanConfig callback)

**Specific Changes**:

4. **Remove XDP Sync Command Gate** — Allow XDP startup without sync command:
   - Remove the `xdpCommandConfigured` variable
   - In the `OnPlanConfig` callback, start XDP components when `planCfg.XDPEnabled` is true AND `XDPStartupFunc != nil`, regardless of `KIRO_XDP_SYNC_COMMAND`
   - The sync command is only needed for `ban/engine.go`'s `SyncToXDP()`, not for XDP component startup

---

**File**: `cmd/kiro-client/main.go`

**Function**: `startXDPComponents`

**Specific Changes**:

5. **Add Initial Config Sync** — Write XDP config to BPF map on startup:
   - Create a `ConfigSync` instance using `xdp.NewConfigSync()`
   - Build an `XDPConfig` struct from environment variables or defaults
   - Call `configSync.SyncConfig(cfg)` to write initial configuration to the `kiro_config` BPF map
   - Start the SYN cookie key rotation via `configSync.StartKeyRotation(ctx)`
   - Accept XDP config parameters (or pass them through the startup function signature)

---

**File**: `internal/client/ban/engine.go`

**Functions**: `Ban`, `CleanupExpired`, `Unban`

**Specific Changes**:

6. **Propagate SyncToXDP Error** — Log and return error from Ban():
   - Change `_ = e.SyncToXDP()` to capture the error
   - Log the error with `log.Printf` including the IP and reason
   - Change `Ban()` signature to return `error` (or keep void and just log, depending on caller expectations — logging is the minimum fix since callers don't currently handle the return)

7. **Rebuild Blocklist on CleanupExpired** — Remove stale entries:
   - After deleting expired entries from the in-memory store, call a new `rebuildBlocklist()` method
   - `rebuildBlocklist()` writes all currently-banned IPs (from the store) to the blocklist file, replacing its contents
   - Trigger `SyncToXDP()` after rebuild to update kernel maps

8. **Remove from Blocklist on Unban** — Clean up single entry:
   - After deleting the IP from the in-memory store, call `rebuildBlocklist()`
   - Trigger `SyncToXDP()` to update kernel maps

9. **Add rebuildBlocklist helper** — New method on InMemoryBanEngine:
   - Truncate and rewrite the blocklist file with all currently-banned IPs in `IP/32` format
   - Must be called while holding the write lock (or acquire it internally)
   - Graceful degradation: log errors but don't crash

## Testing Strategy

### Validation Approach

The testing strategy follows a two-phase approach: first, surface counterexamples that demonstrate the bugs on unfixed code, then verify the fix works correctly and preserves existing behavior.

### Exploratory Bug Condition Checking

**Goal**: Surface counterexamples that demonstrate the bugs BEFORE implementing the fix. Confirm or refute the root cause analysis. If we refute, we will need to re-hypothesize.

**Test Plan**: Write unit tests that exercise each bug condition and assert the expected (correct) behavior. Run these tests on the UNFIXED code to observe failures and confirm the bugs exist.

**Test Cases**:
1. **Challenge Verify Cookie on Failure**: Submit invalid nonce to `handleChallengeVerify`, assert no `Set-Cookie` header in response (will fail on unfixed code — cookie is set unconditionally)
2. **Hold Verify Cookie on Failure**: Submit hold with duration < 2s to `handleHoldVerify`, assert no `Set-Cookie` header in response (will fail on unfixed code)
3. **Loop Bypass Infinite Loop**: Simulate 5 requests from same IP triggering same challenge, assert that after bypass the response includes a `Set-Cookie` header (will fail on unfixed code — no cookie set)
4. **XDP Startup Without Env Var**: Set `KIRO_XDP_SYNC_COMMAND=""` with plan `xdp_enabled=true`, assert XDP components start (will fail on unfixed code)
5. **Missing Config Sync**: Call `startXDPComponents`, assert `kiro_config` BPF map is written (will fail on unfixed code — never called)
6. **Ban Sync Error Visibility**: Mock `SyncToXDP()` to return error, assert error is logged (will fail on unfixed code — error discarded)
7. **Blocklist Stale Entries**: Ban IP, wait for expiry, call `CleanupExpired()`, assert blocklist file does not contain the expired IP (will fail on unfixed code)

**Expected Counterexamples**:
- `Set-Cookie` header present in response even when verification fails
- No `Set-Cookie` header on loop bypass responses
- XDP startup function never called when env var is empty
- `kiro_config` BPF map never written
- No log output when `SyncToXDP()` fails
- Blocklist file contains expired IPs after cleanup

### Fix Checking

**Goal**: Verify that for all inputs where the bug condition holds, the fixed function produces the expected behavior.

**Pseudocode:**
```
FOR ALL input WHERE isBugCondition(input) DO
  result := fixedFunction(input)
  ASSERT expectedBehavior(result)
END FOR
```

Specifically:
- For failed verifications: assert no cookie in response headers
- For successful verifications: assert cookie IS set in response headers
- For loop bypass: assert cookie in response headers AND request proxied
- For XDP startup: assert components start when plan enables XDP
- For config sync: assert BPF map written with non-zero config
- For ban sync: assert error is logged and returned
- For cleanup/unban: assert blocklist file only contains active bans

### Preservation Checking

**Goal**: Verify that for all inputs where the bug condition does NOT hold, the fixed function produces the same result as the original function.

**Pseudocode:**
```
FOR ALL input WHERE NOT isBugCondition(input) DO
  ASSERT originalFunction(input) = fixedFunction(input)
END FOR
```

**Testing Approach**: Property-based testing is recommended for preservation checking because:
- It generates many test cases automatically across the input domain
- It catches edge cases that manual unit tests might miss
- It provides strong guarantees that behavior is unchanged for all non-buggy inputs

**Test Plan**: Observe behavior on UNFIXED code first for valid-cookie requests, transparent verify, and normal challenge serving, then write property-based tests capturing that behavior.

**Test Cases**:
1. **Valid Cookie Passthrough Preservation**: Generate random requests with valid cookies, verify they are proxied without challenge on both original and fixed code
2. **Transparent Verify Preservation**: Verify that `handleTransparentVerify` continues to set cookie only on success (already correct)
3. **Ban Check Preservation**: Verify that `IsBanned()` continues to return true for active bans and false for expired bans
4. **Challenge Serving Preservation**: Verify that challenge pages are served normally when loop count ≤ 3
5. **Blocklist Append Format Preservation**: Verify that newly banned IPs are still written as `IP/32` format
6. **Successful Sync Preservation**: Verify that when sync command succeeds, behavior is unchanged

### Unit Tests

- Test `handleChallengeVerify` with valid nonce → cookie set, HTTP 200
- Test `handleChallengeVerify` with invalid nonce → no cookie, HTTP 403
- Test `handleHoldVerify` with sufficient hold → cookie set, HTTP 200
- Test `handleHoldVerify` with insufficient hold → no cookie, HTTP 403
- Test loop bypass sets cookie in response
- Test `Ban()` logs error when `SyncToXDP()` fails
- Test `CleanupExpired()` rebuilds blocklist without expired IPs
- Test `Unban()` removes IP from blocklist and triggers sync
- Test `startXDPComponents` calls `ConfigSync.SyncConfig()`

### Property-Based Tests

- Generate random challenge verification requests (valid/invalid nonces) and verify cookie is set if and only if verification succeeds
- Generate random IP addresses and ban durations, verify blocklist file always matches in-memory store after cleanup
- Generate random sequences of Ban/Unban/CleanupExpired operations and verify blocklist file consistency
- Generate random request sequences and verify loop bypass always grants a cookie

### Integration Tests

- Full flow: client fails PoW → no cookie → retries → succeeds → gets cookie → subsequent requests pass through
- Full flow: loop detection triggers → bypass with cookie → next request passes cookie check → no infinite loop
- Full flow: XDP startup with plan enabled → config synced → GeoIP loaded → botnet controller running
- Full flow: ban → sync → expire → cleanup → blocklist rebuilt → XDP updated
