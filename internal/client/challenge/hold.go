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

	// Success — do NOT write response here; caller handles it
	// so that Set-Cookie header can be added before WriteHeader.
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

// holdCaptchaHTML là template HTML cho trang Hold-to-Confirm captcha.
const holdCaptchaHTML = `<!DOCTYPE html><html lang="vi"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Kiro - Xác Thực Truy Cập</title><style>:root{color-scheme:dark;--kiro-primary:#0d9488;--kiro-accent:#14b8a6;--kiro-background:#0f0f1a;--kiro-surface:#1a1a2e;--kiro-text-primary:#f0f0f0;--kiro-text-secondary:#a0a0b0;--kiro-border:#2a2a3e;--kiro-success:#10b981;--kiro-danger:#ef4444;--line:rgba(13,148,136,.25);--text:var(--kiro-text-primary);--muted:var(--kiro-text-secondary);--teal:var(--kiro-accent);--red:var(--kiro-danger)}*{box-sizing:border-box;margin:0;padding:0}body{min-height:100vh;background:radial-gradient(ellipse at 50% 0%,#1e3a4a 0%,#0f172a 50%,#0b1120 100%);color:var(--text);font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Inter,Roboto,Oxygen,Ubuntu,sans-serif;line-height:1.5;padding:16px}.container{width:100%;max-width:480px}.card{border:1px solid var(--line);border-radius:14px;background:rgba(30,41,59,.95);padding:32px 28px;box-shadow:0 32px 80px rgba(0,0,0,.4);backdrop-filter:blur(12px)}.logo{width:48px;height:48px;margin-bottom:20px}h1{font-size:1.5rem;font-weight:700;margin-bottom:8px}.description{color:var(--muted);font-size:.95rem;margin-bottom:24px}.hold-area{display:flex;flex-direction:column;align-items:center;gap:16px}.hold-btn{width:100%;padding:18px 32px;border:2px solid var(--line);border-radius:12px;background:rgba(15,23,42,.8);color:var(--text);font-size:1.05rem;user-select:none;transition:border-color .1s,background .1s,transform .1s;position:relative}.hold-btn:active,.hold-btn.holding{border-color:#0ea5e9;background:rgba(14,165,233,.08);transform:scale(.98)}.hold-btn .fill-bar{position:absolute;left:0;bottom:0;height:4px;width:0%;background:linear-gradient(135deg,var(--kiro-primary),#0284c7)}.hold-timer{font-size:.9rem;color:var(--muted)}.status-row{display:flex;justify-content:center;align-items:center;margin-top:12px;font-size:.875rem}.status-text{color:var(--teal);display:flex;align-items:center;gap:6px}.status-text.waiting::before{content:"";width:6px;height:6px;border-radius:50%;background:var(--muted)}.status-text.active::before{content:"";width:6px;height:6px;border-radius:50%;background:var(--teal);animation:pulse 1.2s infinite}@keyframes pulse{0%,100%{opacity:1}50%{opacity:.4}}.error-msg{color:var(--red);margin-top:16px;padding:12px 16px;border-radius:10px;background:rgba(248,113,113,.08);border:1px solid rgba(248,113,113,.2);font-size:.9rem;display:none;text-align:center}.success-msg{color:var(--teal);margin-top:16px;padding:12px 16px;border-radius:10px;background:rgba(20,184,166,.08);border:1px solid rgba(20,184,166,.2);font-size:.9rem;display:none;text-align:center}.footer{margin-top:20px;padding-top:16px;border-top:1px solid var(--line);text-align:center;color:var(--muted);font-size:.8rem}noscript .noscript-box{padding:20px;border-radius:10px;background:rgba(248,113,113,.08);border:1px solid rgba(248,113,113,.2);text-align:center}noscript .noscript-box h2{color:var(--red);font-size:1.1rem;margin-bottom:8px}noscript .noscript-box p{color:var(--muted);font-size:.9rem}@media (max-width: 360px){.card{padding:24px 20px}h1{font-size:1.25rem}.hold-btn{padding:16px 24px;font-size:.95rem}}@media (min-width: 1920px){.card{padding:40px 36px}h1{font-size:1.75rem}}</style></head><body><div class="container"><div class="card"><div class="logo"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" width="48" height="48" role="img" aria-label="Kiro WAF Shield Logo"><defs><linearGradient id="fg" x1="0%" y1="0%" x2="100%" y2="100%"><stop offset="0%" stop-color="#14b8a6"/><stop offset="100%" stop-color="#0d9488"/></linearGradient></defs><path d="M16 2L28 7C28 7 29 17.5 24 23 20.5 27 16 30 16 30 16 30 11.5 27 8 23 3 17.5 4 7 4 7Z" fill="url(#fg)"/><path d="M16 5L26 9C26 9 27 17 22.5 21.5 19.5 24.5 16 27 16 27 16 27 12.5 24.5 9.5 21.5 5 17 6 9 6 9Z" fill="#0f172a"/><rect x="12" y="17" width="8" height="7" rx="1.5" fill="#14b8a6"/><path d="M13.5 17V14.5C13.5 12.5 14.5 11.5 16 11.5 17.5 11.5 18.5 12.5 18.5 14.5V17" fill="none" stroke="#14b8a6" stroke-width="2" stroke-linecap="round"/><circle cx="16" cy="20" r="1.5" fill="#0f172a"/><rect x="15.5" y="20" width="1" height="2.5" rx="0.5" fill="#0f172a"/></svg></div><h1>Xác thực truy cập</h1><p class="description">Vui lòng nhấn và giữ nút bên dưới trong ít nhất {{HOLD_SECONDS}} giây để xác minh bạn là người thật.</p><noscript><div class="noscript-box"><h2>JavaScript bị tắt</h2><p>Trang này yêu cầu JavaScript để xác thực truy cập. Vui lòng bật JavaScript trong cài đặt trình duyệt và tải lại trang.</p></div></noscript><div id="hold-ui"><div class="hold-area"><button class="hold-btn" id="hold-btn" type="button"><span id="btn-text">Nhấn và giữ để xác thực</span><div class="fill-bar" id="fill-bar"></div></button><div class="hold-timer" id="hold-timer"></div></div><div class="status-row"><span class="status-text waiting" id="status-text">Chờ xác thực</span></div><div class="error-msg" id="error-msg">Xác thực thất bại. Vui lòng thử lại.</div><div class="success-msg" id="success-msg">Xác thực thành công! Đang chuyển hướng...</div></div><div class="footer">Được bảo vệ bởi Kiro WAF</div></div></div><script>(function(){"use strict";var token="{{TOKEN}}";var holdSeconds={{HOLD_SECONDS}};var next="{{NEXT}}";var btn=document.getElementById("hold-btn");var btnText=document.getElementById("btn-text");var fillBar=document.getElementById("fill-bar");var timerEl=document.getElementById("hold-timer");var statusEl=document.getElementById("status-text");var errorEl=document.getElementById("error-msg");var successEl=document.getElementById("success-msg");var holdStart=0;var holdInterval=null;var completed=false;function updateTimer(){if(completed)return;var elapsed=(Date.now()-holdStart)/1000;var pct=Math.min(100,(elapsed/holdSeconds)*100);fillBar.style.width=pct+"%";timerEl.textContent=elapsed.toFixed(1)+" / "+holdSeconds+" giây";if(elapsed>=holdSeconds){btnText.textContent="Thả để xác nhận";statusEl.className="status-text active";statusEl.textContent="Đủ thời gian - thả nút";}}function startHold(e){if(completed)return;e.preventDefault();holdStart=Date.now();btn.classList.add("holding");statusEl.className="status-text active";statusEl.textContent="Đang giữ...";errorEl.style.display="none";holdInterval=setInterval(updateTimer,50);updateTimer();}function endHold(e){if(completed||!holdStart)return;e.preventDefault();clearInterval(holdInterval);holdInterval=null;btn.classList.remove("holding");var elapsed=(Date.now()-holdStart)/1000;holdStart=0;if(elapsed>=holdSeconds){completed=true;fillBar.style.width="100%";btnText.textContent="Đang xác nhận...";statusEl.className="status-text active";statusEl.textContent="Đang xác nhận với máy chủ...";verify();}else{fillBar.style.width="0%";timerEl.textContent="Chưa đủ thời gian. Vui lòng giữ lâu hơn.";statusEl.className="status-text waiting";statusEl.textContent="Chờ xác thực";btnText.textContent="Nhấn và giữ để xác thực";}}function verify(){var xhr=new XMLHttpRequest();xhr.open("POST","/__kiro/hold/verify",true);xhr.setRequestHeader("Content-Type","application/json");xhr.onreadystatechange=function(){if(xhr.readyState!==4)return;if(xhr.status===200){successEl.style.display="block";statusEl.className="status-text active";statusEl.textContent="Xác thực thành công!";btn.disabled=true;btnText.textContent="Đã xác thực";setTimeout(function(){window.location.href=next;},600);}else{completed=false;fillBar.style.width="0%";errorEl.style.display="block";statusEl.className="status-text waiting";statusEl.textContent="Chờ xác thực";btnText.textContent="Nhấn và giữ để xác thực";timerEl.textContent="";}};xhr.send(JSON.stringify({token:token}));}btn.addEventListener("mousedown",startHold);btn.addEventListener("mouseup",endHold);btn.addEventListener("mouseleave",endHold);btn.addEventListener("touchstart",startHold);btn.addEventListener("touchend",endHold);btn.addEventListener("touchcancel",endHold);})();</script></body></html>`
