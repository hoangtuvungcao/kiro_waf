package challenge

import (
	"testing"
	"time"
)

func TestAntiReplayValidator_IssueAndConsume(t *testing.T) {
	v := NewAntiReplayValidator()

	token, issuedAt := v.IssueToken("192.168.1.1", 30*time.Second)
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if issuedAt.IsZero() {
		t.Fatal("expected non-zero issuedAt")
	}

	// Consume with correct IP should succeed.
	got, err := v.ConsumeToken(token, "192.168.1.1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != issuedAt {
		t.Fatalf("expected issuedAt %v, got %v", issuedAt, got)
	}
}

func TestAntiReplayValidator_SingleUse(t *testing.T) {
	v := NewAntiReplayValidator()

	token, _ := v.IssueToken("10.0.0.1", 30*time.Second)

	// First consume succeeds.
	_, err := v.ConsumeToken(token, "10.0.0.1")
	if err != nil {
		t.Fatalf("first consume should succeed, got %v", err)
	}

	// Second consume must fail with ErrTokenNotFound.
	_, err = v.ConsumeToken(token, "10.0.0.1")
	if err != ErrTokenNotFound {
		t.Fatalf("expected ErrTokenNotFound on second consume, got %v", err)
	}
}

func TestAntiReplayValidator_SingleUse_DeletesOnFailure(t *testing.T) {
	v := NewAntiReplayValidator()

	token, _ := v.IssueToken("10.0.0.1", 30*time.Second)

	// Consume with wrong IP — should fail but still delete the token.
	_, err := v.ConsumeToken(token, "10.0.0.2")
	if err != ErrIPMismatch {
		t.Fatalf("expected ErrIPMismatch, got %v", err)
	}

	// Token should be gone now.
	if v.Has(token) {
		t.Fatal("token should be deleted after failed consume")
	}

	// Second attempt should get ErrTokenNotFound.
	_, err = v.ConsumeToken(token, "10.0.0.1")
	if err != ErrTokenNotFound {
		t.Fatalf("expected ErrTokenNotFound after failed consume, got %v", err)
	}
}

func TestAntiReplayValidator_IPMismatch(t *testing.T) {
	v := NewAntiReplayValidator()

	token, _ := v.IssueToken("192.168.1.1", 30*time.Second)

	_, err := v.ConsumeToken(token, "192.168.1.2")
	if err != ErrIPMismatch {
		t.Fatalf("expected ErrIPMismatch, got %v", err)
	}
}

func TestAntiReplayValidator_Expired(t *testing.T) {
	v := NewAntiReplayValidator()

	// Issue with a very short TTL and use ConsumeTokenAt to simulate time passing.
	issuedAt := time.Now().UTC().Add(-60 * time.Second)
	token := v.IssueTokenAt("10.0.0.1", 30*time.Second, issuedAt)

	// Token expired 30 seconds ago.
	_, err := v.ConsumeTokenAt(token, "10.0.0.1", time.Now().UTC())
	if err != ErrTokenExpired {
		t.Fatalf("expected ErrTokenExpired, got %v", err)
	}
}

func TestAntiReplayValidator_NotFound(t *testing.T) {
	v := NewAntiReplayValidator()

	_, err := v.ConsumeToken("nonexistent-token", "10.0.0.1")
	if err != ErrTokenNotFound {
		t.Fatalf("expected ErrTokenNotFound, got %v", err)
	}
}

func TestAntiReplayValidator_Cleanup(t *testing.T) {
	v := NewAntiReplayValidator()

	// Issue a token that's already expired.
	issuedAt := time.Now().UTC().Add(-60 * time.Second)
	_ = v.IssueTokenAt("10.0.0.1", 30*time.Second, issuedAt)

	// Issue a token that's still valid.
	v.IssueToken("10.0.0.2", 30*time.Second)

	if v.Len() != 2 {
		t.Fatalf("expected 2 tokens, got %d", v.Len())
	}

	v.Cleanup()

	if v.Len() != 1 {
		t.Fatalf("expected 1 token after cleanup, got %d", v.Len())
	}
}

func TestAntiReplayValidator_IssuedAtReturnedOnIPMismatch(t *testing.T) {
	v := NewAntiReplayValidator()

	issuedAt := time.Now().UTC().Add(-5 * time.Second)
	token := v.IssueTokenAt("10.0.0.1", 30*time.Second, issuedAt)

	// Even on IP mismatch, issuedAt should be returned.
	got, err := v.ConsumeTokenAt(token, "10.0.0.2", time.Now().UTC())
	if err != ErrIPMismatch {
		t.Fatalf("expected ErrIPMismatch, got %v", err)
	}
	if !got.Equal(issuedAt) {
		t.Fatalf("expected issuedAt %v, got %v", issuedAt, got)
	}
}

func TestAntiReplayValidator_UniqueTokens(t *testing.T) {
	v := NewAntiReplayValidator()

	tokens := make(map[string]bool)
	for i := 0; i < 100; i++ {
		token, _ := v.IssueToken("10.0.0.1", 30*time.Second)
		if tokens[token] {
			t.Fatalf("duplicate token generated at iteration %d", i)
		}
		tokens[token] = true
	}
}
