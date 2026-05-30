# Requirements Document

## Introduction

Nâng cấp giao diện và tối ưu hiệu năng cho các trang challenge (Hold page và PoW page) trong Kiro WAF. Mục tiêu là cải thiện trải nghiệm người dùng với giao diện dark theme hiện đại, hiệu ứng mượt mà, đồng thời đảm bảo tối ưu tài nguyên server khi bị tấn công DDoS. Tất cả tài nguyên phải inline (không external resources), response size nhỏ nhất có thể, và challenge pages phải là static HTML thuần túy không tốn tài nguyên server.

## Glossary

- **Challenge_Page**: Trang HTML được serve bởi Kiro WAF để xác thực người dùng trước khi cho phép truy cập backend. Bao gồm Hold page và PoW page.
- **Hold_Page**: Trang challenge yêu cầu người dùng nhấn và giữ nút trong khoảng thời gian tối thiểu (mặc định 2 giây). File: `internal/client/challenge/hold.go`.
- **PoW_Page**: Trang challenge yêu cầu trình duyệt tính toán Proof-of-Work (SHA-256 hash với prefix "0" theo difficulty). File: `internal/client/challenge/pow.go`.
- **SVG_Shield_Logo**: Logo dạng SVG hình khiên (shield) với gradient teal, đã có sẵn dưới dạng base64 data URI trong favicon của các challenge pages hiện tại.
- **Client_WAF**: Reverse proxy component của Kiro WAF, xử lý rate limiting, challenge, và proxy request tới backend.
- **Challenge_Store**: In-memory store quản lý các challenge token đang chờ xác minh, có cleanup định kỳ.
- **Progress_Bar**: Thanh tiến trình hiển thị trạng thái hoàn thành của challenge (giữ nút hoặc tính hash).
- **Response_Size**: Kích thước tổng của HTTP response body (HTML + inline CSS + inline JS).

## Requirements

### Requirement 1: SVG Shield Logo hiển thị inline

**User Story:** As a user, I want to see a recognizable shield logo on challenge pages, so that I can identify the page belongs to a legitimate security system.

#### Acceptance Criteria

1. THE Hold_Page SHALL display the SVG_Shield_Logo as an inline `<svg>` element within a container div positioned immediately before the `<h1>` heading, replacing the current text-only logo div that displays the letter "K".
2. THE PoW_Page SHALL display the SVG_Shield_Logo as an inline `<svg>` element within a container div positioned immediately before the `<h1>` heading, replacing the current text-only logo div that displays the letter "K".
3. THE SVG_Shield_Logo SHALL use the SVG markup decoded from the existing base64-encoded SVG data present in the favicon `<link>` element of each Challenge_Page, embedded directly in the HTML without base64 encoding.
4. THE SVG_Shield_Logo SHALL render at a width of 48 pixels and a height of 48 pixels, using the teal gradient defined by CSS custom properties `--kiro-primary` and `--kiro-accent` already declared in the page stylesheet.
5. THE SVG_Shield_Logo inline `<svg>` element SHALL include a `role="img"` attribute and an `aria-label` attribute with a descriptive text value, so that screen readers can identify the logo.

### Requirement 2: Dark theme giao diện nâng cấp

**User Story:** As a user, I want a visually polished dark theme on challenge pages, so that the verification experience feels modern and trustworthy.

#### Acceptance Criteria

1. THE Hold_Page SHALL use a dark color scheme with a radial-gradient background transitioning from a teal-tinted midtone at the top center, through dark blue-gray (#0f172a) at 50%, to dark navy (#0b1120) at 100%.
2. THE PoW_Page SHALL use the same dark color scheme and background gradient as the Hold_Page for visual consistency.
3. THE Challenge_Page SHALL define CSS custom properties in the `:root` selector including at minimum: primary color, accent color, background color, surface color, text-primary color, text-secondary color, border color, success color, and danger color, and SHALL reference these custom properties for all color values used in the page styles.
4. THE Challenge_Page SHALL apply `backdrop-filter: blur(12px)` on the card element to create a frosted glass effect.
5. THE Challenge_Page SHALL use the system font stack (`-apple-system, BlinkMacSystemFont, "Segoe UI", Inter, Roboto, Oxygen, Ubuntu, sans-serif`) for text rendering.
6. THE Challenge_Page SHALL contain all CSS within an inline `<style>` tag in the HTML document with no external stylesheet references, no external font imports, and no external resource requests.
7. THE Challenge_Page SHALL include responsive adjustments via CSS media queries that reduce card padding and heading font size at viewport widths of 360px or below, and increase card padding and heading font size at viewport widths of 1920px or above.

### Requirement 3: Hiệu ứng animation mượt mà

**User Story:** As a user, I want smooth animations on challenge pages, so that I have clear visual feedback during the verification process.

#### Acceptance Criteria

1. WHILE the user holds the button on the Hold_Page, THE Hold_Page SHALL animate the Progress_Bar fill width from 0% to 100% proportional to elapsed time versus required hold duration, updating every 50 milliseconds.
2. WHILE the PoW computation is running on the PoW_Page, THE PoW_Page SHALL animate the Progress_Bar fill width from 5% to 95% using the formula `5 + (nonces_tried / (16^difficulty * 0.6)) * 90` percent, with a CSS shimmer overlay animation cycling every 1.5 seconds.
3. WHEN the challenge is completed successfully, THE Challenge_Page SHALL set the Progress_Bar fill width to 100%, display a success message with a teal-colored notification box, and redirect to the next URL after 600 milliseconds.
4. WHEN the challenge verification fails, THE Challenge_Page SHALL reset the Progress_Bar fill width to 0%, display an error message with a red-colored notification box, and return the page to its initial interactive state so the user can retry.
5. THE Progress_Bar on the PoW_Page SHALL use a CSS transition of 300 milliseconds with `cubic-bezier(0.4, 0, 0.2, 1)` easing for width changes.
6. WHILE the user is in the holding state on the Hold_Page, THE Hold_Page button SHALL apply a `scale(0.98)` transform, change the border color to the `--blue` brand token (#0ea5e9), and transition these properties within 100 milliseconds.

### Requirement 4: Progress bar animation cho Hold page

**User Story:** As a user, I want to see a progress bar filling up while I hold the button, so that I know how much longer I need to hold.

#### Acceptance Criteria

1. WHEN the user presses and holds the button on the Hold_Page, THE Hold_Page SHALL display a fill bar at the bottom of the button that grows from 0% to 100% width proportional to elapsed time versus the configured hold duration (default 2 seconds), updating every 50 milliseconds.
2. WHILE the user is holding the button, THE Hold_Page SHALL display a timer showing elapsed seconds in format "X.X / Y giây" where X.X is elapsed time with one decimal place and Y is the configured hold duration in whole seconds.
3. WHEN the elapsed hold time reaches the required hold duration, THE Hold_Page SHALL change the button text to "Thả để xác nhận" and the status text to "Đủ thời gian - thả nút".
4. WHEN the user releases the button before the required hold duration, THE Hold_Page SHALL reset the fill bar to 0%, display the timer text "Chưa đủ thời gian. Vui lòng giữ lâu hơn.", reset the status text to "Chờ xác thực", and reset the button text to "Nhấn và giữ để xác thực".
5. THE Hold_Page SHALL support both mouse events (mousedown, mouseup, mouseleave) and touch events (touchstart, touchend, touchcancel) for the hold interaction.
6. WHEN the user releases the button before the required hold duration, THE Hold_Page SHALL allow the user to retry the hold interaction immediately without page reload.
7. WHEN the user releases the button after the required hold duration, THE Hold_Page SHALL set the fill bar to 100%, change the button text to "Đang xác nhận...", and submit the challenge token to the verification endpoint via POST request.

### Requirement 5: Không sử dụng external resources

**User Story:** As a system operator, I want all challenge page resources to be inline, so that pages load instantly without external dependencies and cannot be blocked by network issues.

#### Acceptance Criteria

1. THE Challenge_Page SHALL embed all CSS styles within a single `<style>` element in the HTML `<head>`.
2. THE Challenge_Page SHALL embed all JavaScript within a single `<script>` element at the end of the HTML `<body>`.
3. THE Challenge_Page SHALL use only inline SVG or base64 data URIs for images and icons.
4. THE Challenge_Page SHALL not contain any `<link rel="stylesheet">` elements referencing external URLs.
5. THE Challenge_Page SHALL not contain any `<script src="...">` elements referencing external URLs.
6. THE Challenge_Page SHALL not load any external fonts (use system font stack only).
7. THE Challenge_Page SHALL not contain any CSS `@import` rules or `url()` values that reference external URLs within the `<style>` element.
8. THE Challenge_Page SHALL not contain any `<img>`, `<iframe>`, `<object>`, `<embed>`, `<audio>`, or `<video>` elements with `src` attributes referencing external URLs.

### Requirement 6: Response size tối ưu dưới 10KB

**User Story:** As a system operator, I want challenge page responses to be under 10KB, so that bandwidth usage is minimal during DDoS attacks serving thousands of challenge pages per second.

#### Acceptance Criteria

1. WHEN the Hold_Page is rendered with template variables at their maximum lengths (token: 44 characters, HOLD_SECONDS: 3 characters, NEXT: 2048 characters), THE Challenge_Page response body SHALL have a total size of less than 10240 bytes (10KB) as measured by the byte length of the HTTP response body encoded in UTF-8.
2. WHEN the PoW_Page is rendered with template variables at their maximum lengths (token: 44 characters, salt: 44 characters, DIFFICULTY: 2 characters, NEXT: 2048 characters), THE Challenge_Page response body SHALL have a total size of less than 10240 bytes (10KB) as measured by the byte length of the HTTP response body encoded in UTF-8.
3. THE Challenge_Page CSS SHALL contain no consecutive whitespace characters (spaces, tabs, newlines) between selectors, properties, or values in the rendered output, except single spaces required by CSS syntax.
4. THE Challenge_Page inline JavaScript SHALL contain no line-leading indentation whitespace and no single-line comments in the rendered output.
5. IF a code change causes either challenge page rendered output to exceed 10240 bytes when measured with maximum-length template variables, THEN THE build or test process SHALL report a failure indicating which page exceeded the size limit.

### Requirement 7: Challenge pages là static HTML thuần túy

**User Story:** As a system operator, I want challenge pages to be pure static HTML responses, so that serving them during DDoS attacks does not consume server resources (no DB queries, no memory allocation beyond the response buffer).

#### Acceptance Criteria

1. WHEN serving a Challenge_Page, THE Client_WAF SHALL only perform string replacement on the pre-defined HTML template constants (token, salt, difficulty, next URL, hold seconds) using strings.NewReplacer without any database queries, network calls, or file I/O.
2. WHEN serving a Challenge_Page, THE Client_WAF SHALL not allocate heap memory beyond the challenge token generation (32 bytes random + base64 encoding), the salt generation (32 bytes random + base64 encoding), and the response string buffer.
3. WHILE the Client_WAF is receiving requests from IPs without a valid access cookie, THE Client_WAF SHALL serve Challenge_Pages as static HTML responses without proxying any request to the backend until the client presents a valid solved challenge or hold verification.
4. THE Challenge_Store cleanup goroutine SHALL run on a fixed 60-second interval using a time.Ticker, iterating over all stored entries and deleting only those whose ExpiresAt is before the current time.
5. THE Rate_Limiter cleanup goroutine SHALL run on a fixed 120-second interval using a time.Ticker.
6. WHEN serving a Challenge_Page, THE Client_WAF SHALL return the complete HTML response in a single write call with a Content-Type header of "text/html; charset=utf-8" and a Cache-Control header of "no-store, no-cache, must-revalidate".

### Requirement 8: Noscript fallback

**User Story:** As a user with JavaScript disabled, I want to see a clear message explaining that JavaScript is required, so that I understand why the page is not working.

#### Acceptance Criteria

1. WHEN JavaScript is disabled in the browser, THE Challenge_Page SHALL display a `<noscript>` block containing an error box on both the PoW challenge page and the Hold challenge page.
2. THE noscript error box SHALL display the heading "JavaScript bị tắt" followed by the instruction text "Trang này yêu cầu JavaScript để xác thực truy cập. Vui lòng bật JavaScript trong cài đặt trình duyệt và tải lại trang." in Vietnamese.
3. THE noscript error box SHALL be styled with the dark theme using a danger-colored border and heading, a semi-transparent danger-tinted background, centered text, rounded corners, and padding consistent with other message boxes on the challenge pages.
4. WHEN JavaScript is disabled, THE Challenge_Page SHALL still render the page layout (logo, card container, footer) so that the noscript message appears within the styled card context rather than on a blank page.

### Requirement 9: Responsive design 320px đến 2560px

**User Story:** As a user on any device, I want challenge pages to display correctly on screens from 320px to 2560px wide, so that I can complete the challenge regardless of my device.

#### Acceptance Criteria

1. THE Challenge_Page SHALL use a max-width of 480px for the card container with 100% width and 16px body padding as the default layout for all viewport widths from 320px to 2560px.
2. WHILE the viewport width is 361px to 1919px, THE Challenge_Page SHALL apply card padding of 32px vertical and 28px horizontal, and a heading font size of 1.5rem.
3. WHILE the viewport width is 360px or less, THE Challenge_Page SHALL apply card padding of 24px vertical and 20px horizontal, a heading font size of 1.25rem, and button padding of 16px vertical and 24px horizontal.
4. WHILE the viewport width is 1920px or more, THE Challenge_Page SHALL apply card padding of 40px vertical and 36px horizontal, and a heading font size of 1.75rem.
5. THE Challenge_Page SHALL NOT produce horizontal scrollbars or clip interactive content at any viewport width between 320px and 2560px.
6. THE Challenge_Page SHALL render all interactive elements (buttons, hold areas) with a minimum tap target size of 44×44 CSS pixels at viewport widths of 768px or less.

### Requirement 10: Cookie được set trước khi write response

**User Story:** As a system operator, I want the access cookie to be set before the response body is written, so that the cookie is always included in the HTTP response headers.

#### Acceptance Criteria

1. WHEN a request arrives at `/__kiro/challenge/verify` or `/__kiro/hold/verify`, THE Client_WAF SHALL call `setAccessCookie` on the ResponseWriter before calling `VerifyChallenge` or `VerifyHold` (which may call `WriteHeader` or `Write`).
2. THE access cookie SHALL be set with name "kiro_access", HttpOnly flag, SameSite=Lax, Secure flag, Path="/", and MaxAge equal to the configured cookie TTL (default: 1200 seconds).
3. IF the subsequent verification fails (PoW solution invalid, hold duration too short, or token expired), THEN THE Client_WAF SHALL return the appropriate 4xx status code, and the pre-set Set-Cookie header SHALL remain in the response (no explicit removal required, as the failed response prevents the client from using the cookie for access).

### Requirement 11: Binary fingerprint disabled

**User Story:** As a system operator, I want binary fingerprint verification to be disabled, so that deploying a new binary does not trigger lockout of the node.

#### Acceptance Criteria

1. THE Client_WAF SHALL send an empty string for the fingerprint_hash field in heartbeat requests to the master server.
2. THE Client_WAF SHALL not open or read the executable binary file for SHA-256 hash computation at any point during its runtime lifecycle (startup, heartbeat loop, or request handling).
3. THE Client_WAF SHALL not expose any configuration option or environment variable that enables binary fingerprint verification.
