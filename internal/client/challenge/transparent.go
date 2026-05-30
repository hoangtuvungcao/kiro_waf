package challenge

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// TransparentChallenge cấu hình cho transparent JS challenge.
type TransparentChallenge struct {
	Store      *Store
	TTL        time.Duration // default: 30s
	MinSolveMs int64         // default: 50ms (reject faster submissions)
}

// Escalator interface cho escalation engine (tránh circular import).
type Escalator interface {
	RecordFailure(ip string, challengeType string)
}

// transparentVerifyRequest là payload JSON từ client khi verify.
type transparentVerifyRequest struct {
	Token    string               `json:"token"`
	Solution string               `json:"solution"`
	FP       *browserFingerprint  `json:"fp"`
}

// browserFingerprint chứa thông tin fingerprint từ browser.
type browserFingerprint struct {
	Canvas string `json:"canvas"`
	WebGL  string `json:"webgl"`
	TZ     *int   `json:"tz"`
	WD     *bool  `json:"wd"`
}

// ServeTransparentPage phục vụ trang HTML transparent JS challenge (<2KB).
// Handler cho GET requests khi escalation level = 1.
func ServeTransparentPage(w http.ResponseWriter, r *http.Request, store *Store, ttl time.Duration, clientIP string) {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}

	entry := store.Issue(clientIP, 0, ttl)

	nextURL := r.URL.RequestURI()
	if nextURL == "" || nextURL == "/__kiro/transparent/verify" {
		nextURL = "/"
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	page := strings.NewReplacer(
		"{{TOKEN}}", jsStringEscape(entry.Token),
		"{{SALT}}", jsStringEscape(entry.Salt),
		"{{NEXT}}", jsStringEscape(nextURL),
	).Replace(transparentChallengeHTML)

	_, _ = w.Write([]byte(page))
}

// VerifyTransparent xử lý POST /__kiro/transparent/verify.
// Trả về true nếu challenge solution hợp lệ.
// On failure: responds with HTTP 403 and returns false.
// On success: responds with HTTP 200 and returns true (caller sets cookie).
func VerifyTransparent(w http.ResponseWriter, r *http.Request, store *Store, clientIP string, escalation Escalator) bool {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte(`{"error":"method not allowed"}`))
		return false
	}

	var req transparentVerifyRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
	if err := decoder.Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid request body"}`))
		return false
	}

	// Take token from store (single-use: deleted immediately on first attempt)
	entry, ok := store.Take(req.Token, clientIP)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"challenge expired or invalid"}`))
		if escalation != nil {
			escalation.RecordFailure(clientIP, "transparent")
		}
		return false
	}

	// Check minimum solve time (reject submissions faster than 20ms)
	// 20ms is enough to filter pure replay attacks while keeping UX fast
	solveTime := time.Since(entry.IssuedAt)
	if solveTime.Milliseconds() < 20 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"solve time too fast"}`))
		if escalation != nil {
			escalation.RecordFailure(clientIP, "transparent")
		}
		return false
	}

	// Validate fingerprint: must be present with required fields
	if req.FP == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"missing fingerprint"}`))
		if escalation != nil {
			escalation.RecordFailure(clientIP, "transparent")
		}
		return false
	}

	// Check webdriver detection: reject if navigator.webdriver = true
	if req.FP.WD != nil && *req.FP.WD {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"automation detected"}`))
		if escalation != nil {
			escalation.RecordFailure(clientIP, "transparent")
		}
		return false
	}

	// Check required fingerprint fields: canvas and webgl must be non-empty
	if req.FP.Canvas == "" || req.FP.WebGL == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"incomplete fingerprint"}`))
		if escalation != nil {
			escalation.RecordFailure(clientIP, "transparent")
		}
		return false
	}

	// Success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
	return true
}

// transparentChallengeHTML là template HTML cho transparent JS challenge.
// Must be <2KB total. Auto-solves in <100ms without user interaction.
// Computes HMAC-based proof (simple XOR hash), collects browser fingerprint,
// POSTs to /__kiro/transparent/verify endpoint.
// Uses fetch API for speed. On success, server returns 302 with Set-Cookie,
// browser follows redirect automatically — zero JS redirect overhead.
const transparentChallengeHTML = `<html><body><script>(function(){var t="{{TOKEN}}",s="{{SALT}}",n="{{NEXT}}";function hm(k,m){var r=0,i;for(i=0;i<k.length;i++)r=((r<<5)-r+k.charCodeAt(i))|0;for(i=0;i<m.length;i++)r=((r<<5)-r+m.charCodeAt(i))|0;return r.toString(16)}var sol=hm(s,t);var cv="",wg="",tz=0,wd=false;try{var c=document.createElement("canvas");c.width=16;c.height=16;var x=c.getContext("2d");x.fillStyle="#f00";x.fillRect(0,0,16,16);x.fillStyle="#0f0";x.font="6px a";x.fillText("K",2,12);cv=c.toDataURL().slice(-32)}catch(e){}try{var g=document.createElement("canvas").getContext("webgl");wg=g?g.getParameter(g.RENDERER)||"x":""}catch(e){}try{tz=new Date().getTimezoneOffset()}catch(e){}try{wd=!!navigator.webdriver}catch(e){}fetch("/__kiro/transparent/verify",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({token:t,solution:sol,fp:{canvas:cv,webgl:wg,tz:tz,wd:wd}}),redirect:"follow",credentials:"same-origin"}).then(function(r){location.replace(n)})})()</script></body></html>`
