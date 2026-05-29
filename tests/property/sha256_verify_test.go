// Feature: waf-system-overhaul, Property 8: SHA-256 Update Verification
// **Validates: Requirements 5.3, 5.4**
//
// For any byte array (artifact content), việc tính SHA-256 hash rồi so sánh với giá trị
// expected SHALL chấp nhận khi hash khớp chính xác và SHALL từ chối khi có bất kỳ sự
// khác biệt nào (kể cả 1 bit).
package property

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"pgregory.net/rapid"
)

// verifySHA256 computes the SHA-256 hash of content and compares it to the expected hex string.
// Returns true if and only if the computed hash matches the expected hash exactly.
func verifySHA256(content []byte, expectedHex string) bool {
	sum := sha256.Sum256(content)
	computed := hex.EncodeToString(sum[:])
	return computed == expectedHex
}

// TestSHA256_PositiveMatch verifies that for any random byte array, computing its SHA-256
// hash and comparing with itself always results in a match.
func TestSHA256_PositiveMatch(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random byte array of varying length (0 to 4096 bytes)
		content := rapid.SliceOfN(rapid.Byte(), 0, 4096).Draw(t, "content")

		// Compute SHA-256 hash
		sum := sha256.Sum256(content)
		expectedHex := hex.EncodeToString(sum[:])

		// Property: comparing hash with itself SHALL always match
		if !verifySHA256(content, expectedHex) {
			t.Fatalf("SHA-256 verification failed for matching hash: content_len=%d, hash=%s",
				len(content), expectedHex)
		}
	})
}

// TestSHA256_NegativeFlipBit verifies that for any random byte array, computing its SHA-256
// hash and then flipping 1 bit in the hash results in a mismatch.
func TestSHA256_NegativeFlipBit(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random byte array of varying length (0 to 4096 bytes)
		content := rapid.SliceOfN(rapid.Byte(), 0, 4096).Draw(t, "content")

		// Compute SHA-256 hash
		sum := sha256.Sum256(content)
		hashBytes := sum[:]

		// Pick a random byte index and bit position to flip
		byteIdx := rapid.IntRange(0, len(hashBytes)-1).Draw(t, "byteIdx")
		bitIdx := rapid.IntRange(0, 7).Draw(t, "bitIdx")

		// Create a modified hash with 1 bit flipped
		modifiedHash := make([]byte, len(hashBytes))
		copy(modifiedHash, hashBytes)
		modifiedHash[byteIdx] ^= 1 << uint(bitIdx)

		modifiedHex := hex.EncodeToString(modifiedHash)

		// Property: comparing with a 1-bit-flipped hash SHALL always mismatch
		if verifySHA256(content, modifiedHex) {
			originalHex := hex.EncodeToString(hashBytes)
			t.Fatalf("SHA-256 verification should have failed for 1-bit-flipped hash: "+
				"content_len=%d, original=%s, modified=%s, byteIdx=%d, bitIdx=%d",
				len(content), originalHex, modifiedHex, byteIdx, bitIdx)
		}
	})
}

// TestSHA256_NegativeModifyContent verifies that for any random byte array, computing its
// SHA-256 hash and then modifying 1 byte of the content results in a different hash.
func TestSHA256_NegativeModifyContent(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random byte array with at least 1 byte (1 to 4096 bytes)
		content := rapid.SliceOfN(rapid.Byte(), 1, 4096).Draw(t, "content")

		// Compute original SHA-256 hash
		originalSum := sha256.Sum256(content)
		originalHex := hex.EncodeToString(originalSum[:])

		// Pick a random byte index to modify
		modIdx := rapid.IntRange(0, len(content)-1).Draw(t, "modIdx")

		// Generate a different byte value for that position
		originalByte := content[modIdx]
		// Generate a new byte that is different from the original
		newByte := rapid.Byte().Draw(t, "newByte")
		if newByte == originalByte {
			// Ensure the byte is actually different by XOR with non-zero value
			newByte = originalByte ^ 0xFF
		}

		// Create modified content
		modifiedContent := make([]byte, len(content))
		copy(modifiedContent, content)
		modifiedContent[modIdx] = newByte

		// Compute hash of modified content
		modifiedSum := sha256.Sum256(modifiedContent)
		modifiedHex := hex.EncodeToString(modifiedSum[:])

		// Property: modifying 1 byte of content SHALL produce a different hash
		if modifiedHex == originalHex {
			t.Fatalf("SHA-256 hash did not change after modifying content: "+
				"content_len=%d, modIdx=%d, originalByte=0x%02x, newByte=0x%02x, hash=%s",
				len(content), modIdx, originalByte, newByte, originalHex)
		}

		// Additionally verify: the original hash no longer matches the modified content
		if verifySHA256(modifiedContent, originalHex) {
			t.Fatalf("SHA-256 verification should have failed for modified content: "+
				"content_len=%d, modIdx=%d, originalHash=%s, modifiedHash=%s",
				len(content), modIdx, originalHex, modifiedHex)
		}
	})
}
