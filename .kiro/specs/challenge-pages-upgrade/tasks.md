# Implementation Plan: Challenge Pages Upgrade

## Overview

Upgrade the Kiro WAF challenge pages (Hold page and PoW page) with inline SVG shield logo, modern dark theme, smooth animations, and optimized response size under 10KB. All changes are to the HTML template constants in Go files, maintaining the existing `strings.NewReplacer` architecture. Cookie ordering and binary fingerprint behavior are already correct and must not be reverted.

## Tasks

- [x] 1. Replace text logo with inline SVG shield on both pages
  - [x] 1.1 Update Hold page logo in `internal/client/challenge/hold.go`
    - Replace the `<div class="logo">K</div>` with the decoded inline SVG shield element
    - SVG must have `width="48"` `height="48"` `role="img"` `aria-label="Kiro WAF Shield Logo"`
    - Use the SVG markup from the existing base64 favicon (decoded), with gradient colors matching CSS custom properties
    - Update `.logo` CSS class to remove text-styling (font-weight, font-size, color) and keep only container sizing/margin
    - _Requirements: 1.1, 1.3, 1.4, 1.5_

  - [x] 1.2 Update PoW page logo in `internal/client/challenge/pow.go`
    - Replace the `<div class="logo">K</div>` with the same decoded inline SVG shield element
    - SVG must have `width="48"` `height="48"` `role="img"` `aria-label="Kiro WAF Shield Logo"`
    - Update `.logo` CSS class to remove text-styling and keep only container sizing/margin
    - _Requirements: 1.2, 1.3, 1.4, 1.5_

- [x] 2. Verify dark theme and CSS custom properties
  - [x] 2.1 Audit and confirm CSS custom properties on Hold page
    - Verify `:root` contains all required tokens: `--kiro-primary`, `--kiro-accent`, `--kiro-background`, `--kiro-surface`, `--kiro-text-primary`, `--kiro-text-secondary`, `--kiro-border`, `--kiro-success`, `--kiro-danger`
    - Verify `color-scheme: dark` is present
    - Verify `backdrop-filter: blur(12px)` on `.card`
    - Verify system font stack is used
    - Verify radial-gradient background on body
    - Verify no external stylesheet references, no `@import`, no external `url()` values
    - _Requirements: 2.1, 2.3, 2.4, 2.5, 2.6_

  - [x] 2.2 Audit and confirm CSS custom properties on PoW page
    - Same verification as 2.1 for the PoW page template
    - Verify visual consistency with Hold page (same gradient, same custom properties)
    - _Requirements: 2.2, 2.3, 2.4, 2.5, 2.6_

  - [x] 2.3 Verify responsive breakpoints on both pages
    - Confirm `@media (max-width: 360px)` reduces card padding to 24px/20px, heading to 1.25rem
    - Confirm `@media (min-width: 1920px)` increases card padding to 40px/36px, heading to 1.75rem
    - Confirm Hold page also adjusts button padding at ≤360px to 16px/24px
    - _Requirements: 2.7, 9.1, 9.2, 9.3, 9.4_

- [x] 3. Verify Hold page animations and interactions
  - [x] 3.1 Confirm Hold page progress bar and timer logic
    - Verify fill bar animates from 0% to 100% proportional to elapsed/holdSeconds, updating every 50ms
    - Verify timer displays "X.X / Y giây" format
    - Verify button text changes to "Thả để xác nhận" when time is met
    - Verify status text changes to "Đủ thời gian - thả nút" when time is met
    - Verify on early release: fill resets to 0%, timer shows "Chưa đủ thời gian. Vui lòng giữ lâu hơn.", button resets
    - Verify on successful release: fill 100%, button "Đang xác nhận...", POST to verify endpoint
    - _Requirements: 3.1, 3.4, 3.6, 4.1, 4.2, 4.3, 4.4, 4.6, 4.7_

  - [x] 3.2 Confirm Hold page event handlers
    - Verify mousedown, mouseup, mouseleave event listeners on button
    - Verify touchstart, touchend, touchcancel event listeners on button
    - Verify `scale(0.98)` transform and border-color change to `--blue` during hold with 100ms transition
    - _Requirements: 4.5, 3.6_

- [x] 4. Verify PoW page animations
  - [x] 4.1 Confirm PoW progress bar behavior
    - Verify progress formula: `5 + (nonces_tried / (16^difficulty * 0.6)) * 90` capped at 95%
    - Verify CSS shimmer animation cycling every 1.5s on progress fill
    - Verify CSS transition of 300ms with `cubic-bezier(0.4, 0, 0.2, 1)` on progress fill width
    - Verify on success: fill 100%, success message displayed, redirect after 600ms
    - Verify on failure: fill 0%, error message displayed
    - _Requirements: 3.2, 3.3, 3.4, 3.5_

- [x] 5. Verify noscript fallback on both pages
  - [x] 5.1 Confirm noscript blocks on both pages
    - Verify `<noscript>` block exists on both Hold and PoW pages
    - Verify heading "JavaScript bị tắt" and instruction text in Vietnamese
    - Verify styled with danger-colored border, semi-transparent background, centered text, rounded corners
    - Verify page layout (logo, card, footer) still renders when JS is disabled
    - _Requirements: 8.1, 8.2, 8.3, 8.4_

- [x] 6. Checkpoint - Verify all structural requirements
  - Ensure all tests pass, ask the user if questions arise.

- [x] 7. Implement response size test and no-external-resources test
  - [x] 7.1 Add response size regression test in `internal/client/challenge/pow_test.go`
    - Create `TestHoldPageResponseSizeUnder10KB` that renders Hold page with max-length template variables (token: 44 chars, HOLD_SECONDS: 3 chars, NEXT: 2048 chars) and asserts byte length < 10240
    - Create `TestPoWPageResponseSizeUnder10KB` that renders PoW page with max-length template variables (token: 44 chars, salt: 44 chars, DIFFICULTY: 2 chars, NEXT: 2048 chars) and asserts byte length < 10240
    - _Requirements: 6.1, 6.2, 6.5_

  - [x] 7.2 Add no-external-resources test for Hold page
    - Create `TestHoldPage_NoExternalDependencies` verifying no `<link rel="stylesheet">`, no `<script src=`, no external `url()`, no `@import`, no external `<img>` etc.
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5, 5.6, 5.7, 5.8_

  - [x] 7.3 Add SVG logo presence test for both pages
    - Create `TestHoldPage_InlineSVGLogo` verifying rendered output contains `<svg` with `role="img"` and `aria-label`
    - Create `TestPoWPage_InlineSVGLogo` verifying rendered output contains `<svg` with `role="img"` and `aria-label`
    - _Requirements: 1.1, 1.2, 1.5_

  - [x] 7.4 Add CSS custom properties and responsive breakpoint tests
    - Create `TestHoldPage_CSSCustomProperties` verifying `:root` contains all required custom properties
    - Create `TestPoWPage_CSSCustomProperties` verifying same
    - Create `TestHoldPage_ResponsiveBreakpoints` verifying media queries at 360px and 1920px
    - Create `TestPoWPage_ResponsiveBreakpoints` verifying media queries at 360px and 1920px
    - _Requirements: 2.3, 2.7, 9.3, 9.4_

- [ ] 8. Implement property-based test for cookie header presence
  - [ ]* 8.1 Write property test for cookie always set before verification response
    - **Property 1: Cookie header presence on all verification responses**
    - **Validates: Requirements 10.1, 10.3**
    - Use `pgregory.net/rapid` library
    - Generate random scenarios: valid/invalid tokens, correct/incorrect IPs, valid/invalid nonces, sufficient/insufficient hold durations
    - Assert `Set-Cookie` header with name "kiro_access" is present in ALL responses regardless of verification outcome
    - Place test in `internal/client/challenge/pow_test.go` or a new `internal/client/proxy_property_test.go`

- [x] 9. Verify static HTML serving constraints
  - [x] 9.1 Confirm template serving uses only strings.NewReplacer
    - Verify `ServeHoldPage` and `ServeChallengePage` only perform string replacement on const templates
    - Verify no database queries, network calls, or file I/O during page serving
    - Verify response headers: `Content-Type: text/html; charset=utf-8` and `Cache-Control: no-store, no-cache, must-revalidate`
    - _Requirements: 7.1, 7.6_

  - [x] 9.2 Confirm cleanup goroutine intervals
    - Verify challenge store cleanup runs on 60-second ticker
    - Verify rate limiter cleanup runs on 120-second ticker
    - _Requirements: 7.4, 7.5_

- [x] 10. Verify cookie and binary fingerprint behavior (no changes needed)
  - [x] 10.1 Confirm cookie-before-response ordering in proxy.go
    - Verify `handleChallengeVerify` calls `setAccessCookie` before `VerifyChallenge`
    - Verify `handleHoldVerify` calls `setAccessCookie` before `VerifyHold`
    - Verify cookie attributes: name "kiro_access", HttpOnly, SameSite=Lax, Secure, Path="/", MaxAge=CookieTTL
    - Do NOT modify proxy.go — this is already correctly implemented
    - _Requirements: 10.1, 10.2_

  - [x] 10.2 Confirm binary fingerprint is disabled
    - Verify heartbeat sends empty string for `fingerprint_hash`
    - Verify no binary file reading occurs at runtime
    - Do NOT modify this behavior — already correctly implemented
    - _Requirements: 11.1, 11.2, 11.3_

- [x] 11. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties from the design document
- Unit tests validate specific examples and edge cases
- Tasks 2–5 and 9–10 are verification/audit tasks — the current code already implements most requirements. These tasks confirm correctness and fix any gaps found.
- Do NOT revert cookie ordering in proxy.go (Requirement 10)
- Do NOT revert binary fingerprint disabling (Requirement 11)
- Response size must stay under 10KB — the SVG logo replacement must not bloat the template beyond budget

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "1.2"] },
    { "id": 1, "tasks": ["2.1", "2.2", "2.3", "3.1", "3.2", "4.1", "5.1"] },
    { "id": 2, "tasks": ["7.1", "7.2", "7.3", "7.4", "9.1", "9.2", "10.1", "10.2"] },
    { "id": 3, "tasks": ["8.1"] }
  ]
}
```
