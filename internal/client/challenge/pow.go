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

// CleanupAt xóa các challenge đã hết hạn tại thời điểm cho trước (for testing).
func (s *Store) CleanupAt(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for token, entry := range s.items {
		if now.After(entry.ExpiresAt) {
			delete(s.items, token)
		}
	}
}

// IssueAt tạo challenge mới tại thời điểm cho trước (for testing).
func (s *Store) IssueAt(clientIP string, difficulty int, ttl time.Duration, issuedAt time.Time) ChallengeEntry {
	token := randomToken()
	salt := randomToken()
	entry := ChallengeEntry{
		Token:      token,
		Salt:       salt,
		ClientIP:   clientIP,
		Difficulty: difficulty,
		IssuedAt:   issuedAt,
		ExpiresAt:  issuedAt.Add(ttl),
	}
	s.mu.Lock()
	s.items[token] = entry
	s.mu.Unlock()
	return entry
}

// Has returns true if the token exists in the store (for testing).
func (s *Store) Has(token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, exists := s.items[token]
	return exists
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

	// Success — do NOT write response here; caller handles it
	// so that Set-Cookie header can be added before WriteHeader.
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

// powChallengeHTML là template HTML cho trang JS Proof-of-Work challenge.
const powChallengeHTML = `<!DOCTYPE html><html lang="vi"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Kiro - Xác Thực Truy Cập</title><style>:root{color-scheme:dark;--kiro-primary:#0d9488;--kiro-accent:#14b8a6;--kiro-background:#0f0f1a;--kiro-surface:#1a1a2e;--kiro-text-primary:#f0f0f0;--kiro-text-secondary:#a0a0b0;--kiro-border:#2a2a3e;--kiro-success:#10b981;--kiro-danger:#ef4444;--line:rgba(13,148,136,.25);--text:var(--kiro-text-primary);--muted:var(--kiro-text-secondary);--teal:var(--kiro-accent);--red:var(--kiro-danger)}*{box-sizing:border-box;margin:0;padding:0}body{min-height:100vh;display:flex;align-items:center;justify-content:center;background:radial-gradient(ellipse at 50% 0%,#1e3a4a 0%,#0f172a 50%,#0b1120 100%);color:var(--text);font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Inter,Roboto,Oxygen,Ubuntu,sans-serif;line-height:1.5;padding:16px}.container{width:100%;max-width:480px}.card{border:1px solid var(--line);border-radius:14px;background:rgba(30,41,59,.95);padding:32px 28px;box-shadow:0 32px 80px rgba(0,0,0,.4);backdrop-filter:blur(12px)}.logo{width:48px;height:48px;margin-bottom:20px}h1{font-size:1.5rem;font-weight:700;margin-bottom:8px}.description{color:var(--muted);font-size:.95rem;margin-bottom:24px}.progress-container{margin-bottom:16px}.progress-bar{height:8px;border-radius:4px;background:rgba(15,23,42,.8);border:1px solid var(--line);overflow:hidden}.progress-fill{height:100%;width:5%;background:linear-gradient(135deg,var(--kiro-primary),#0284c7);border-radius:4px;transition:width .3s cubic-bezier(.4,0,.2,1)}.status-row{display:flex;justify-content:space-between;align-items:center;margin-top:12px;font-size:.875rem}.status-text{color:var(--teal);display:flex;align-items:center;gap:6px}.status-text::before{content:"";width:6px;height:6px;border-radius:50%;background:var(--teal);animation:pulse 1.2s infinite}@keyframes pulse{0%,100%{opacity:1}50%{opacity:.4}}.nonce-count{color:var(--muted);font-variant-numeric:tabular-nums}.error-msg{color:var(--red);margin-top:16px;padding:12px 16px;border-radius:10px;background:rgba(248,113,113,.08);border:1px solid rgba(248,113,113,.2);font-size:.9rem;display:none}.success-msg{color:var(--teal);margin-top:16px;padding:12px 16px;border-radius:10px;background:rgba(20,184,166,.08);border:1px solid rgba(20,184,166,.2);font-size:.9rem;display:none}.footer{margin-top:20px;padding-top:16px;border-top:1px solid var(--line);text-align:center;color:var(--muted);font-size:.8rem}noscript .noscript-box{padding:20px;border-radius:10px;background:rgba(248,113,113,.08);border:1px solid rgba(248,113,113,.2);text-align:center}noscript .noscript-box h2{color:var(--red);font-size:1.1rem;margin-bottom:8px}noscript .noscript-box p{color:var(--muted);font-size:.9rem}@media (max-width: 360px){.card{padding:24px 20px}h1{font-size:1.25rem}}@media (min-width: 1920px){.card{padding:40px 36px}h1{font-size:1.75rem}}</style></head><body><div class="container"><div class="card"><div class="logo"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" width="48" height="48" role="img" aria-label="Kiro WAF Shield Logo"><defs><linearGradient id="fg" x1="0%" y1="0%" x2="100%" y2="100%"><stop offset="0%" stop-color="#14b8a6"/><stop offset="100%" stop-color="#0d9488"/></linearGradient></defs><path d="M16 2L28 7C28 7 29 17.5 24 23 20.5 27 16 30 16 30 16 30 11.5 27 8 23 3 17.5 4 7 4 7Z" fill="url(#fg)"/><path d="M16 5L26 9C26 9 27 17 22.5 21.5 19.5 24.5 16 27 16 27 16 27 12.5 24.5 9.5 21.5 5 17 6 9 6 9Z" fill="#0f172a"/><rect x="12" y="17" width="8" height="7" rx="1.5" fill="#14b8a6"/><path d="M13.5 17V14.5C13.5 12.5 14.5 11.5 16 11.5 17.5 11.5 18.5 12.5 18.5 14.5V17" fill="none" stroke="#14b8a6" stroke-width="2" stroke-linecap="round"/><circle cx="16" cy="20" r="1.5" fill="#0f172a"/><rect x="15.5" y="20" width="1" height="2.5" rx="0.5" fill="#0f172a"/></svg></div><h1>Đang xác thực truy cập</h1><p class="description">Hệ thống đang kiểm tra trình duyệt của bạn trước khi cho phép truy cập. Quá trình này diễn ra tự động và chỉ mất vài giây.</p><noscript><div class="noscript-box"><h2>JavaScript bị tắt</h2><p>Trang này yêu cầu JavaScript để xác thực truy cập. Vui lòng bật JavaScript trong cài đặt trình duyệt và tải lại trang.</p></div></noscript><div id="pow-ui"><div class="progress-container"><div class="progress-bar"><div class="progress-fill" id="progress-fill"></div></div></div><div class="status-row"><span class="status-text" id="status-text">Đang khởi tạo...</span><span class="nonce-count" id="nonce-count">0</span></div><div class="error-msg" id="error-msg">Xác thực thất bại. Vui lòng tải lại trang để thử lại.</div><div class="success-msg" id="success-msg">Xác thực thành công! Đang chuyển hướng...</div></div><div class="footer">Được bảo vệ bởi Kiro WAF</div></div></div><script>(function(){"use strict";var token="{{TOKEN}}";var salt="{{SALT}}";var difficulty={{DIFFICULTY}};var next="{{NEXT}}";var fill=document.getElementById("progress-fill");var statusEl=document.getElementById("status-text");var nonceEl=document.getElementById("nonce-count");var errorEl=document.getElementById("error-msg");var successEl=document.getElementById("success-msg");function rrot(n,x){return(x>>>n)|(x<<(32-n));}function sha256(ascii){var maxWord=4294967296;var result="";var words=[];var asciiBitLength=ascii.length*8;var hash=[];var k=[];var candidate=2;function isPrime(n){for(var i=2;i*i<=n;i++){if(n%i===0)return false;}return true;}function frac(n){return((n-Math.floor(n))*maxWord)|0;}for(var pc=0;pc<64;candidate++){if(isPrime(candidate)){if(pc<8)hash[pc]=frac(Math.pow(candidate,.5));k[pc++]=frac(Math.pow(candidate,1/3));}}ascii+="\x80";while(ascii.length%64-56)ascii+="\x00";for(var i=0;i<ascii.length;i++){words[i>>2]|=ascii.charCodeAt(i)<<((3-i)%4)*8;}words[words.length]=((asciiBitLength/maxWord)|0);words[words.length]=asciiBitLength;for(var j=0;j<words.length;){var w=words.slice(j,j+=16);var oldHash=hash.slice(0);for(var i2=0;i2<64;i2++){var w15=w[i2-15],w2=w[i2-2];var a=hash[0],e=hash[4];var temp1=hash[7]+(rrot(6,e)^rrot(11,e)^rrot(25,e))+((e&hash[5])^((~e)&hash[6]))+k[i2]+(w[i2]=(i2<16)?w[i2]:(w[i2-16]+(rrot(7,w15)^rrot(18,w15)^(w15>>>3))+w[i2-7]+(rrot(17,w2)^rrot(19,w2)^(w2>>>10)))|0);var temp2=(rrot(2,a)^rrot(13,a)^rrot(22,a))+((a&hash[1])^(a&hash[2])^(hash[1]&hash[2]));hash=[(temp1+temp2)|0].concat(hash);hash[4]=(hash[4]+temp1)|0;hash.pop();}for(var i3=0;i3<8;i3++)hash[i3]=(hash[i3]+oldHash[i3])|0;}for(var i4=0;i4<8;i4++){for(var j2=3;j2+1;j2--){var b=(hash[i4]>>(j2*8))&255;result+=((b<16)?"0":"")+b.toString(16);}}return result;}function solve(){var prefix="";for(var d=0;d<difficulty;d++)prefix+="0";var nonce=0;var batchSize=500;statusEl.textContent="Đang tính toán bằng chứng...";function batch(){var end=nonce+batchSize;for(;nonce<end;nonce++){var h=sha256(token+":"+salt+":"+nonce);if(h.substring(0,difficulty)===prefix){nonceEl.textContent=nonce.toLocaleString();fill.style.width="100%";statusEl.textContent="Đang xác nhận với máy chủ...";verify(nonce);return;}}nonceEl.textContent=nonce.toLocaleString();var pct=Math.min(95,5+(nonce/(Math.pow(16,difficulty)*.6))*90);fill.style.width=pct+"%";setTimeout(batch,0);}batch();}function verify(nonce){var xhr=new XMLHttpRequest();xhr.open("POST","/__kiro/challenge/verify",true);xhr.setRequestHeader("Content-Type","application/json");xhr.onreadystatechange=function(){if(xhr.readyState!==4)return;if(xhr.status===200){successEl.style.display="block";statusEl.textContent="Xác thực thành công!";fill.style.width="100%";setTimeout(function(){window.location.href=next;},600);}else{errorEl.style.display="block";statusEl.textContent="Xác thực thất bại";fill.style.width="0%";}};xhr.send(JSON.stringify({token:token,nonce:String(nonce)}));}solve();})();</script></body></html>`// powChallengeHTML là template HTML embedded cho trang JS Proof-of-Work challenge.
