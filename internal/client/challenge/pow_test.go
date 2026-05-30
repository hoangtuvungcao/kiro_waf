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
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		if strings.Contains(line, "http://") || strings.Contains(line, "https://") {
			// Allow only internal references
			if !strings.Contains(line, "/__kiro/") {
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
