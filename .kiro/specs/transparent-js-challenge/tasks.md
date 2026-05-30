# Implementation Plan: Transparent JS Challenge + XDP Hardening

## Overview

This implementation plan covers a multi-layered defense system combining L7 intelligent challenge escalation (Go) with XDP/eBPF packet-level filtering (C). The L7 layer introduces a transparent JavaScript challenge, 5-tier escalation engine, cookie hardening with TLS fingerprint binding, per-cookie rate limiting, and Cloudflare proxy compatibility. The XDP layer adds SYN cookie validation, connection tracking, GeoIP blocking, and distributed botnet detection.

Tasks are ordered to build foundational components first (interfaces, data models), then core logic, then integration/wiring. XDP and L7 layers are developed in parallel where possible.

## Tasks

- [x] 1. Core interfaces, data models, and shared utilities
  - [x] 1.1 Create Cookie Manager V2 with TLS fingerprint binding (`internal/client/cookie/manager_v2.go`)
    - Define `CookieManagerV2` struct with `secret []byte` and `defaultTTL time.Duration`
    - Implement cookie payload format: Version(1B) | IP_Hash(4B, FNV-1a) | TLS_FP_Hash(4B, FNV-1a) | Expiry(8B) | Nonce(8B) | HMAC(32B)
    - Implement `GenerateCookie(ip, tlsFingerprint string, ttl time.Duration) (string, error)` with base64url encoding
    - Implement `ValidateCookie(cookie, ip, tlsFingerprint string) (bool, time.Duration, error)` returning remaining TTL
    - Implement `ShouldRefresh(remainingTTL, totalTTL time.Duration) bool` (true when <50% remaining)
    - Handle fallback: when `tlsFingerprint` is empty, use zero hash for TLS field
    - _Requirements: 4.1, 4.2, 4.3, 4.5, 6.1, 6.4, 6.5_

  - [ ]* 1.2 Write property tests for Cookie Manager V2
    - **Property 9: TLS fingerprint binding rejects mismatched cookies**
    - **Property 10: Missing TLS fingerprint falls back to IP-only**
    - **Property 13: Cookie refresh preserves bindings**
    - **Property 14: Cookie refresh at 50% TTL**
    - **Validates: Requirements 4.1, 4.2, 4.3, 4.5, 6.2, 6.4, 6.5**

  - [x] 1.3 Create Escalation Engine (`internal/client/escalation/engine.go`)
    - Define `EscalationEngine` struct with `sync.RWMutex`, `states map[string]*IPState`, `adminAllowlist map[string]bool`, `config EscalationConfig`
    - Define `EscalationConfig` with `FailureThreshold int`, `FailureWindow time.Duration`, `CooldownDuration time.Duration`
    - Define `IPState` with `Level int`, `FailureCount int`, `LastFailure time.Time`, `LastEscalation time.Time`, `LastActivityAt time.Time`
    - Implement `NewEscalationEngine(config EscalationConfig, adminIPs []string) *EscalationEngine`
    - Implement `GetLevel(ip string) int` — returns 0 for admin IPs, checks cooldown-based de-escalation, returns current level
    - Implement `RecordFailure(ip string, challengeType string)` — increments failure count, escalates when threshold exceeded
    - Implement `RecordSuccess(ip string)` — resets failure count (does NOT de-escalate immediately)
    - Implement `Cleanup()` — removes stale entries older than 2× cooldown
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 3.7_

  - [ ]* 1.4 Write property tests for Escalation Engine
    - **Property 6: Admin IPs bypass all challenges**
    - **Property 7: Escalation on repeated failures**
    - **Property 8: De-escalation after cooldown**
    - **Validates: Requirements 3.1, 3.3, 3.4, 3.7**

  - [x] 1.5 Create TLS Fingerprint Extractor (`internal/client/fingerprint/tls.go`)
    - Define `TLSExtractor` struct
    - Implement `ExtractFingerprint(r *http.Request) string` — checks `X-TLS-Fingerprint` header, then `CF-JA3` header, returns empty string if unavailable
    - Keep implementation simple: hash of available TLS info from headers
    - _Requirements: 4.4, 4.5_

- [x] 2. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 3. L7 challenge serving and verification
  - [x] 3.1 Create Transparent JS Challenge page and verification (`internal/client/challenge/transparent.go`)
    - Define `TransparentChallenge` struct with `Store *Store`, `TTL time.Duration`, `MinSolveMs int64`
    - Implement `ServeTransparentPage(w, r, store, ttl, clientIP)` — serves <2KB inline HTML/JS page
    - The JS page must: compute HMAC-based proof, collect browser fingerprint (canvas, WebGL, timezone, webdriver), POST to `/__kiro/transparent/verify`
    - Set headers: `Cache-Control: no-store, no-cache, must-revalidate`, `X-Content-Type-Options: nosniff`
    - Implement `VerifyTransparent(w, r, store, clientIP, escalation) bool`
    - Verify: token exists, not expired, IP matches, solve time >= 50ms, parse fingerprint JSON
    - On webdriver=true or missing fingerprint fields: reject + escalate to level 2
    - Delete token from store immediately on first attempt (single-use)
    - On success: return true (caller sets cookie); on failure: return false + HTTP 403
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 8.1, 8.4, 8.5, 9.1, 9.2, 9.3, 9.4, 9.5_

  - [ ]* 3.2 Write property tests for Transparent Challenge
    - **Property 1: Escalation level routing (level 1 → transparent)**
    - **Property 2: Valid challenge solution grants access**
    - **Property 3: Invalid token submission is rejected**
    - **Property 4: Challenge tokens are single-use**
    - **Property 5: Submissions faster than minimum solve time are rejected**
    - **Property 17: Webdriver detection escalates**
    - **Validates: Requirements 1.1, 2.2, 2.3, 2.4, 2.5, 8.1, 8.5, 9.3, 9.4**

  - [x] 3.3 Create Anti-Replay Validator (`internal/client/challenge/antireplay.go`)
    - Implement enhanced token store logic ensuring single-use semantics
    - `IssueToken(clientIP string, ttl time.Duration) (token string, issuedAt time.Time)`
    - `ConsumeToken(token string, clientIP string) (issuedAt time.Time, err error)` — deletes on first call regardless of outcome
    - Validate: token exists, IP matches, not expired
    - Return `issuedAt` so caller can check minimum solve time
    - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5_

  - [x] 3.4 Create Loop Detector (`internal/client/challenge/loopdetect.go`)
    - Define `LoopDetector` struct with `mu sync.Mutex`, `records map[string]*loopRecord`
    - Implement `ShouldBypass(ip string, challengeType string) bool` — returns true if same IP received same challenge >3 times in 10 seconds
    - Implement `Record(ip string, challengeType string)` — records challenge issuance
    - Implement `Cleanup()` — removes entries older than 30 seconds
    - _Requirements: 7.5_

  - [ ]* 3.5 Write property test for Loop Detector
    - **Property 16: Redirect loop bypass**
    - **Validates: Requirements 7.5**

- [x] 4. L7 rate limiting and Cloudflare support
  - [x] 4.1 Create Per-Cookie Rate Limiter (`internal/client/ratelimit/cookie_limiter.go`)
    - Define `CookieRateLimiter` struct with `mu sync.Mutex`, `counters map[uint64]*cookieCounter`, `revoked map[uint64]time.Time`, `threshold int`, `window time.Duration`
    - Use FNV-1a hash of cookie value as map key for O(1) lookup
    - Implement `RecordAndCheck(cookieValue string) bool` — increments counter, returns false if revoked or threshold exceeded
    - Implement `IsRevoked(cookieValue string) bool`
    - Implement `Cleanup()` — removes expired counters and old revocation entries
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5_

  - [ ]* 4.2 Write property tests for Per-Cookie Rate Limiter
    - **Property 11: Per-cookie rate limit revocation**
    - **Property 12: Cookie expiry triggers re-challenge**
    - **Validates: Requirements 5.2, 5.3, 6.1, 6.3**

  - [x] 4.3 Create Cloudflare IP Extractor (`internal/client/cf/extractor.go`)
    - Define `CFExtractor` struct with `trustedRanges []*net.IPNet`, `trustMode string`
    - Implement `NewCFExtractor(mode string) *CFExtractor` — loads Cloudflare published IP ranges (hardcoded known ranges)
    - Implement `ExtractClientIP(r *http.Request) string` — if peer IP is in Cloudflare range, use CF-Connecting-IP header
    - Implement `IsCloudflarePeer(remoteAddr string) bool`
    - _Requirements: 7.1, 7.2, 7.3, 7.4_

  - [ ]* 4.4 Write property test for Cloudflare IP Extractor
    - **Property 15: Cloudflare IP extraction**
    - **Validates: Requirements 7.1, 7.2**

- [x] 5. Checkpoint - Ensure all L7 component tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 6. XDP SYN cookie and connection tracking
  - [x] 6.1 Implement SYN cookie logic in XDP (`internal/client/xdp/xdp_filter.c`)
    - Add `syn_cookie_key` struct (k0, k1 for SipHash) to config or separate array map
    - Implement inline SipHash-2-4 function (~20 cycles for 12-byte input, truncated to 32 bits)
    - Add `syn_rate` per-CPU array map for global SYN rate tracking
    - Add new stat counters: `KIRO_STAT_DROP_INVALID_ACK`, `KIRO_STAT_SYNCOOKIE_ISSUED`, `KIRO_STAT_SYNCOOKIE_VALID`
    - On SYN packet when rate > threshold: compute cookie, craft SYN-ACK, return `XDP_TX`
    - On ACK packet: recompute cookie from (src_ip, src_port, dst_port, timestamp_bucket), validate `ack_num - 1`
    - Check both current and previous timestamp bucket (1-second buckets) for boundary crossing
    - Extend `kiro_xdp_config` with `syn_cookie_threshold`, `syn_cookie_active` fields
    - _Requirements: 12.1, 12.2, 12.3, 12.4, 12.5, 12.6_

  - [ ]* 6.2 Write property tests for SYN cookie computation (Go mirror)
    - Create `internal/client/xdp/syncookie_test.go` with Go implementation of SipHash matching C
    - **Property 18: SYN cookie round-trip**
    - **Property 19: Invalid ACK is dropped**
    - **Validates: Requirements 12.1, 12.2, 12.4, 12.5**

  - [x] 6.3 Implement Connection Tracker in XDP (`internal/client/xdp/xdp_filter.c`)
    - Add `conn_tracker` LRU hash map (524,288 entries) with `conn_key` and `conn_value` structs
    - On successful SYN cookie ACK validation: insert into `conn_tracker`
    - On TCP data packets (non-SYN, non-RST, non-FIN): lookup `conn_tracker` → DROP if absent
    - On RST/FIN packets: delete from `conn_tracker`
    - Add `conn_tracker_enabled` field to `kiro_xdp_config`
    - Integrate into main XDP flow after allowlist/blocklist, before rate limiting
    - _Requirements: 13.1, 13.2, 13.3, 13.4, 13.5, 13.6_

  - [ ]* 6.4 Write property tests for Connection Tracker
    - **Property 20: Valid handshake inserts into connection tracker**
    - **Property 21: Untracked data packets are dropped**
    - **Property 22: RST/FIN removes from connection tracker**
    - **Validates: Requirements 12.3, 13.2, 13.3, 13.5**

- [x] 7. XDP GeoIP and botnet detection
  - [x] 7.1 Implement GeoIP blocking in XDP (`internal/client/xdp/xdp_filter.c`)
    - Add `geoip_map` LPM trie (524,288 entries) with `lpm_v4_key` → `geoip_value` (country code as `__u16`)
    - Add `country_blocklist` hash map (256 entries) with `__u16` → `__u8`
    - Add `geoip_enabled` field to `kiro_xdp_config`
    - Add `KIRO_STAT_DROP_GEOIP` counter
    - Insert GeoIP check after blocklist, before private source check
    - Lookup order: LPM lookup → get country → hash lookup in blocklist → DROP if found
    - _Requirements: 14.1, 14.2, 14.3, 14.4, 14.6_

  - [x] 7.2 Create GeoIP Loader from userspace (`internal/client/xdp/geoip_loader.go`)
    - Implement `GeoIPLoader` struct with BPF map file descriptors
    - Implement `LoadFromCSV(path string) error` — parse MaxMind GeoLite2 CSV, populate `geoip_map` via `bpf_map_update_elem`
    - Implement `LoadBlockedCountries(countries []string) error` — populate `country_blocklist`
    - Implement `StartPeriodicRefresh(ctx context.Context, interval time.Duration)` — refresh every 24h
    - Read `KIRO_XDP_BLOCKED_COUNTRIES` env var for country codes
    - _Requirements: 14.5, 14.6_

  - [ ]* 7.3 Write property test for GeoIP blocking
    - **Property 23: GeoIP blocked country drops packets**
    - **Validates: Requirements 14.3**

  - [x] 7.4 Implement Distributed Botnet Detector in XDP (`internal/client/xdp/xdp_filter.c`)
    - Add `ip_dedup` LRU hash map (262,144 entries) for new-IP deduplication
    - Add `new_ip_rate` per-CPU array map (1 entry) with `new_ip_counter` struct
    - Add `botnet_new_ip_threshold`, `botnet_cooldown_seconds`, `botnet_mode_active` to `kiro_xdp_config`
    - For each packet: check if `src_ip` in `ip_dedup`; if not, increment `new_ip_rate` counter, insert into `ip_dedup`
    - Per-CPU threshold = `botnet_new_ip_threshold / num_cpus` (approximate activation)
    - In botnet mode: drop packets from IPs NOT in `conn_tracker`
    - Add `KIRO_STAT_DROP_BOTNET` counter
    - _Requirements: 15.1, 15.2, 15.3, 15.5, 15.6_

  - [x] 7.5 Create botnet mode controller from userspace (`internal/client/xdp/botnet_controller.go`)
    - Implement `BotnetController` struct monitoring `new_ip_rate` map
    - Sum per-CPU counters to get true rate
    - When rate < 50% threshold for cooldown period: set `botnet_mode_active = 0` in `kiro_config`
    - Provide manual override: `ForceDisable()` writes `botnet_mode_active = 0`
    - _Requirements: 15.4, 15.6_

  - [ ]* 7.6 Write property tests for Botnet Detection
    - **Property 24: Botnet mode activation and enforcement**
    - **Property 25: Botnet mode cooldown exit**
    - **Property 26: New-IP counter accuracy**
    - **Validates: Requirements 15.2, 15.3, 15.4, 15.1**

- [x] 8. Checkpoint - Ensure all XDP tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 9. Integration and wiring
  - [x] 9.1 Update `proxy.go` to integrate new components
    - Replace `hasValidCookie` to use `CookieManagerV2` with TLS fingerprint from `TLSExtractor`
    - Add cookie refresh logic: if valid cookie with <50% TTL remaining, set refreshed cookie in response
    - Integrate `CookieRateLimiter`: after cookie validation, check per-cookie rate → revoke if exceeded
    - Integrate `LoopDetector`: before serving any challenge, check `ShouldBypass` → proxy if true
    - Replace challenge routing logic with `EscalationEngine.GetLevel(ip)`:
      - Level 0: proxy directly
      - Level 1: serve transparent challenge
      - Level 2: serve PoW challenge
      - Level 3: serve hold challenge
      - Level 4: ban + 403
    - Add `/__kiro/transparent/verify` endpoint handling
    - On challenge failure: call `EscalationEngine.RecordFailure(ip, type)`
    - On challenge success: call `EscalationEngine.RecordSuccess(ip)`, set cookie v2
    - Integrate `CFExtractor` for client IP resolution (priority: CF-Connecting-IP → X-Forwarded-For → RemoteAddr)
    - _Requirements: 1.1, 2.2, 3.2, 3.3, 3.4, 3.5, 4.1, 5.3, 6.2, 7.1, 7.5_

  - [x] 9.2 Update `client_waf.go` to initialize new components
    - Add `EscalationEngine` initialization with config from env vars
    - Add `CookieManagerV2` initialization with secret and default TTL (5 min)
    - Add `CookieRateLimiter` initialization with threshold from env (default: 300/min)
    - Add `CFExtractor` initialization with trust mode from env
    - Add `TLSExtractor` initialization
    - Add `LoopDetector` initialization
    - Add periodic cleanup goroutines for: escalation engine (5 min), cookie rate limiter (2 min), loop detector (30s)
    - Add new env vars: `KIRO_TRANSPARENT_TTL`, `KIRO_COOKIE_SHORT_TTL`, `KIRO_ESCALATION_THRESHOLD`, `KIRO_ESCALATION_COOLDOWN`, `KIRO_COOKIE_RATE_LIMIT`, `KIRO_CF_TRUST_MODE`, `KIRO_XDP_BLOCKED_COUNTRIES`
    - Wire GeoIP loader startup (if XDP enabled)
    - Wire botnet controller startup (if XDP enabled)
    - _Requirements: 11.1, 11.2, 11.3, 11.4_

  - [x] 9.3 Update XDP config struct and userspace sync (`internal/client/xdp/`)
    - Update Go-side `kiro_xdp_config` struct to match extended C struct
    - Add fields: `syn_cookie_threshold`, `botnet_new_ip_threshold`, `botnet_cooldown_seconds`, `botnet_mode_active`, `syn_cookie_active`, `geoip_enabled`, `conn_tracker_enabled`
    - Update config sync function to write new fields to BPF map
    - Add SYN cookie key rotation (every 24h) via `syn_cookie_key` map update
    - _Requirements: 12.1, 14.6, 15.6_

  - [ ]* 9.4 Write integration tests for full request flow
    - Test: new IP → transparent challenge → verify → cookie → proxy
    - Test: failed transparent → escalate to PoW → verify → cookie
    - Test: cookie rate limit exceeded → revoke → re-challenge
    - Test: cookie with wrong TLS fingerprint → reject → challenge
    - Test: Cloudflare peer → use CF-Connecting-IP
    - Test: loop detection → bypass after 3 same challenges in 10s
    - _Requirements: 1.1, 2.2, 3.3, 4.3, 5.3, 7.1, 7.5_

- [x] 10. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties from the design document (26 total)
- Unit tests validate specific examples and edge cases
- XDP C code changes (tasks 6.1, 6.3, 7.1, 7.4) all modify `xdp_filter.c` and must be sequential
- L7 Go components (tasks 1.x, 3.x, 4.x) are independent and can be developed in parallel
- The `kiro_xdp_config` struct extension (task 9.3) must be compatible with existing fields
- All XDP maps use LRU for auto-eviction to maintain <100ns per-packet budget
- GeoIP data loaded from userspace (MaxMind GeoLite2) — not embedded in BPF program
- Botnet mode cooldown is controlled from userspace goroutine reading per-CPU counters

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "1.3", "1.5"] },
    { "id": 1, "tasks": ["1.2", "1.4", "3.1", "3.3", "3.4", "4.1", "4.3"] },
    { "id": 2, "tasks": ["3.2", "3.5", "4.2", "4.4", "6.1"] },
    { "id": 3, "tasks": ["6.2", "6.3"] },
    { "id": 4, "tasks": ["6.4", "7.1"] },
    { "id": 5, "tasks": ["7.2", "7.3", "7.4"] },
    { "id": 6, "tasks": ["7.5", "7.6"] },
    { "id": 7, "tasks": ["9.1", "9.2", "9.3"] },
    { "id": 8, "tasks": ["9.4"] }
  ]
}
```
