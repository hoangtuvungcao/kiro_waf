package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"testing"
)

func TestExtractFingerprint_XTLSFingerprintHeader(t *testing.T) {
	extractor := NewTLSExtractor()

	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("X-TLS-Fingerprint", "abc123def456")

	got := extractor.ExtractFingerprint(req)
	if got != "abc123def456" {
		t.Errorf("expected 'abc123def456', got %q", got)
	}
}

func TestExtractFingerprint_CFJA3Header(t *testing.T) {
	extractor := NewTLSExtractor()

	req, _ := http.NewRequest("GET", "/", nil)
	ja3Value := "769,47-53-5-10-49161-49162-49171-49172-50-56-19-4,0-10-11,23-24-25,0"
	req.Header.Set("CF-JA3", ja3Value)

	got := extractor.ExtractFingerprint(req)

	// Should be SHA-256 hash of the JA3 value
	h := sha256.Sum256([]byte(ja3Value))
	expected := hex.EncodeToString(h[:])

	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestExtractFingerprint_XTLSFingerprintTakesPriority(t *testing.T) {
	extractor := NewTLSExtractor()

	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("X-TLS-Fingerprint", "priority-fingerprint")
	req.Header.Set("CF-JA3", "should-not-be-used")

	got := extractor.ExtractFingerprint(req)
	if got != "priority-fingerprint" {
		t.Errorf("expected 'priority-fingerprint', got %q", got)
	}
}

func TestExtractFingerprint_NoHeaders(t *testing.T) {
	extractor := NewTLSExtractor()

	req, _ := http.NewRequest("GET", "/", nil)

	got := extractor.ExtractFingerprint(req)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestExtractFingerprint_EmptyHeaders(t *testing.T) {
	extractor := NewTLSExtractor()

	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("X-TLS-Fingerprint", "")
	req.Header.Set("CF-JA3", "")

	got := extractor.ExtractFingerprint(req)
	if got != "" {
		t.Errorf("expected empty string for empty headers, got %q", got)
	}
}

func TestNewTLSExtractor(t *testing.T) {
	extractor := NewTLSExtractor()
	if extractor == nil {
		t.Fatal("expected non-nil extractor")
	}
}
