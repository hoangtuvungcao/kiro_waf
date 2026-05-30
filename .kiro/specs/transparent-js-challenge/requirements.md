# Requirements Document

## Introduction

This feature upgrades the Kiro WAF challenge system from a binary "challenge-all-new or rate-limit-only" approach to a multi-tiered, intelligent challenge system. The centerpiece is a **Transparent JS Challenge** — an invisible, auto-solving JavaScript snippet that filters out bots without JS engines while being completely transparent to real users. The system also introduces cookie hardening (TLS fingerprint binding, per-cookie rate limiting, short-lived rotation), a 5-level tiered escalation model, Cloudflare proxy compatibility, and anti-replay/anti-automation defenses.

Additionally, the XDP/eBPF packet filter is upgraded with **SYN cookie validation**, **stateful connection tracking**, **GeoIP blocking**, and **distributed botnet detection** to close gaps where botnets using many unique IPs across different subnets can bypass per-IP/per-subnet rate limits.

## Glossary

- **Client_WAF**: The Kiro reverse proxy component that intercepts HTTP requests, applies challenge logic, and proxies verified traffic to the backend.
- **Transparent_JS_Challenge**: An invisible JavaScript challenge page (<2KB) that auto-solves in <100ms without user interaction, sets an access cookie, and redirects to the original URL.
- **PoW_Challenge**: A visible Proof-of-Work challenge requiring the browser to compute a SHA-256 hash with a specified difficulty prefix.
- **Hold_Challenge**: A visible Hold-to-Confirm captcha requiring the user to press and hold a button for a minimum duration.
- **Challenge_Level**: An integer (0–4) representing the escalation tier assigned to a request based on IP reputation and behavior.
- **Cookie_Manager**: The component responsible for generating, validating, rotating, and revoking HMAC-SHA256 access cookies.
- **TLS_Fingerprint**: A hash derived from the TLS ClientHello parameters (JA3/JA4 style) used to bind cookies to a specific client connection profile.
- **Per_Cookie_Rate_Limiter**: A component that tracks request counts per issued cookie value and revokes cookies exceeding a configured threshold.
- **Escalation_Engine**: The component that determines the Challenge_Level for a given IP based on failed challenge history, rate limit violations, and behavioral signals.
- **Browser_Fingerprint**: A set of client-side signals (canvas hash, WebGL renderer, timezone, installed plugins) collected by the Transparent_JS_Challenge to detect headless browsers.
- **CF_Connecting_IP**: The HTTP header set by Cloudflare containing the real client IP address when traffic passes through Cloudflare proxy.
- **Challenge_Token**: A single-use, time-limited, IP-bound token issued by the Challenge Store for challenge verification.
- **XDP_Filter**: The eBPF program attached to the network interface at the XDP hook point, processing packets before they reach the kernel TCP/IP stack.
- **SYN_Cookie_Validation**: A stateless technique where the XDP filter validates TCP SYN-ACK sequences to ensure the client completed a proper 3-way handshake before allowing further packets.
- **Connection_Tracker**: An XDP-level lightweight connection state table that tracks established TCP connections to distinguish legitimate traffic from spoofed packets.
- **GeoIP_Map**: A BPF LPM trie map containing IP-to-country mappings, enabling per-country blocking or rate limiting at XDP speed.
- **Global_New_IP_Rate**: A per-second counter of unique new source IPs seen by the XDP filter, used to detect distributed botnet attacks where each IP sends few packets but the aggregate is overwhelming.

## Requirements

### Requirement 1: Transparent JS Challenge Page Serving

**User Story:** As a site operator, I want new visitors without a valid cookie to receive an invisible JS challenge instead of a visible PoW page, so that real users experience no friction while bots without JS engines are blocked.

#### Acceptance Criteria

1. WHEN a request arrives without a valid access cookie and the Escalation_Engine assigns Challenge_Level 1, THE Client_WAF SHALL respond with the Transparent_JS_Challenge page.
2. THE Transparent_JS_Challenge page response SHALL have a total size of less than 2048 bytes (2KB).
3. THE Transparent_JS_Challenge page SHALL contain inline JavaScript that auto-solves the challenge without user interaction.
4. WHEN executed in a modern browser (Chrome, Firefox, Safari, Edge latest 2 versions), THE Transparent_JS_Challenge JavaScript SHALL complete solving within 100 milliseconds.
5. THE Transparent_JS_Challenge page SHALL set Cache-Control header to "no-store, no-cache, must-revalidate" to prevent caching by intermediaries.
6. THE Transparent_JS_Challenge page SHALL set X-Content-Type-Options header to "nosniff".

### Requirement 2: Transparent JS Challenge Verification

**User Story:** As a site operator, I want the transparent challenge solution to be verified server-side before granting access, so that forged solutions are rejected.

#### Acceptance Criteria

1. WHEN the Transparent_JS_Challenge JavaScript completes solving, THE Client_WAF SHALL receive a POST request to the verification endpoint containing the challenge token and computed solution.
2. WHEN a valid solution is submitted with a matching token, correct client IP, and within the Challenge_Token TTL, THE Client_WAF SHALL set an access cookie and respond with HTTP 200.
3. IF the submitted token does not exist in the Challenge Store, THEN THE Client_WAF SHALL respond with HTTP 403 and a JSON error message.
4. IF the submitted token has expired (exceeds TTL), THEN THE Client_WAF SHALL respond with HTTP 403 and a JSON error message.
5. IF the client IP does not match the IP bound to the Challenge_Token, THEN THE Client_WAF SHALL respond with HTTP 403 and a JSON error message.
6. WHEN verification succeeds, THE Transparent_JS_Challenge JavaScript SHALL redirect the browser to the originally requested URL.

### Requirement 3: Tiered Challenge Level System

**User Story:** As a site operator, I want requests to be classified into challenge levels (0–4) based on IP reputation and behavior, so that trusted traffic passes freely while suspicious traffic faces progressively harder challenges.

#### Acceptance Criteria

1. THE Escalation_Engine SHALL assign Challenge_Level 0 (no challenge) to requests from IPs listed in the configured admin IP allowlist.
2. WHEN a request arrives from an IP not in the allowlist and without a valid access cookie, THE Escalation_Engine SHALL assign Challenge_Level 1 (Transparent JS Challenge) as the default for new visitors.
3. WHEN an IP has failed the Transparent_JS_Challenge more than a configurable threshold (default: 3 failures within 5 minutes), THE Escalation_Engine SHALL escalate to Challenge_Level 2 (PoW Challenge).
4. WHEN an IP has failed the PoW_Challenge or exceeds the per-IP soft rate limit threshold, THE Escalation_Engine SHALL escalate to Challenge_Level 3 (Hold Challenge).
5. WHEN an IP exceeds the per-IP hard rate limit threshold, THE Escalation_Engine SHALL escalate to Challenge_Level 4 (Block) and ban the IP for the configured ban duration.
6. THE Escalation_Engine SHALL store escalation state in memory without requiring database queries.
7. WHEN the configured escalation cooldown period elapses without further violations from an IP, THE Escalation_Engine SHALL de-escalate the Challenge_Level by one tier.

### Requirement 4: Cookie Hardening with TLS Fingerprint Binding

**User Story:** As a site operator, I want access cookies to be bound to the client TLS fingerprint in addition to IP, so that stolen or replayed cookies from different TLS profiles are rejected.

#### Acceptance Criteria

1. WHEN generating an access cookie, THE Cookie_Manager SHALL include the TLS_Fingerprint hash in the HMAC payload alongside the client IP and expiry timestamp.
2. WHEN validating an access cookie, THE Cookie_Manager SHALL verify that the current request TLS_Fingerprint matches the fingerprint bound in the cookie.
3. IF the TLS_Fingerprint of the current request does not match the fingerprint in the cookie, THEN THE Cookie_Manager SHALL reject the cookie and return a validation error.
4. THE Client_WAF SHALL extract the TLS_Fingerprint from the TLS connection state or from a trusted upstream header (e.g., Cloudflare CF-JA3 header when behind Cloudflare proxy).
5. IF TLS_Fingerprint data is unavailable (e.g., plain HTTP or missing header), THEN THE Cookie_Manager SHALL fall back to IP-only binding without rejecting the request.

### Requirement 5: Per-Cookie Rate Limiting and Revocation

**User Story:** As a site operator, I want each issued cookie to have its own request rate limit, so that an attacker who solves a challenge once cannot use that cookie to send unlimited requests.

#### Acceptance Criteria

1. THE Per_Cookie_Rate_Limiter SHALL track the number of requests per cookie value within a sliding time window.
2. WHEN a cookie exceeds the configured per-cookie request threshold (default: 300 requests per minute), THE Per_Cookie_Rate_Limiter SHALL revoke the cookie by adding it to a revocation set.
3. WHEN a revoked cookie is presented in a subsequent request, THE Client_WAF SHALL reject the cookie and serve the appropriate challenge based on the current Challenge_Level for that IP.
4. THE Per_Cookie_Rate_Limiter SHALL operate in O(1) time complexity for lookup and increment operations.
5. THE Per_Cookie_Rate_Limiter SHALL periodically clean up expired entries to prevent unbounded memory growth.

### Requirement 6: Cookie Rotation with Short-Lived Tokens

**User Story:** As a site operator, I want access cookies to be short-lived with an automatic refresh mechanism, so that the window for cookie replay attacks is minimized.

#### Acceptance Criteria

1. THE Cookie_Manager SHALL generate access cookies with a configurable short TTL (default: 5 minutes).
2. WHEN a valid access cookie has less than 50% of its TTL remaining, THE Client_WAF SHALL issue a refreshed cookie with a new expiry in the response.
3. WHEN a cookie expires without being refreshed, THE Client_WAF SHALL serve the Transparent_JS_Challenge (Challenge_Level 1) to re-verify the client.
4. THE refreshed cookie SHALL maintain the same IP and TLS_Fingerprint binding as the original cookie.
5. THE Cookie_Manager SHALL use a different HMAC nonce for each refreshed cookie to prevent replay of old cookie values.

### Requirement 7: Cloudflare Proxy Compatibility

**User Story:** As a site operator running behind Cloudflare proxy, I want the challenge system to correctly identify client IPs and avoid redirect loops, so that the WAF functions correctly in a Cloudflare-proxied environment.

#### Acceptance Criteria

1. WHEN the CF-Connecting-IP header is present, THE Client_WAF SHALL use its value as the client IP address instead of X-Forwarded-For or RemoteAddr.
2. THE Client_WAF SHALL validate that CF-Connecting-IP requests originate from known Cloudflare IP ranges before trusting the header.
3. THE Transparent_JS_Challenge page SHALL use a relative URL for the verification endpoint to avoid redirect loops through Cloudflare.
4. THE Client_WAF SHALL set "Cache-Control: no-store, private" on all challenge page responses to prevent Cloudflare from caching challenge pages.
5. IF the Client_WAF detects a potential redirect loop (same IP receiving the same challenge more than 3 times within 10 seconds), THEN THE Client_WAF SHALL bypass the challenge and proxy the request to the backend.

### Requirement 8: Anti-Replay and Timestamp Validation

**User Story:** As a site operator, I want challenge tokens to be strictly single-use and time-bound, so that captured tokens cannot be replayed by attackers.

#### Acceptance Criteria

1. THE Challenge Store SHALL enforce single-use semantics by deleting the token immediately upon first verification attempt (regardless of success or failure).
2. WHEN a challenge token is submitted after its TTL has elapsed, THE Client_WAF SHALL reject the solution with HTTP 403.
3. THE Challenge_Token TTL SHALL be configurable with a default of 30 seconds for Transparent_JS_Challenge and 90 seconds for PoW_Challenge and Hold_Challenge.
4. WHEN a challenge token is issued, THE Challenge Store SHALL record the issuance timestamp with UTC precision.
5. THE Client_WAF SHALL reject challenge solutions where the time between issuance and submission is less than 50 milliseconds (indicating automated submission faster than realistic browser execution).

### Requirement 9: Headless Browser Detection in JS Challenge

**User Story:** As a site operator, I want the transparent JS challenge to detect headless browsers and automation frameworks, so that sophisticated bots using JS engines are identified and escalated.

#### Acceptance Criteria

1. THE Transparent_JS_Challenge JavaScript SHALL check for the presence of navigator.webdriver property and report it in the solution payload.
2. THE Transparent_JS_Challenge JavaScript SHALL collect a Browser_Fingerprint including: canvas rendering hash, WebGL renderer string, and timezone offset.
3. WHEN the solution payload indicates navigator.webdriver is true, THE Client_WAF SHALL reject the solution and escalate the IP to Challenge_Level 2.
4. WHEN the Browser_Fingerprint is missing expected fields (canvas hash, WebGL renderer), THE Client_WAF SHALL flag the request as suspicious and escalate the IP to Challenge_Level 2.
5. THE Browser_Fingerprint collection SHALL complete within the 100ms auto-solve time budget of the Transparent_JS_Challenge.

### Requirement 10: Performance Requirements

**User Story:** As a site operator, I want the challenge system to add minimal latency and resource usage, so that legitimate traffic is not degraded.

#### Acceptance Criteria

1. THE Client_WAF SHALL validate access cookies in O(1) time using HMAC verification without database queries.
2. THE Client_WAF SHALL serve challenge pages from in-memory templates without filesystem reads or database queries.
3. THE Escalation_Engine level lookup for a given IP SHALL complete in O(1) time using an in-memory map.
4. THE Transparent_JS_Challenge page total response size SHALL remain under 2048 bytes.
5. WHEN the Transparent_JS_Challenge JavaScript executes in a modern browser, THE challenge SHALL auto-solve and submit within 100 milliseconds.
6. THE Per_Cookie_Rate_Limiter lookup and increment operations SHALL complete in O(1) amortized time.

### Requirement 11: Configuration and Operational Control

**User Story:** As a site operator, I want all challenge thresholds and behaviors to be configurable via environment variables, so that I can tune the system without code changes.

#### Acceptance Criteria

1. THE Client_WAF SHALL read the following configuration from environment variables: transparent challenge TTL, PoW challenge TTL, Hold challenge TTL, per-cookie rate limit threshold, escalation failure threshold, escalation cooldown duration, cookie short TTL, and Cloudflare IP trust mode.
2. THE Client_WAF SHALL provide sensible defaults for all configurable values so that the system operates correctly without explicit configuration.
3. WHEN the KIRO_CHALLENGE_ALL_NEW environment variable is set to "true", THE Client_WAF SHALL serve Transparent_JS_Challenge (instead of PoW_Challenge) to all new visitors without valid cookies.
4. THE Client_WAF SHALL log challenge events (issue, verify success, verify failure, escalation, cookie revocation) at INFO level with the client IP and challenge type.

### Requirement 12: XDP SYN Cookie Validation

**User Story:** As a site operator, I want the XDP filter to validate TCP handshakes using SYN cookies, so that SYN flood attacks with spoofed source IPs are dropped at line rate without consuming kernel TCP stack resources.

#### Acceptance Criteria

1. WHEN the XDP_Filter receives a TCP SYN packet and the global SYN rate exceeds a configurable threshold (default: 10,000 SYN/s), THE XDP_Filter SHALL respond with a SYN-ACK containing a cryptographic SYN cookie (encoded in the sequence number) and drop the original SYN.
2. WHEN the XDP_Filter receives a TCP ACK packet, THE XDP_Filter SHALL validate the acknowledgment number against the expected SYN cookie value for the source IP.
3. IF the ACK validation succeeds, THE XDP_Filter SHALL add the source IP to the Connection_Tracker allowlist and pass the packet.
4. IF the ACK validation fails, THE XDP_Filter SHALL drop the packet and increment the DROP_INVALID_ACK statistics counter.
5. THE SYN cookie computation SHALL use a keyed hash (SipHash or truncated SHA) of (source IP, source port, dest port, timestamp) to prevent forgery.
6. THE SYN cookie validation SHALL complete within the <100ns per-packet budget.

### Requirement 13: XDP Lightweight Connection Tracking

**User Story:** As a site operator, I want the XDP filter to track established TCP connections, so that packets from connections that never completed a handshake are dropped immediately.

#### Acceptance Criteria

1. THE Connection_Tracker SHALL maintain a BPF LRU hash map of established TCP connections keyed by (source IP, source port, dest port) tuple.
2. WHEN a TCP connection is validated (SYN cookie ACK passes or first data packet after SYN-ACK), THE XDP_Filter SHALL insert the connection into the Connection_Tracker.
3. WHEN a TCP data packet (non-SYN, non-RST) arrives for a connection NOT in the Connection_Tracker, THE XDP_Filter SHALL drop the packet.
4. THE Connection_Tracker map SHALL have a maximum capacity of 524,288 entries with LRU eviction.
5. WHEN a TCP RST or FIN packet is received for a tracked connection, THE XDP_Filter SHALL remove the connection from the tracker.
6. THE Connection_Tracker lookup SHALL complete in O(1) time using BPF hash map operations.

### Requirement 14: XDP GeoIP Blocking

**User Story:** As a site operator, I want to block or rate-limit traffic from specific countries at XDP speed, so that attacks originating from countries with no legitimate users are dropped before reaching the application layer.

#### Acceptance Criteria

1. THE XDP_Filter SHALL maintain a GeoIP_Map (BPF LPM trie) mapping IPv4 prefixes to 2-letter country codes.
2. THE XDP_Filter SHALL maintain a country blocklist (BPF hash map) of country codes to block.
3. WHEN a packet's source IP resolves to a country in the blocklist, THE XDP_Filter SHALL drop the packet and increment the DROP_GEOIP statistics counter.
4. THE GeoIP_Map SHALL support at least 500,000 prefix entries to cover the full IPv4 GeoIP database.
5. THE GeoIP_Map SHALL be populated from userspace by the Go client binary at startup and refreshed periodically (default: every 24 hours).
6. THE country blocklist SHALL be configurable via environment variable KIRO_XDP_BLOCKED_COUNTRIES (comma-separated 2-letter codes, e.g., "CN,RU,KP").

### Requirement 15: XDP Distributed Botnet Detection

**User Story:** As a site operator, I want the XDP filter to detect distributed botnet attacks where thousands of unique IPs each send few packets, so that the aggregate flood is mitigated even when individual IPs are below per-IP thresholds.

#### Acceptance Criteria

1. THE XDP_Filter SHALL maintain a Global_New_IP_Rate counter tracking the number of unique new source IPs seen per second.
2. WHEN the Global_New_IP_Rate exceeds a configurable threshold (default: 5,000 new IPs/second), THE XDP_Filter SHALL enter "botnet mode" and apply stricter per-IP rate limits (reduce thresholds by 75%).
3. WHILE in botnet mode, THE XDP_Filter SHALL drop all packets from IPs not in the Connection_Tracker (only allow established connections).
4. WHEN the Global_New_IP_Rate drops below 50% of the threshold for a configurable cooldown period (default: 30 seconds), THE XDP_Filter SHALL exit botnet mode and restore normal thresholds.
5. THE Global_New_IP_Rate SHALL be computed using a per-CPU approximate counter with a BPF LRU hash map for IP deduplication within the counting window.
6. THE botnet mode state SHALL be stored in the kiro_config BPF array map so that userspace can monitor and override it.
