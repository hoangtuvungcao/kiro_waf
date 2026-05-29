// Package challenge triển khai JS Proof-of-Work và Hold-to-Confirm captcha.
// Cung cấp handlers cho /__kiro/challenge và /__kiro/hold endpoints.
package challenge

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// DefaultDifficulty là số ký tự "0" prefix mặc định cho PoW.
const DefaultDifficulty = 4

// ChallengeEntry lưu trữ thông tin challenge đã phát hành.
type ChallengeEntry struct {
	Token      string
	Salt       string
	ClientIP   string
	Difficulty int
	IssuedAt   time.Time
	ExpiresAt  time.Time
}

// Store quản lý các challenge đang chờ xác minh.
type Store struct {
	mu    sync.Mutex
	items map[string]ChallengeEntry
}

// NewStore tạo Store mới.
func NewStore() *Store {
	return &Store{items: make(map[string]ChallengeEntry)}
}

// Issue tạo challenge mới và lưu vào store.
func (s *Store) Issue(clientIP string, difficulty int, ttl time.Duration) ChallengeEntry {
	token := randomToken()
	salt := randomToken()
	entry := ChallengeEntry{
		Token:      token,
		Salt:       salt,
		ClientIP:   clientIP,
		Difficulty: difficulty,
		IssuedAt:   time.Now().UTC(),
		ExpiresAt:  time.Now().UTC().Add(ttl),
	}
	s.mu.Lock()
	s.items[token] = entry
	s.mu.Unlock()
	return entry
}

// Take lấy và xóa challenge khỏi store. Trả về false nếu không tìm thấy,
// IP không khớp, hoặc đã hết hạn.
func (s *Store) Take(token, clientIP string) (ChallengeEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.items[token]
	if !ok {
		return ChallengeEntry{}, false
	}
	delete(s.items, token)
	if entry.ClientIP != clientIP || time.Now().UTC().After(entry.ExpiresAt) {
		return ChallengeEntry{}, false
	}
	return entry, true
}

// Cleanup xóa các challenge đã hết hạn.
func (s *Store) Cleanup() {
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	for token, entry := range s.items {
		if now.After(entry.ExpiresAt) {
			delete(s.items, token)
		}
	}
}

// ValidPoW kiểm tra tính hợp lệ của Proof-of-Work solution.
// SHA-256(token + ":" + salt + ":" + nonce) phải bắt đầu bằng difficulty ký tự "0".
func ValidPoW(token, salt, nonce string, difficulty int) bool {
	if difficulty <= 0 {
		return true
	}
	input := token + ":" + salt + ":" + nonce
	sum := sha256.Sum256([]byte(input))
	hashHex := hex.EncodeToString(sum[:])
	prefix := strings.Repeat("0", difficulty)
	return strings.HasPrefix(hashHex, prefix)
}

// PoWHandler xử lý các request liên quan đến JS Proof-of-Work challenge.
type PoWHandler struct {
	Store      *Store
	Difficulty int
	TTL        time.Duration
}

// NewPoWHandler tạo PoWHandler mới với cấu hình mặc định.
func NewPoWHandler(store *Store, difficulty int, ttl time.Duration) *PoWHandler {
	if difficulty <= 0 {
		difficulty = DefaultDifficulty
	}
	if ttl <= 0 {
		ttl = 90 * time.Second
	}
	return &PoWHandler{
		Store:      store,
		Difficulty: difficulty,
		TTL:        ttl,
	}
}

// ServeChallengePage phục vụ trang HTML JS Proof-of-Work challenge.
// Handler cho GET /__kiro/challenge
func ServeChallengePage(w http.ResponseWriter, r *http.Request, store *Store, difficulty int, ttl time.Duration, clientIP string) {
	if difficulty <= 0 {
		difficulty = DefaultDifficulty
	}
	if ttl <= 0 {
		ttl = 90 * time.Second
	}

	entry := store.Issue(clientIP, difficulty, ttl)

	nextURL := r.URL.RequestURI()
	if nextURL == "" || nextURL == "/__kiro/challenge" {
		nextURL = "/"
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	page := strings.NewReplacer(
		"{{TOKEN}}", jsStringEscape(entry.Token),
		"{{SALT}}", jsStringEscape(entry.Salt),
		"{{DIFFICULTY}}", fmt.Sprintf("%d", entry.Difficulty),
		"{{NEXT}}", jsStringEscape(nextURL),
	).Replace(powChallengeHTML)

	_, _ = w.Write([]byte(page))
}

// VerifyChallenge xử lý POST /__kiro/challenge/verify.
// Trả về true nếu PoW solution hợp lệ.
func VerifyChallenge(w http.ResponseWriter, r *http.Request, store *Store, clientIP string) bool {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte(`{"error":"method not allowed"}`))
		return false
	}

	var req struct {
		Token string `json:"token"`
		Nonce string `json:"nonce"`
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

	if !ValidPoW(req.Token, entry.Salt, req.Nonce, entry.Difficulty) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"proof of work failed"}`))
		return false
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
	return true
}

// randomToken sinh token ngẫu nhiên 32 bytes, mã hóa base64url.
func randomToken() string {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

// jsStringEscape escape chuỗi cho sử dụng an toàn trong JavaScript string literal.
func jsStringEscape(s string) string {
	b, _ := json.Marshal(s)
	// json.Marshal wraps in quotes, strip them for template embedding
	return string(b[1 : len(b)-1])
}

// powChallengeHTML là template HTML embedded cho trang JS Proof-of-Work challenge.
// Thiết kế tông tối với branding Kiro, responsive 320px-2560px,
// chỉ báo tiến trình CSS mượt mà, văn bản tiếng Việt, noscript fallback.
const powChallengeHTML = `<!DOCTYPE html>
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

.progress-container {
  margin-bottom: 16px;
}

.progress-bar {
  height: 8px;
  border-radius: 100px;
  background: rgba(8, 17, 15, 0.8);
  border: 1px solid var(--line);
  overflow: hidden;
  position: relative;
}

.progress-fill {
  height: 100%;
  width: 5%;
  background: var(--gradient);
  border-radius: 100px;
  transition: width 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  position: relative;
}

.progress-fill::after {
  content: "";
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: linear-gradient(90deg, transparent, rgba(255,255,255,0.2), transparent);
  animation: shimmer 1.5s infinite;
}

@keyframes shimmer {
  0% { transform: translateX(-100%); }
  100% { transform: translateX(100%); }
}

.status-row {
  display: flex;
  justify-content: space-between;
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

.status-text::before {
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

.nonce-count {
  color: var(--muted);
  font-variant-numeric: tabular-nums;
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
<h1>Đang xác thực truy cập</h1>
<p class="description">Hệ thống đang kiểm tra trình duyệt của bạn trước khi cho phép truy cập. Quá trình này diễn ra tự động và chỉ mất vài giây.</p>

<noscript>
<div class="noscript-box">
<h2>JavaScript bị tắt</h2>
<p>Trang này yêu cầu JavaScript để xác thực truy cập. Vui lòng bật JavaScript trong cài đặt trình duyệt và tải lại trang.</p>
</div>
</noscript>

<div id="pow-ui">
<div class="progress-container">
<div class="progress-bar">
<div class="progress-fill" id="progress-fill"></div>
</div>
</div>
<div class="status-row">
<span class="status-text" id="status-text">Đang khởi tạo...</span>
<span class="nonce-count" id="nonce-count">0</span>
</div>
<div class="error-msg" id="error-msg">Xác thực thất bại. Vui lòng tải lại trang để thử lại.</div>
<div class="success-msg" id="success-msg">Xác thực thành công! Đang chuyển hướng...</div>
</div>

<div class="footer">Được bảo vệ bởi Kiro WAF</div>
</div>
</div>

<script>
(function(){
"use strict";
var token = "{{TOKEN}}";
var salt = "{{SALT}}";
var difficulty = {{DIFFICULTY}};
var next = "{{NEXT}}";

var fill = document.getElementById("progress-fill");
var statusEl = document.getElementById("status-text");
var nonceEl = document.getElementById("nonce-count");
var errorEl = document.getElementById("error-msg");
var successEl = document.getElementById("success-msg");

function rrot(n, x) { return (x >>> n) | (x << (32 - n)); }

function sha256(ascii) {
  var maxWord = 4294967296;
  var result = "";
  var words = [];
  var asciiBitLength = ascii.length * 8;
  var hash = [];
  var k = [];
  var candidate = 2;

  function isPrime(n) {
    for (var i = 2; i * i <= n; i++) { if (n % i === 0) return false; }
    return true;
  }
  function frac(n) { return ((n - Math.floor(n)) * maxWord) | 0; }

  for (var pc = 0; pc < 64; candidate++) {
    if (isPrime(candidate)) {
      if (pc < 8) hash[pc] = frac(Math.pow(candidate, 0.5));
      k[pc++] = frac(Math.pow(candidate, 1.0/3.0));
    }
  }

  ascii += "\x80";
  while (ascii.length % 64 - 56) ascii += "\x00";
  for (var i = 0; i < ascii.length; i++) {
    words[i >> 2] |= ascii.charCodeAt(i) << ((3 - i) % 4) * 8;
  }
  words[words.length] = ((asciiBitLength / maxWord) | 0);
  words[words.length] = asciiBitLength;

  for (var j = 0; j < words.length;) {
    var w = words.slice(j, j += 16);
    var oldHash = hash.slice(0);
    for (var i2 = 0; i2 < 64; i2++) {
      var w15 = w[i2 - 15], w2 = w[i2 - 2];
      var a = hash[0], e = hash[4];
      var temp1 = hash[7] + (rrot(6, e) ^ rrot(11, e) ^ rrot(25, e)) +
        ((e & hash[5]) ^ ((~e) & hash[6])) + k[i2] +
        (w[i2] = (i2 < 16) ? w[i2] :
          (w[i2-16] + (rrot(7,w15) ^ rrot(18,w15) ^ (w15>>>3)) +
           w[i2-7] + (rrot(17,w2) ^ rrot(19,w2) ^ (w2>>>10))) | 0);
      var temp2 = (rrot(2,a) ^ rrot(13,a) ^ rrot(22,a)) +
        ((a & hash[1]) ^ (a & hash[2]) ^ (hash[1] & hash[2]));
      hash = [(temp1 + temp2) | 0].concat(hash);
      hash[4] = (hash[4] + temp1) | 0;
      hash.pop();
    }
    for (var i3 = 0; i3 < 8; i3++) hash[i3] = (hash[i3] + oldHash[i3]) | 0;
  }

  for (var i4 = 0; i4 < 8; i4++) {
    for (var j2 = 3; j2 + 1; j2--) {
      var b = (hash[i4] >> (j2 * 8)) & 255;
      result += ((b < 16) ? "0" : "") + b.toString(16);
    }
  }
  return result;
}

function solve() {
  var prefix = "";
  for (var d = 0; d < difficulty; d++) prefix += "0";
  var nonce = 0;
  var batchSize = 500;

  statusEl.textContent = "Đang tính toán bằng chứng...";

  function batch() {
    var end = nonce + batchSize;
    for (; nonce < end; nonce++) {
      var h = sha256(token + ":" + salt + ":" + nonce);
      if (h.substring(0, difficulty) === prefix) {
        nonceEl.textContent = nonce.toLocaleString();
        fill.style.width = "100%";
        statusEl.textContent = "Đang xác nhận với máy chủ...";
        verify(nonce);
        return;
      }
    }
    nonceEl.textContent = nonce.toLocaleString();
    var pct = Math.min(95, 5 + (nonce / (Math.pow(16, difficulty) * 0.6)) * 90);
    fill.style.width = pct + "%";
    setTimeout(batch, 0);
  }

  batch();
}

function verify(nonce) {
  var xhr = new XMLHttpRequest();
  xhr.open("POST", "/__kiro/challenge/verify", true);
  xhr.setRequestHeader("Content-Type", "application/json");
  xhr.onreadystatechange = function() {
    if (xhr.readyState !== 4) return;
    if (xhr.status === 200) {
      successEl.style.display = "block";
      statusEl.textContent = "Xác thực thành công!";
      fill.style.width = "100%";
      setTimeout(function() { window.location.href = next; }, 600);
    } else {
      errorEl.style.display = "block";
      statusEl.textContent = "Xác thực thất bại";
      fill.style.width = "0%";
    }
  };
  xhr.send(JSON.stringify({token: token, nonce: String(nonce)}));
}

solve();
})();
</script>
</body>
</html>`
