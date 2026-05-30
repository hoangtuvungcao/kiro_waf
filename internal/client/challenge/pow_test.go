package challenge

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidPoW_ValidSolution(t *testing.T) {
	token := "test-token-abc"
	salt := "test-salt-xyz"
	difficulty := 2

	// Find a valid nonce
	var nonce string
	prefix := strings.Repeat("0", difficulty)
	for i := 0; i < 1000000; i++ {
		n := fmt.Sprintf("%d", i)
		input := token + ":" + salt + ":" + n
		sum := sha256.Sum256([]byte(input))
		h := hex.EncodeToString(sum[:])
		if strings.HasPrefix(h, prefix) {
			nonce = n
			break
		}
	}
	if nonce == "" {
		t.Fatal("could not find valid nonce in 1M iterations")
	}

	if !ValidPoW(token, salt, nonce, difficulty) {
		t.Errorf("ValidPoW returned false for valid solution (nonce=%s)", nonce)
	}
}

func TestValidPoW_InvalidSolution(t *testing.T) {
	// This nonce is extremely unlikely to produce a hash starting with "0000"
	if ValidPoW("token", "salt", "definitely-not-valid-nonce-xyz", 4) {
		// Verify it actually doesn't match
		input := "token:salt:definitely-not-valid-nonce-xyz"
		sum := sha256.Sum256([]byte(input))
		h := hex.EncodeToString(sum[:])
		if !strings.HasPrefix(h, "0000") {
			t.Error("ValidPoW returned true for invalid solution")
		}
	}
}

func TestValidPoW_ZeroDifficulty(t *testing.T) {
	// difficulty <= 0 should always return true
	if !ValidPoW("any", "any", "any", 0) {
		t.Error("ValidPoW should return true for difficulty 0")
	}
	if !ValidPoW("any", "any", "any", -1) {
		t.Error("ValidPoW should return true for negative difficulty")
	}
}

func TestServeChallengePage_ContentType(t *testing.T) {
	store := NewStore()
	req := httptest.NewRequest(http.MethodGet, "/__kiro/challenge", nil)
	w := httptest.NewRecorder()

	ServeChallengePage(w, req, store, 4, 0, "192.168.1.1")

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("expected text/html content type, got %s", ct)
	}
}

func TestServeChallengePage_ContainsVietnameseText(t *testing.T) {
	store := NewStore()
	req := httptest.NewRequest(http.MethodGet, "/__kiro/challenge", nil)
	w := httptest.NewRecorder()

	ServeChallengePage(w, req, store, 4, 0, "10.0.0.1")

	body := w.Body.String()
	vietnameseTexts := []string{
		"Đang xác thực truy cập",
		"Hệ thống đang kiểm tra",
		"Đang tính toán",
		"Xác thực thất bại",
	}
	for _, text := range vietnameseTexts {
		if !strings.Contains(body, text) {
			t.Errorf("page missing Vietnamese text: %q", text)
		}
	}
}

func TestServeChallengePage_ContainsNoscriptFallback(t *testing.T) {
	store := NewStore()
	req := httptest.NewRequest(http.MethodGet, "/__kiro/challenge", nil)
	w := httptest.NewRecorder()

	ServeChallengePage(w, req, store, 4, 0, "10.0.0.1")

	body := w.Body.String()
	if !strings.Contains(body, "<noscript>") {
		t.Error("page missing noscript fallback")
	}
	if !strings.Contains(body, "JavaScript") {
		t.Error("noscript fallback should mention JavaScript")
	}
}

func TestServeChallengePage_NoExternalDependencies(t *testing.T) {
	store := NewStore()
	req := httptest.NewRequest(http.MethodGet, "/__kiro/challenge", nil)
	w := httptest.NewRecorder()

	ServeChallengePage(w, req, store, 4, 0, "10.0.0.1")

	body := w.Body.String()
	// Should not contain external URLs (http:// or https://) except internal /__kiro/ paths
	// and SVG namespace declarations (xmlns="http://www.w3.org/2000/svg")
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		if strings.Contains(line, "http://") || strings.Contains(line, "https://") {
			// Allow internal references and SVG namespace
			if !strings.Contains(line, "/__kiro/") && !strings.Contains(line, "xmlns") {
				t.Errorf("page contains external dependency: %s", strings.TrimSpace(line))
			}
		}
	}
}

func TestServeChallengePage_ResponsiveViewport(t *testing.T) {
	store := NewStore()
	req := httptest.NewRequest(http.MethodGet, "/__kiro/challenge", nil)
	w := httptest.NewRecorder()

	ServeChallengePage(w, req, store, 4, 0, "10.0.0.1")

	body := w.Body.String()
	if !strings.Contains(body, "viewport") {
		t.Error("page missing viewport meta tag for responsive design")
	}
}

func TestServeChallengePage_DarkTheme(t *testing.T) {
	store := NewStore()
	req := httptest.NewRequest(http.MethodGet, "/__kiro/challenge", nil)
	w := httptest.NewRecorder()

	ServeChallengePage(w, req, store, 4, 0, "10.0.0.1")

	body := w.Body.String()
	if !strings.Contains(body, "color-scheme: dark") && !strings.Contains(body, "color-scheme:dark") {
		t.Error("page missing dark color scheme")
	}
}

func TestServeChallengePage_KiroBranding(t *testing.T) {
	store := NewStore()
	req := httptest.NewRequest(http.MethodGet, "/__kiro/challenge", nil)
	w := httptest.NewRecorder()

	ServeChallengePage(w, req, store, 4, 0, "10.0.0.1")

	body := w.Body.String()
	if !strings.Contains(body, "Kiro") {
		t.Error("page missing Kiro branding")
	}
}

func TestVerifyChallenge_ValidSolution(t *testing.T) {
	store := NewStore()

	// Issue a challenge
	entry := store.Issue("192.168.1.1", 2, 90*1e9)

	// Find valid nonce
	prefix := strings.Repeat("0", 2)
	var nonce string
	for i := 0; i < 1000000; i++ {
		n := fmt.Sprintf("%d", i)
		input := entry.Token + ":" + entry.Salt + ":" + n
		sum := sha256.Sum256([]byte(input))
		h := hex.EncodeToString(sum[:])
		if strings.HasPrefix(h, prefix) {
			nonce = n
			break
		}
	}
	if nonce == "" {
		t.Fatal("could not find valid nonce")
	}

	body, _ := json.Marshal(map[string]string{"token": entry.Token, "nonce": nonce})
	req := httptest.NewRequest(http.MethodPost, "/__kiro/challenge/verify", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	ok := VerifyChallenge(w, req, store, "192.168.1.1")
	if !ok {
		t.Errorf("VerifyChallenge returned false for valid solution, status=%d, body=%s", w.Code, w.Body.String())
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestVerifyChallenge_InvalidNonce(t *testing.T) {
	store := NewStore()
	entry := store.Issue("192.168.1.1", 4, 90*1e9)

	body, _ := json.Marshal(map[string]string{"token": entry.Token, "nonce": "invalid"})
	req := httptest.NewRequest(http.MethodPost, "/__kiro/challenge/verify", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	ok := VerifyChallenge(w, req, store, "192.168.1.1")
	if ok {
		t.Error("VerifyChallenge should return false for invalid nonce")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestVerifyChallenge_WrongIP(t *testing.T) {
	store := NewStore()
	entry := store.Issue("192.168.1.1", 1, 90*1e9)

	// Find valid nonce
	var nonce string
	for i := 0; i < 1000000; i++ {
		n := fmt.Sprintf("%d", i)
		input := entry.Token + ":" + entry.Salt + ":" + n
		sum := sha256.Sum256([]byte(input))
		h := hex.EncodeToString(sum[:])
		if strings.HasPrefix(h, "0") {
			nonce = n
			break
		}
	}

	body, _ := json.Marshal(map[string]string{"token": entry.Token, "nonce": nonce})
	req := httptest.NewRequest(http.MethodPost, "/__kiro/challenge/verify", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Different IP than what was issued
	ok := VerifyChallenge(w, req, store, "10.0.0.99")
	if ok {
		t.Error("VerifyChallenge should reject when client IP doesn't match")
	}
}

func TestVerifyChallenge_MethodNotAllowed(t *testing.T) {
	store := NewStore()
	req := httptest.NewRequest(http.MethodGet, "/__kiro/challenge/verify", nil)
	w := httptest.NewRecorder()

	ok := VerifyChallenge(w, req, store, "192.168.1.1")
	if ok {
		t.Error("VerifyChallenge should reject GET method")
	}
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestStore_IssueTake(t *testing.T) {
	store := NewStore()
	entry := store.Issue("1.2.3.4", 3, 60*1e9)

	if entry.Token == "" {
		t.Error("token should not be empty")
	}
	if entry.Salt == "" {
		t.Error("salt should not be empty")
	}
	if entry.ClientIP != "1.2.3.4" {
		t.Errorf("expected IP 1.2.3.4, got %s", entry.ClientIP)
	}

	// Take should succeed with correct IP
	taken, ok := store.Take(entry.Token, "1.2.3.4")
	if !ok {
		t.Error("Take should succeed for valid token and IP")
	}
	if taken.Salt != entry.Salt {
		t.Error("taken entry should have same salt")
	}

	// Take again should fail (already consumed)
	_, ok = store.Take(entry.Token, "1.2.3.4")
	if ok {
		t.Error("Take should fail for already consumed token")
	}
}

func TestStore_TakeWrongIP(t *testing.T) {
	store := NewStore()
	entry := store.Issue("1.2.3.4", 3, 60*1e9)

	_, ok := store.Take(entry.Token, "5.6.7.8")
	if ok {
		t.Error("Take should fail for wrong IP")
	}
}

func TestHoldPageResponseSizeUnder10KB(t *testing.T) {
	store := NewStore()

	// Use max-length NEXT URL: "/" + 2047 chars = 2048 chars total
	maxNext := "/" + strings.Repeat("x", 2047)
	req := httptest.NewRequest(http.MethodGet, maxNext, nil)
	w := httptest.NewRecorder()

	// holdSeconds = 999 (3 chars, max length)
	ServeHoldPage(w, req, store, 999, 0, "192.168.1.1")

	resp := w.Result()
	bodyBytes := w.Body.Bytes()
	bodyLen := len(bodyBytes)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if bodyLen >= 10240 {
		t.Errorf("Hold page response size %d bytes exceeds 10KB limit (10240 bytes)", bodyLen)
	}
	t.Logf("Hold page response size: %d bytes (%.1f%% of 10KB budget)", bodyLen, float64(bodyLen)/10240*100)
}

func TestPoWPageResponseSizeUnder10KB(t *testing.T) {
	store := NewStore()

	// Use max-length NEXT URL: "/" + 2047 chars = 2048 chars total
	maxNext := "/" + strings.Repeat("x", 2047)
	req := httptest.NewRequest(http.MethodGet, maxNext, nil)
	w := httptest.NewRecorder()

	// difficulty = 99 (2 chars, max length)
	ServeChallengePage(w, req, store, 99, 0, "192.168.1.1")

	resp := w.Result()
	bodyBytes := w.Body.Bytes()
	bodyLen := len(bodyBytes)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if bodyLen >= 10240 {
		t.Errorf("PoW page response size %d bytes exceeds 10KB limit (10240 bytes)", bodyLen)
	}
	t.Logf("PoW page response size: %d bytes (%.1f%% of 10KB budget)", bodyLen, float64(bodyLen)/10240*100)
}

func TestHoldPage_NoExternalDependencies(t *testing.T) {
	store := NewStore()
	req := httptest.NewRequest(http.MethodGet, "/__kiro/hold", nil)
	w := httptest.NewRecorder()

	ServeHoldPage(w, req, store, 2, 0, "10.0.0.1")

	body := w.Body.String()
	bodyLower := strings.ToLower(body)

	// Requirement 5.4: No <link rel="stylesheet"> elements referencing external URLs
	if strings.Contains(bodyLower, `<link rel="stylesheet"`) || strings.Contains(bodyLower, `<link rel='stylesheet'`) {
		t.Error("Hold page contains <link rel=\"stylesheet\"> — external stylesheet not allowed")
	}

	// Requirement 5.5: No <script src="..."> elements referencing external URLs
	if strings.Contains(bodyLower, `<script src=`) || strings.Contains(bodyLower, `<script src =`) {
		t.Error("Hold page contains <script src=...> — external script not allowed")
	}

	// Requirement 5.7: No CSS @import rules
	if strings.Contains(body, "@import") {
		t.Error("Hold page contains @import — external CSS import not allowed")
	}

	// Requirement 5.8: No <img>, <iframe>, <object>, <embed>, <audio>, <video> with external src
	externalMediaTags := []string{"<img", "<iframe", "<object", "<embed", "<audio", "<video"}
	for _, tag := range externalMediaTags {
		if strings.Contains(bodyLower, tag) {
			t.Errorf("Hold page contains %s element — external media elements not allowed", tag)
		}
	}

	// Requirement 5.6: No external fonts (no font-face with external url, no external font imports)
	if strings.Contains(bodyLower, "@font-face") {
		t.Error("Hold page contains @font-face — external fonts not allowed")
	}

	// Requirement 5.7: No url() values referencing external URLs in CSS
	// Check for url() with http:// or https:// (external resources)
	// Allow data: URIs and internal references
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		// Check for external url() in CSS
		lower := strings.ToLower(line)
		if strings.Contains(lower, "url(") {
			// Allow data: URIs (e.g., url("data:image/svg+xml;base64,..."))
			// Allow url(#id) references (SVG internal references)
			// Disallow url("http://...") or url("https://...")
			if strings.Contains(lower, `url("http`) || strings.Contains(lower, `url('http`) || strings.Contains(lower, `url(http`) {
				t.Errorf("Hold page line %d contains url() with external URL: %s", i+1, strings.TrimSpace(line))
			}
		}
	}

	// Requirements 5.4, 5.5, 5.7: No external URLs (http:// or https://) except:
	// - SVG namespace declarations (xmlns="http://www.w3.org/2000/svg")
	// - Internal /__kiro/ paths
	for i, line := range lines {
		if strings.Contains(line, "http://") || strings.Contains(line, "https://") {
			// Allow SVG namespace URIs
			if strings.Contains(line, "xmlns") {
				continue
			}
			// Allow internal /__kiro/ paths
			if strings.Contains(line, "/__kiro/") {
				continue
			}
			t.Errorf("Hold page line %d contains external URL: %s", i+1, strings.TrimSpace(line))
		}
	}

	// Requirement 5.1: All CSS within inline <style> tag (verify style tag exists)
	if !strings.Contains(body, "<style>") {
		t.Error("Hold page missing inline <style> element")
	}

	// Requirement 5.2: All JS within inline <script> tag (verify script tag exists without src)
	if !strings.Contains(body, "<script>") {
		t.Error("Hold page missing inline <script> element")
	}

	// Requirement 5.3: Only inline SVG or base64 data URIs for images
	// Already covered by the checks above (no <img> tags, no external url())
}

func TestHoldPage_InlineSVGLogo(t *testing.T) {
	store := NewStore()
	req := httptest.NewRequest(http.MethodGet, "/__kiro/hold", nil)
	w := httptest.NewRecorder()

	ServeHoldPage(w, req, store, 2, 0, "10.0.0.1")

	body := w.Body.String()
	if !strings.Contains(body, "<svg") {
		t.Error("Hold page missing inline <svg element")
	}
	if !strings.Contains(body, `role="img"`) {
		t.Error("Hold page SVG missing role=\"img\" attribute")
	}
	if !strings.Contains(body, `aria-label`) {
		t.Error("Hold page SVG missing aria-label attribute")
	}
}

func TestPoWPage_InlineSVGLogo(t *testing.T) {
	store := NewStore()
	req := httptest.NewRequest(http.MethodGet, "/__kiro/challenge", nil)
	w := httptest.NewRecorder()

	ServeChallengePage(w, req, store, 4, 0, "10.0.0.1")

	body := w.Body.String()
	if !strings.Contains(body, "<svg") {
		t.Error("PoW page missing inline <svg element")
	}
	if !strings.Contains(body, `role="img"`) {
		t.Error("PoW page SVG missing role=\"img\" attribute")
	}
	if !strings.Contains(body, `aria-label`) {
		t.Error("PoW page SVG missing aria-label attribute")
	}
}

func TestHoldPage_CSSCustomProperties(t *testing.T) {
	store := NewStore()
	req := httptest.NewRequest(http.MethodGet, "/__kiro/hold", nil)
	w := httptest.NewRecorder()

	ServeHoldPage(w, req, store, 2, 0, "10.0.0.1")

	body := w.Body.String()

	// Verify :root contains all required CSS custom properties
	requiredProps := []string{
		"--kiro-primary",
		"--kiro-accent",
		"--kiro-background",
		"--kiro-surface",
		"--kiro-text-primary",
		"--kiro-text-secondary",
		"--kiro-border",
		"--kiro-success",
		"--kiro-danger",
	}

	// Verify :root selector exists
	if !strings.Contains(body, ":root") {
		t.Fatal("Hold page missing :root CSS selector")
	}

	for _, prop := range requiredProps {
		if !strings.Contains(body, prop) {
			t.Errorf("Hold page :root missing CSS custom property %q", prop)
		}
	}
}

func TestPoWPage_CSSCustomProperties(t *testing.T) {
	store := NewStore()
	req := httptest.NewRequest(http.MethodGet, "/__kiro/challenge", nil)
	w := httptest.NewRecorder()

	ServeChallengePage(w, req, store, 4, 0, "10.0.0.1")

	body := w.Body.String()

	// Verify :root contains all required CSS custom properties
	requiredProps := []string{
		"--kiro-primary",
		"--kiro-accent",
		"--kiro-background",
		"--kiro-surface",
		"--kiro-text-primary",
		"--kiro-text-secondary",
		"--kiro-border",
		"--kiro-success",
		"--kiro-danger",
	}

	// Verify :root selector exists
	if !strings.Contains(body, ":root") {
		t.Fatal("PoW page missing :root CSS selector")
	}

	for _, prop := range requiredProps {
		if !strings.Contains(body, prop) {
			t.Errorf("PoW page :root missing CSS custom property %q", prop)
		}
	}
}

func TestHoldPage_ResponsiveBreakpoints(t *testing.T) {
	store := NewStore()
	req := httptest.NewRequest(http.MethodGet, "/__kiro/hold", nil)
	w := httptest.NewRecorder()

	ServeHoldPage(w, req, store, 2, 0, "10.0.0.1")

	body := w.Body.String()

	// Verify media query for small screens (360px)
	if !strings.Contains(body, "@media (max-width: 360px)") && !strings.Contains(body, "@media(max-width:360px)") && !strings.Contains(body, "@media (max-width:360px)") {
		t.Error("Hold page missing responsive breakpoint @media (max-width: 360px)")
	}

	// Verify media query for large screens (1920px)
	if !strings.Contains(body, "@media (min-width: 1920px)") && !strings.Contains(body, "@media(min-width:1920px)") && !strings.Contains(body, "@media (min-width:1920px)") {
		t.Error("Hold page missing responsive breakpoint @media (min-width: 1920px)")
	}
}

func TestPoWPage_ResponsiveBreakpoints(t *testing.T) {
	store := NewStore()
	req := httptest.NewRequest(http.MethodGet, "/__kiro/challenge", nil)
	w := httptest.NewRecorder()

	ServeChallengePage(w, req, store, 4, 0, "10.0.0.1")

	body := w.Body.String()

	// Verify media query for small screens (360px)
	if !strings.Contains(body, "@media (max-width: 360px)") && !strings.Contains(body, "@media(max-width:360px)") && !strings.Contains(body, "@media (max-width:360px)") {
		t.Error("PoW page missing responsive breakpoint @media (max-width: 360px)")
	}

	// Verify media query for large screens (1920px)
	if !strings.Contains(body, "@media (min-width: 1920px)") && !strings.Contains(body, "@media(min-width:1920px)") && !strings.Contains(body, "@media (min-width:1920px)") {
		t.Error("PoW page missing responsive breakpoint @media (min-width: 1920px)")
	}
}
