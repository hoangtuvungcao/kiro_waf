package challenge

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// mockEscalator implements Escalator interface for testing.
type mockEscalator struct {
	failures []struct {
		ip            string
		challengeType string
	}
}

func (m *mockEscalator) RecordFailure(ip string, challengeType string) {
	m.failures = append(m.failures, struct {
		ip            string
		challengeType string
	}{ip, challengeType})
}

func TestServeTransparentPage_Headers(t *testing.T) {
	store := NewStore()
	req := httptest.NewRequest(http.MethodGet, "/some/page", nil)
	w := httptest.NewRecorder()

	ServeTransparentPage(w, req, store, 30*time.Second, "192.168.1.1")

	resp := w.Result()
	if resp.Header.Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("expected Content-Type text/html; charset=utf-8, got %s", resp.Header.Get("Content-Type"))
	}
	if resp.Header.Get("Cache-Control") != "no-store, no-cache, must-revalidate" {
		t.Errorf("expected Cache-Control no-store, no-cache, must-revalidate, got %s", resp.Header.Get("Cache-Control"))
	}
	if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("expected X-Content-Type-Options nosniff, got %s", resp.Header.Get("X-Content-Type-Options"))
	}
}

func TestServeTransparentPage_SizeUnder2KB(t *testing.T) {
	store := NewStore()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	ServeTransparentPage(w, req, store, 30*time.Second, "10.0.0.1")

	body := w.Body.Bytes()
	if len(body) >= 2048 {
		t.Errorf("transparent page size %d bytes exceeds 2KB limit", len(body))
	}
	t.Logf("Transparent page size: %d bytes (%.1f%% of 2KB budget)", len(body), float64(len(body))/2048*100)
}

func TestServeTransparentPage_ContainsInlineJS(t *testing.T) {
	store := NewStore()
	req := httptest.NewRequest(http.MethodGet, "/target", nil)
	w := httptest.NewRecorder()

	ServeTransparentPage(w, req, store, 30*time.Second, "10.0.0.1")

	body := w.Body.String()
	if !strings.Contains(body, "<script>") {
		t.Error("page must contain inline <script> tag")
	}
	if !strings.Contains(body, "/__kiro/transparent/verify") {
		t.Error("page must POST to /__kiro/transparent/verify")
	}
	if !strings.Contains(body, "webdriver") {
		t.Error("page must check navigator.webdriver")
	}
	if !strings.Contains(body, "canvas") {
		t.Error("page must collect canvas fingerprint")
	}
	if !strings.Contains(body, "webgl") || !strings.Contains(body, "RENDERER") {
		t.Error("page must collect WebGL renderer")
	}
	if !strings.Contains(body, "getTimezoneOffset") {
		t.Error("page must collect timezone offset")
	}
}

func TestServeTransparentPage_NoExternalResources(t *testing.T) {
	store := NewStore()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	ServeTransparentPage(w, req, store, 30*time.Second, "10.0.0.1")

	body := w.Body.String()
	if strings.Contains(body, "src=") && !strings.Contains(body, "src=\"data:") {
		// Check for external script/img src (but allow data: URIs)
		if strings.Contains(body, "http://") || strings.Contains(body, "https://") {
			t.Error("page must not reference external resources")
		}
	}
}

func TestVerifyTransparent_ValidSolution(t *testing.T) {
	store := NewStore()
	clientIP := "192.168.1.100"
	entry := store.IssueAt(clientIP, 0, 30*time.Second, time.Now().UTC().Add(-100*time.Millisecond))

	fp := browserFingerprint{
		Canvas: "abc123hash",
		WebGL:  "ANGLE (Intel HD Graphics)",
		TZ:     intPtr(-420),
		WD:     boolPtr(false),
	}
	payload := transparentVerifyRequest{
		Token:    entry.Token,
		Solution: "somesolution",
		FP:       &fp,
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/__kiro/transparent/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	result := VerifyTransparent(w, req, store, clientIP, nil)

	if !result {
		t.Error("expected VerifyTransparent to return true for valid solution")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", w.Code)
	}
}

func TestVerifyTransparent_InvalidToken(t *testing.T) {
	store := NewStore()
	clientIP := "192.168.1.100"
	esc := &mockEscalator{}

	fp := browserFingerprint{
		Canvas: "abc123hash",
		WebGL:  "ANGLE (Intel HD Graphics)",
		TZ:     intPtr(-420),
		WD:     boolPtr(false),
	}
	payload := transparentVerifyRequest{
		Token:    "nonexistent-token",
		Solution: "somesolution",
		FP:       &fp,
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/__kiro/transparent/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	result := VerifyTransparent(w, req, store, clientIP, esc)

	if result {
		t.Error("expected VerifyTransparent to return false for invalid token")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("expected HTTP 403, got %d", w.Code)
	}
	if len(esc.failures) != 1 {
		t.Errorf("expected 1 escalation failure recorded, got %d", len(esc.failures))
	}
}

func TestVerifyTransparent_IPMismatch(t *testing.T) {
	store := NewStore()
	clientIP := "192.168.1.100"
	entry := store.IssueAt(clientIP, 0, 30*time.Second, time.Now().UTC().Add(-100*time.Millisecond))
	esc := &mockEscalator{}

	fp := browserFingerprint{
		Canvas: "abc123hash",
		WebGL:  "ANGLE (Intel HD Graphics)",
		TZ:     intPtr(-420),
		WD:     boolPtr(false),
	}
	payload := transparentVerifyRequest{
		Token:    entry.Token,
		Solution: "somesolution",
		FP:       &fp,
	}
	body, _ := json.Marshal(payload)

	// Use different IP for verification
	req := httptest.NewRequest(http.MethodPost, "/__kiro/transparent/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	result := VerifyTransparent(w, req, store, "10.0.0.99", esc)

	if result {
		t.Error("expected VerifyTransparent to return false for IP mismatch")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("expected HTTP 403, got %d", w.Code)
	}
}

func TestVerifyTransparent_SingleUse(t *testing.T) {
	store := NewStore()
	clientIP := "192.168.1.100"
	entry := store.IssueAt(clientIP, 0, 30*time.Second, time.Now().UTC().Add(-100*time.Millisecond))

	fp := browserFingerprint{
		Canvas: "abc123hash",
		WebGL:  "ANGLE (Intel HD Graphics)",
		TZ:     intPtr(-420),
		WD:     boolPtr(false),
	}
	payload := transparentVerifyRequest{
		Token:    entry.Token,
		Solution: "somesolution",
		FP:       &fp,
	}
	body, _ := json.Marshal(payload)

	// First attempt: should succeed
	req1 := httptest.NewRequest(http.MethodPost, "/__kiro/transparent/verify", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	result1 := VerifyTransparent(w1, req1, store, clientIP, nil)
	if !result1 {
		t.Error("first attempt should succeed")
	}

	// Second attempt with same token: should fail (single-use)
	req2 := httptest.NewRequest(http.MethodPost, "/__kiro/transparent/verify", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	result2 := VerifyTransparent(w2, req2, store, clientIP, nil)
	if result2 {
		t.Error("second attempt should fail (token is single-use)")
	}
	if w2.Code != http.StatusForbidden {
		t.Errorf("expected HTTP 403 on second attempt, got %d", w2.Code)
	}
}

func TestVerifyTransparent_SolveTimeTooFast(t *testing.T) {
	store := NewStore()
	clientIP := "192.168.1.100"
	// Issue token just now (solve time will be < 20ms)
	entry := store.IssueAt(clientIP, 0, 30*time.Second, time.Now().UTC())
	esc := &mockEscalator{}

	fp := browserFingerprint{
		Canvas: "abc123hash",
		WebGL:  "ANGLE (Intel HD Graphics)",
		TZ:     intPtr(-420),
		WD:     boolPtr(false),
	}
	payload := transparentVerifyRequest{
		Token:    entry.Token,
		Solution: "somesolution",
		FP:       &fp,
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/__kiro/transparent/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	result := VerifyTransparent(w, req, store, clientIP, esc)

	if result {
		t.Error("expected VerifyTransparent to reject submissions faster than 20ms")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("expected HTTP 403, got %d", w.Code)
	}
	if len(esc.failures) != 1 {
		t.Errorf("expected 1 escalation failure, got %d", len(esc.failures))
	}
}

func TestVerifyTransparent_WebdriverDetected(t *testing.T) {
	store := NewStore()
	clientIP := "192.168.1.100"
	entry := store.IssueAt(clientIP, 0, 30*time.Second, time.Now().UTC().Add(-100*time.Millisecond))
	esc := &mockEscalator{}

	fp := browserFingerprint{
		Canvas: "abc123hash",
		WebGL:  "ANGLE (Intel HD Graphics)",
		TZ:     intPtr(-420),
		WD:     boolPtr(true), // webdriver detected!
	}
	payload := transparentVerifyRequest{
		Token:    entry.Token,
		Solution: "somesolution",
		FP:       &fp,
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/__kiro/transparent/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	result := VerifyTransparent(w, req, store, clientIP, esc)

	if result {
		t.Error("expected VerifyTransparent to reject webdriver=true")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("expected HTTP 403, got %d", w.Code)
	}
	if len(esc.failures) != 1 || esc.failures[0].challengeType != "transparent" {
		t.Error("expected escalation failure recorded for transparent challenge")
	}
}

func TestVerifyTransparent_MissingFingerprint(t *testing.T) {
	store := NewStore()
	clientIP := "192.168.1.100"
	entry := store.IssueAt(clientIP, 0, 30*time.Second, time.Now().UTC().Add(-100*time.Millisecond))
	esc := &mockEscalator{}

	payload := transparentVerifyRequest{
		Token:    entry.Token,
		Solution: "somesolution",
		FP:       nil, // no fingerprint
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/__kiro/transparent/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	result := VerifyTransparent(w, req, store, clientIP, esc)

	if result {
		t.Error("expected VerifyTransparent to reject missing fingerprint")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("expected HTTP 403, got %d", w.Code)
	}
}

func TestVerifyTransparent_IncompleteFingerprint_MissingCanvas(t *testing.T) {
	store := NewStore()
	clientIP := "192.168.1.100"
	entry := store.IssueAt(clientIP, 0, 30*time.Second, time.Now().UTC().Add(-100*time.Millisecond))
	esc := &mockEscalator{}

	fp := browserFingerprint{
		Canvas: "", // missing canvas
		WebGL:  "ANGLE (Intel HD Graphics)",
		TZ:     intPtr(-420),
		WD:     boolPtr(false),
	}
	payload := transparentVerifyRequest{
		Token:    entry.Token,
		Solution: "somesolution",
		FP:       &fp,
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/__kiro/transparent/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	result := VerifyTransparent(w, req, store, clientIP, esc)

	if result {
		t.Error("expected VerifyTransparent to reject incomplete fingerprint (missing canvas)")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("expected HTTP 403, got %d", w.Code)
	}
}

func TestVerifyTransparent_IncompleteFingerprint_MissingWebGL(t *testing.T) {
	store := NewStore()
	clientIP := "192.168.1.100"
	entry := store.IssueAt(clientIP, 0, 30*time.Second, time.Now().UTC().Add(-100*time.Millisecond))
	esc := &mockEscalator{}

	fp := browserFingerprint{
		Canvas: "abc123hash",
		WebGL:  "", // missing WebGL
		TZ:     intPtr(-420),
		WD:     boolPtr(false),
	}
	payload := transparentVerifyRequest{
		Token:    entry.Token,
		Solution: "somesolution",
		FP:       &fp,
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/__kiro/transparent/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	result := VerifyTransparent(w, req, store, clientIP, esc)

	if result {
		t.Error("expected VerifyTransparent to reject incomplete fingerprint (missing WebGL)")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("expected HTTP 403, got %d", w.Code)
	}
}

func TestVerifyTransparent_MethodNotAllowed(t *testing.T) {
	store := NewStore()
	req := httptest.NewRequest(http.MethodGet, "/__kiro/transparent/verify", nil)
	w := httptest.NewRecorder()

	result := VerifyTransparent(w, req, store, "192.168.1.1", nil)

	if result {
		t.Error("expected VerifyTransparent to reject non-POST methods")
	}
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected HTTP 405, got %d", w.Code)
	}
}

func TestVerifyTransparent_InvalidJSON(t *testing.T) {
	store := NewStore()
	req := httptest.NewRequest(http.MethodPost, "/__kiro/transparent/verify", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	result := VerifyTransparent(w, req, store, "192.168.1.1", nil)

	if result {
		t.Error("expected VerifyTransparent to reject invalid JSON")
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected HTTP 400, got %d", w.Code)
	}
}

func TestVerifyTransparent_ExpiredToken(t *testing.T) {
	store := NewStore()
	clientIP := "192.168.1.100"
	// Issue token that's already expired
	entry := store.IssueAt(clientIP, 0, 1*time.Millisecond, time.Now().UTC().Add(-1*time.Second))
	esc := &mockEscalator{}

	fp := browserFingerprint{
		Canvas: "abc123hash",
		WebGL:  "ANGLE (Intel HD Graphics)",
		TZ:     intPtr(-420),
		WD:     boolPtr(false),
	}
	payload := transparentVerifyRequest{
		Token:    entry.Token,
		Solution: "somesolution",
		FP:       &fp,
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/__kiro/transparent/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	result := VerifyTransparent(w, req, store, clientIP, esc)

	if result {
		t.Error("expected VerifyTransparent to reject expired token")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("expected HTTP 403, got %d", w.Code)
	}
}

// Helper functions
func intPtr(v int) *int {
	return &v
}

func boolPtr(v bool) *bool {
	return &v
}
