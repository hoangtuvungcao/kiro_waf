# Bugfix Requirements Document

## Introduction

This document addresses a set of critical bugs that render the WAF's challenge/captcha system and XDP kernel-level blocking non-functional. The challenge verification endpoints (`handleChallengeVerify`, `handleHoldVerify`) grant access cookies unconditionally regardless of verification outcome, allowing bots to bypass protection. The loop detector creates an infinite bypass path without ever requiring challenge completion. On the XDP side, the startup gate requires an environment variable that defaults to empty, no initial config is written to BPF maps (leaving all features disabled at zero values), ban sync errors are silently discarded, and the blocklist file grows unbounded with stale entries that are never cleaned up.

## Bug Analysis

### Current Behavior (Defect)

1.1 WHEN a client submits a PoW challenge verification request to `/__kiro/challenge/verify` THEN the system sets an access cookie unconditionally BEFORE checking whether `challenge.VerifyChallenge()` succeeds or fails, granting valid access even on failed verification

1.2 WHEN a client submits a hold verification request to `/__kiro/hold/verify` THEN the system sets an access cookie unconditionally BEFORE checking whether `challenge.VerifyHold()` succeeds or fails, granting valid access even on failed verification

1.3 WHEN the LoopDetector determines an IP should bypass a challenge (same challenge issued >3 times in 10s) THEN the system proxies the request directly to the backend without setting any access cookie, causing the next request to again lack a valid cookie and re-trigger the challenge in an infinite loop

1.4 WHEN `KIRO_XDP_SYNC_COMMAND` environment variable is not set (empty string) THEN the system sets `xdpCommandConfigured = false` and XDP components never start even when the plan's `xdp_enabled` flag is true

1.5 WHEN `startXDPComponents` is called after plan confirms XDP is enabled THEN the system starts GeoIP loader and botnet controller but never calls `ConfigSync.SyncConfig()` to write initial configuration to the `kiro_config` BPF map, leaving all XDP features disabled (rate_limit_enabled=0, geoip_enabled=0, all thresholds=0)

1.6 WHEN `Ban()` calls `SyncToXDP()` and the sync command fails THEN the system silently discards the error (`_ = e.SyncToXDP()`), resulting in the ban only being enforced at L7 while XDP never blocks the IP at kernel level

1.7 WHEN a ban expires and `CleanupExpired()` removes the IP from the in-memory store THEN the system does not remove the IP from the blocklist file, causing the file to grow unbounded with stale entries

1.8 WHEN `Unban()` is called to remove an IP from the ban store THEN the system removes the IP from memory but does not remove it from the blocklist file, leaving a stale entry that may cause incorrect XDP blocking

### Expected Behavior (Correct)

2.1 WHEN a client submits a PoW challenge verification request to `/__kiro/challenge/verify` THEN the system SHALL call `challenge.VerifyChallenge()` first and only set the access cookie if verification succeeds

2.2 WHEN a client submits a hold verification request to `/__kiro/hold/verify` THEN the system SHALL call `challenge.VerifyHold()` first and only set the access cookie if verification succeeds

2.3 WHEN the LoopDetector determines an IP should bypass a challenge THEN the system SHALL set a short-lived access cookie before proxying to the backend, preventing infinite re-triggering of the challenge on subsequent requests

2.4 WHEN `KIRO_XDP_SYNC_COMMAND` is not set but the plan's `xdp_enabled` flag is true THEN the system SHALL still start XDP components (GeoIP loader, botnet controller, config sync) using bpftool-based map updates directly, independent of the sync command

2.5 WHEN `startXDPComponents` is called THEN the system SHALL call `ConfigSync.SyncConfig()` with the configured XDP parameters to write initial configuration to the `kiro_config` BPF map, enabling rate limiting, GeoIP, and other features as configured

2.6 WHEN `Ban()` calls `SyncToXDP()` and the sync command fails THEN the system SHALL log the error and return it to the caller so that the failure is observable and can be acted upon

2.7 WHEN a ban expires and `CleanupExpired()` removes IPs from the in-memory store THEN the system SHALL rebuild the blocklist file to contain only currently-banned IPs, removing stale entries

2.8 WHEN `Unban()` is called to remove an IP THEN the system SHALL also remove the IP from the blocklist file and trigger an XDP sync to ensure the IP is no longer blocked at kernel level

### Unchanged Behavior (Regression Prevention)

3.1 WHEN a client submits a transparent challenge verification to `/__kiro/transparent/verify` and verification succeeds THEN the system SHALL CONTINUE TO set the access cookie and return HTTP 200 with `{"status":"ok"}`

3.2 WHEN a client has a valid `kiro_access` cookie THEN the system SHALL CONTINUE TO proxy requests directly to the backend without issuing challenges

3.3 WHEN `SyncToXDP()` is called with a valid sync command and the command succeeds THEN the system SHALL CONTINUE TO execute the sync command and return nil

3.4 WHEN an IP is banned and `IsBanned()` is called THEN the system SHALL CONTINUE TO return true and block the request with HTTP 403

3.5 WHEN `KIRO_XDP_SYNC_COMMAND` is set and the plan enables XDP THEN the system SHALL CONTINUE TO start XDP components using the configured sync command

3.6 WHEN the LoopDetector has not detected a loop (challenge issued ≤3 times in 10s) THEN the system SHALL CONTINUE TO serve the challenge page normally without bypassing

3.7 WHEN `appendToBlocklist` is called for a newly banned IP THEN the system SHALL CONTINUE TO write the IP in CIDR /32 format to the blocklist file
