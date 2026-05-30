// Feature: waf-system-overhaul, Property 1: PoW Verification Correctness
// **Validates: Requirements 1.1**
//
// For any valid token, salt, và nonce sao cho SHA-256(token + ":" + salt + ":" + nonce)
// bắt đầu bằng đúng `difficulty` ký tự "0", hàm `validPoW` SHALL trả về true.
// Ngược lại, for any nonce mà hash không thỏa mãn điều kiện prefix, hàm SHALL trả về false.
package property

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"kiro_waf/internal/client/challenge"

	"pgregory.net/rapid"
)

// findValidNonce brute-forces a nonce that produces a hash with the required prefix.
// Returns the nonce string and true if found, or ("", false) if not found within maxIter.
func findValidNonce(token, salt string, difficulty int, maxIter int) (string, bool) {
	prefix := strings.Repeat("0", difficulty)
	for i := 0; i < maxIter; i++ {
		nonce := fmt.Sprintf("%d", i)
		input := token + ":" + salt + ":" + nonce
		sum := sha256.Sum256([]byte(input))
		h := hex.EncodeToString(sum[:])
		if strings.HasPrefix(h, prefix) {
			return nonce, true
		}
	}
	return "", false
}

// computeHash returns the hex-encoded SHA-256 hash of token:salt:nonce.
func computeHash(token, salt, nonce string) string {
	input := token + ":" + salt + ":" + nonce
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}

// TestPoW_PositiveCase verifies that for any random token, salt, and a computed valid nonce,
// ValidPoW returns true.
func TestPoW_PositiveCase(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random token and salt (printable ASCII, reasonable length)
		token := rapid.StringMatching(`[a-zA-Z0-9_\-]{8,64}`).Draw(t, "token")
		salt := rapid.StringMatching(`[a-zA-Z0-9_\-]{8,64}`).Draw(t, "salt")
		// Use difficulty 1-3 for test speed
		difficulty := rapid.IntRange(1, 3).Draw(t, "difficulty")

		// Find a valid nonce by brute force
		nonce, found := findValidNonce(token, salt, difficulty, 5000000)
		if !found {
			t.Skip("could not find valid nonce within iteration limit")
		}

		// Verify the hash actually has the correct prefix (sanity check)
		h := computeHash(token, salt, nonce)
		prefix := strings.Repeat("0", difficulty)
		if !strings.HasPrefix(h, prefix) {
			t.Fatalf("sanity check failed: hash %s does not start with %s", h, prefix)
		}

		// Property: ValidPoW MUST return true for a valid nonce
		if !challenge.ValidPoW(token, salt, nonce, difficulty) {
			t.Fatalf("ValidPoW returned false for valid solution: token=%q, salt=%q, nonce=%q, difficulty=%d, hash=%s",
				token, salt, nonce, difficulty, h)
		}
	})
}

// TestPoW_NegativeCase verifies that for any random token, salt, and random nonce
// where the hash does NOT satisfy the difficulty prefix, ValidPoW returns false.
func TestPoW_NegativeCase(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random token, salt, and nonce
		token := rapid.StringMatching(`[a-zA-Z0-9_\-]{8,64}`).Draw(t, "token")
		salt := rapid.StringMatching(`[a-zA-Z0-9_\-]{8,64}`).Draw(t, "salt")
		nonce := rapid.StringMatching(`[a-zA-Z0-9]{1,20}`).Draw(t, "nonce")
		// Use difficulty 2-3 to make it very unlikely a random nonce satisfies the prefix
		difficulty := rapid.IntRange(2, 3).Draw(t, "difficulty")

		// Compute the hash and check if it actually satisfies the prefix
		h := computeHash(token, salt, nonce)
		prefix := strings.Repeat("0", difficulty)

		if strings.HasPrefix(h, prefix) {
			// This random nonce happens to be valid — skip this iteration
			t.Skip("random nonce happened to produce valid hash, skipping")
		}

		// Property: ValidPoW MUST return false for an invalid nonce
		if challenge.ValidPoW(token, salt, nonce, difficulty) {
			t.Fatalf("ValidPoW returned true for invalid solution: token=%q, salt=%q, nonce=%q, difficulty=%d, hash=%s",
				token, salt, nonce, difficulty, h)
		}
	})
}
