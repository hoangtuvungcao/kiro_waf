package challenge

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// DefaultHoldSeconds là thời gian giữ tối thiểu mặc định (giây).
const DefaultHoldSeconds = 2

// ValidHold kiểm tra thời gian giữ có đủ holdSeconds hay không.
// Trả về true khi verifyAt - issuedAt >= holdSeconds.
func ValidHold(issuedAt time.Time, verifyAt time.Time, holdSeconds int) bool {
	if holdSeconds <= 0 {
		return true
	}
	elapsed := verifyAt.Sub(issuedAt)
	return elapsed >= time.Duration(holdSeconds)*time.Second
}

// ServeHoldPage phục vụ trang HTML Hold-to-Confirm captcha.
// Handler cho GET /__kiro/hold
func ServeHoldPage(w http.ResponseWriter, r *http.Request, store *Store, holdSeconds int, ttl time.Duration, clientIP string) {
	if holdSeconds <= 0 {
		holdSeconds = DefaultHoldSeconds
	}
	if ttl <= 0 {
		ttl = 90 * time.Second
	}

	entry := store.Issue(clientIP, 0, ttl)

	nextURL := r.URL.RequestURI()
	if nextURL == "" || nextURL == "/__kiro/hold" {
		nextURL = "/"
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	page := strings.NewReplacer(
		"{{TOKEN}}", jsStringEscape(entry.Token),
		"{{HOLD_SECONDS}}", intToStr(holdSeconds),
		"{{NEXT}}", jsStringEscape(nextURL),
	).Replace(holdCaptchaHTML)

	_, _ = w.Write([]byte(page))
}

// VerifyHold xử lý POST /__kiro/hold/verify.
// Trả về true nếu hold duration >= holdSeconds (mặc định 2 giây).
func VerifyHold(w http.ResponseWriter, r *http.Request, store *Store, clientIP string) bool {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte(`{"error":"method not allowed"}`))
		return false
	}

	var req struct {
		Token string `json:"token"`
	}

	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
	if err := decoder.Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid request body"}`))
		return false
	}

	entry, ok := store.Take(req.Token, clientIP)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"challenge expired or invalid"}`))
		return false
	}

	now := time.Now().UTC()
	if !ValidHold(entry.IssuedAt, now, DefaultHoldSeconds) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"hold duration too short"}`))
		return false
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
	return true
}

// intToStr chuyển int thành string (tránh import strconv cho helper nhỏ).
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + intToStr(-n)
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

// holdCaptchaHTML là template HTML embedded cho trang Hold-to-Confirm captcha.
// Thiết kế tông tối với branding Kiro, responsive 320px-2560px,
// nút giữ tối thiểu 2 giây, văn bản tiếng Việt, noscript fallback.
const holdCaptchaHTML = `<!DOCTYPE html>
<html lang="vi">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Kiro - Xác Thực Truy Cập</title>
<style>
:root {
  color-scheme: dark;
  --bg: #07100f;
  --panel: #0f1a17;
  --line: #29443e;
  --text: #eef8f5;
  --muted: #a9bdb7;
  --green: #67d891;
  --cyan: #64d8c9;
  --red: #ff8a8a;
  --gradient: linear-gradient(135deg, var(--green), var(--cyan));
}

* {
  box-sizing: border-box;
  margin: 0;
  padding: 0;
}

body {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: radial-gradient(ellipse at 50% 0%, #17302b 0%, #07100f 50%, #050807 100%);
  color: var(--text);
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Inter, Roboto, Oxygen, Ubuntu, sans-serif;
  line-height: 1.5;
  padding: 16px;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

.container {
  width: 100%;
  max-width: 480px;
  min-width: 0;
}

.card {
  border: 1px solid var(--line);
  border-radius: 12px;
  background: rgba(15, 26, 23, 0.97);
  padding: 32px 28px;
  box-shadow: 0 32px 80px rgba(0, 0, 0, 0.4), 0 0 0 1px rgba(103, 216, 145, 0.04);
  backdrop-filter: blur(8px);
}

.logo {
  width: 48px;
  height: 48px;
  border-radius: 10px;
  background: var(--gradient);
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 900;
  font-size: 1.4rem;
  color: #06100e;
  margin-bottom: 20px;
  box-shadow: 0 4px 16px rgba(100, 216, 201, 0.2);
}

h1 {
  font-size: 1.5rem;
  font-weight: 700;
  margin-bottom: 8px;
  letter-spacing: -0.02em;
}

.description {
  color: var(--muted);
  font-size: 0.95rem;
  margin-bottom: 24px;
  line-height: 1.6;
}

.hold-area {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
  margin-bottom: 16px;
}

.hold-btn {
  width: 100%;
  max-width: 320px;
  padding: 18px 32px;
  border: 2px solid var(--line);
  border-radius: 10px;
  background: rgba(8, 17, 15, 0.8);
  color: var(--text);
  font-size: 1.05rem;
  font-weight: 600;
  cursor: pointer;
  user-select: none;
  -webkit-user-select: none;
  touch-action: manipulation;
  transition: border-color 0.2s, background 0.2s, transform 0.1s;
  position: relative;
  overflow: hidden;
}

.hold-btn:hover {
  border-color: var(--green);
  background: rgba(103, 216, 145, 0.05);
}

.hold-btn:active,
.hold-btn.holding {
  border-color: var(--cyan);
  background: rgba(100, 216, 201, 0.08);
  transform: scale(0.98);
}

.hold-btn .fill-bar {
  position: absolute;
  left: 0;
  bottom: 0;
  height: 4px;
  width: 0%;
  background: var(--gradient);
  border-radius: 0 0 8px 8px;
  transition: width 0.1s linear;
}

.hold-btn.holding .fill-bar {
  transition: width 0.1s linear;
}

.hold-timer {
  font-size: 0.9rem;
  color: var(--muted);
  font-variant-numeric: tabular-nums;
  min-height: 1.4em;
}

.status-row {
  display: flex;
  justify-content: center;
  align-items: center;
  margin-top: 12px;
  font-size: 0.875rem;
}

.status-text {
  color: var(--green);
  display: flex;
  align-items: center;
  gap: 6px;
}

.status-text.waiting::before {
  content: "";
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--muted);
}

.status-text.active::before {
  content: "";
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--green);
  animation: pulse 1.2s infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}

.error-msg {
  color: var(--red);
  margin-top: 16px;
  padding: 12px 16px;
  border-radius: 8px;
  background: rgba(255, 138, 138, 0.08);
  border: 1px solid rgba(255, 138, 138, 0.2);
  font-size: 0.9rem;
  display: none;
  text-align: center;
}

.success-msg {
  color: var(--green);
  margin-top: 16px;
  padding: 12px 16px;
  border-radius: 8px;
  background: rgba(103, 216, 145, 0.08);
  border: 1px solid rgba(103, 216, 145, 0.2);
  font-size: 0.9rem;
  display: none;
  text-align: center;
}

.footer {
  margin-top: 20px;
  padding-top: 16px;
  border-top: 1px solid var(--line);
  text-align: center;
  color: var(--muted);
  font-size: 0.8rem;
}

noscript .noscript-box {
  padding: 20px;
  border-radius: 8px;
  background: rgba(255, 138, 138, 0.08);
  border: 1px solid rgba(255, 138, 138, 0.2);
  text-align: center;
}

noscript .noscript-box h2 {
  color: var(--red);
  font-size: 1.1rem;
  margin-bottom: 8px;
}

noscript .noscript-box p {
  color: var(--muted);
  font-size: 0.9rem;
}

@media (max-width: 360px) {
  .card {
    padding: 24px 20px;
  }
  h1 {
    font-size: 1.25rem;
  }
  .hold-btn {
    padding: 16px 24px;
    font-size: 0.95rem;
  }
}

@media (min-width: 1920px) {
  .card {
    padding: 40px 36px;
  }
  h1 {
    font-size: 1.75rem;
  }
}
</style>
</head>
<body>
<div class="container">
<div class="card">
<div class="logo">K</div>
<h1>Xác thực truy cập</h1>
<p class="description">Vui lòng nhấn và giữ nút bên dưới trong ít nhất {{HOLD_SECONDS}} giây để xác minh bạn là người thật.</p>

<noscript>
<div class="noscript-box">
<h2>JavaScript bị tắt</h2>
<p>Trang này yêu cầu JavaScript để xác thực truy cập. Vui lòng bật JavaScript trong cài đặt trình duyệt và tải lại trang.</p>
</div>
</noscript>

<div id="hold-ui">
<div class="hold-area">
<button class="hold-btn" id="hold-btn" type="button">
<span id="btn-text">Nhấn và giữ để xác thực</span>
<div class="fill-bar" id="fill-bar"></div>
</button>
<div class="hold-timer" id="hold-timer"></div>
</div>
<div class="status-row">
<span class="status-text waiting" id="status-text">Chờ xác thực</span>
</div>
<div class="error-msg" id="error-msg">Xác thực thất bại. Vui lòng thử lại.</div>
<div class="success-msg" id="success-msg">Xác thực thành công! Đang chuyển hướng...</div>
</div>

<div class="footer">Được bảo vệ bởi Kiro WAF</div>
</div>
</div>

<script>
(function(){
"use strict";
var token = "{{TOKEN}}";
var holdSeconds = {{HOLD_SECONDS}};
var next = "{{NEXT}}";

var btn = document.getElementById("hold-btn");
var btnText = document.getElementById("btn-text");
var fillBar = document.getElementById("fill-bar");
var timerEl = document.getElementById("hold-timer");
var statusEl = document.getElementById("status-text");
var errorEl = document.getElementById("error-msg");
var successEl = document.getElementById("success-msg");

var holdStart = 0;
var holdInterval = null;
var completed = false;

function updateTimer() {
  if (completed) return;
  var elapsed = (Date.now() - holdStart) / 1000;
  var pct = Math.min(100, (elapsed / holdSeconds) * 100);
  fillBar.style.width = pct + "%";
  timerEl.textContent = elapsed.toFixed(1) + " / " + holdSeconds + " giây";

  if (elapsed >= holdSeconds) {
    btnText.textContent = "Thả để xác nhận";
    statusEl.className = "status-text active";
    statusEl.textContent = "Đủ thời gian - thả nút";
  }
}

function startHold(e) {
  if (completed) return;
  e.preventDefault();
  holdStart = Date.now();
  btn.classList.add("holding");
  statusEl.className = "status-text active";
  statusEl.textContent = "Đang giữ...";
  errorEl.style.display = "none";
  holdInterval = setInterval(updateTimer, 50);
  updateTimer();
}

function endHold(e) {
  if (completed || !holdStart) return;
  e.preventDefault();
  clearInterval(holdInterval);
  holdInterval = null;
  btn.classList.remove("holding");

  var elapsed = (Date.now() - holdStart) / 1000;
  holdStart = 0;

  if (elapsed >= holdSeconds) {
    completed = true;
    fillBar.style.width = "100%";
    btnText.textContent = "Đang xác nhận...";
    statusEl.className = "status-text active";
    statusEl.textContent = "Đang xác nhận với máy chủ...";
    verify();
  } else {
    fillBar.style.width = "0%";
    timerEl.textContent = "Chưa đủ thời gian. Vui lòng giữ lâu hơn.";
    statusEl.className = "status-text waiting";
    statusEl.textContent = "Chờ xác thực";
    btnText.textContent = "Nhấn và giữ để xác thực";
  }
}

function verify() {
  var xhr = new XMLHttpRequest();
  xhr.open("POST", "/__kiro/hold/verify", true);
  xhr.setRequestHeader("Content-Type", "application/json");
  xhr.onreadystatechange = function() {
    if (xhr.readyState !== 4) return;
    if (xhr.status === 200) {
      successEl.style.display = "block";
      statusEl.className = "status-text active";
      statusEl.textContent = "Xác thực thành công!";
      btn.disabled = true;
      btnText.textContent = "Đã xác thực";
      setTimeout(function() { window.location.href = next; }, 600);
    } else {
      completed = false;
      fillBar.style.width = "0%";
      errorEl.style.display = "block";
      statusEl.className = "status-text waiting";
      statusEl.textContent = "Chờ xác thực";
      btnText.textContent = "Nhấn và giữ để xác thực";
      timerEl.textContent = "";
    }
  };
  xhr.send(JSON.stringify({token: token}));
}

// Mouse events
btn.addEventListener("mousedown", startHold);
btn.addEventListener("mouseup", endHold);
btn.addEventListener("mouseleave", endHold);

// Touch events
btn.addEventListener("touchstart", startHold);
btn.addEventListener("touchend", endHold);
btn.addEventListener("touchcancel", endHold);
})();
</script>
</body>
</html>`
