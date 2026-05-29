package licenseverify

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVerifyLicenseAllowedMode(t *testing.T) {
	file, publicKey := signedTestLicense(t, testPayload())
	res, err := Verify(file, publicKey, Options{
		RequiredMode:           "full",
		RequiredFeatures:       []string{"waf", "bot_defense"},
		MachineFingerprintHash: testPayload().MachineBinding.FingerprintHash,
		Now:                    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("verify license: %v", err)
	}
	if !res.Valid {
		t.Fatal("expected valid license")
	}
	if res.LicenseID != "lic_test" {
		t.Fatalf("license id = %q, want lic_test", res.LicenseID)
	}
}

func TestVerifyRejectsInvalidSignature(t *testing.T) {
	file, publicKey := signedTestLicense(t, testPayload())
	file.Payload.Plan = "tampered"
	if _, err := Verify(file, publicKey, Options{
		RequiredMode: "full",
		Now:          time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}); err == nil {
		t.Fatal("expected invalid signature rejection")
	}
}

func TestVerifyRejectsMode(t *testing.T) {
	file, publicKey := signedTestLicense(t, testPayload())
	if _, err := Verify(file, publicKey, Options{
		RequiredMode: "server_only_missing",
		Now:          time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}); err == nil {
		t.Fatal("expected mode rejection")
	}
}

func TestVerifyRejectsMissingFeature(t *testing.T) {
	file, publicKey := signedTestLicense(t, testPayload())
	if _, err := Verify(file, publicKey, Options{
		RequiredMode:     "full",
		RequiredFeatures: []string{"runtime_security"},
		Now:              time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}); err == nil {
		t.Fatal("expected feature rejection")
	}
}

func TestVerifyRejectsMachineBinding(t *testing.T) {
	file, publicKey := signedTestLicense(t, testPayload())
	if _, err := Verify(file, publicKey, Options{
		RequiredMode:           "full",
		MachineFingerprintHash: FingerprintHash("different", "machine"),
		Now:                    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}); err == nil {
		t.Fatal("expected machine binding rejection")
	}
}

func TestVerifyAllowsGracePeriod(t *testing.T) {
	payload := testPayload()
	payload.ExpiresAt = "2026-01-01T00:00:00Z"
	payload.GraceDays = 7
	file, publicKey := signedTestLicense(t, payload)
	res, err := Verify(file, publicKey, Options{
		RequiredMode: "full",
		Now:          time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("expected grace period to pass: %v", err)
	}
	if !res.Expired {
		t.Fatal("expected expired flag inside grace period")
	}
}

func TestVerifyRejectsExpiredWhenGraceDisabled(t *testing.T) {
	payload := testPayload()
	payload.ExpiresAt = "2026-01-01T00:00:00Z"
	payload.GraceDays = 7
	file, publicKey := signedTestLicense(t, payload)
	if _, err := Verify(file, publicKey, Options{
		RequiredMode:       "full",
		DisableGracePeriod: true,
		Now:                time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC),
	}); err == nil {
		t.Fatal("expected expired license rejection when grace is disabled")
	}
}

func TestVerifyRejectsExpiredBeyondGrace(t *testing.T) {
	payload := testPayload()
	payload.ExpiresAt = "2026-01-01T00:00:00Z"
	payload.GraceDays = 7
	file, publicKey := signedTestLicense(t, payload)
	if _, err := Verify(file, publicKey, Options{
		RequiredMode: "full",
		Now:          time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC),
	}); err == nil {
		t.Fatal("expected expiry beyond grace rejection")
	}
}

func TestVerifyRejectsRevokedLicense(t *testing.T) {
	payload := testPayload()
	payload.LicenseID = "lic_revoked"
	file, publicKey, privateKey := signedTestLicenseWithKeys(t, payload)
	revocations, err := SignRevocationList(RevocationListPayload{
		GeneratedAt: "2026-05-28T00:00:00Z",
		Revoked: []RevokedLicense{{
			LicenseID: "lic_revoked",
			Reason:    "non-payment",
			RevokedAt: "2026-05-28T00:00:00Z",
		}},
	}, privateKey)
	if err != nil {
		t.Fatalf("sign revocations: %v", err)
	}
	dir := t.TempDir()
	revocationPath := filepath.Join(dir, "revocations.json")
	writeRevocationList(t, revocationPath, revocations)
	_, err = Verify(file, publicKey, Options{
		RevocationListPath: revocationPath,
		Now:                time.Date(2026, 5, 29, 0, 0, 0, 0, time.UTC),
	})
	if err == nil || !strings.Contains(err.Error(), "revoked") {
		t.Fatalf("error = %v, want revoked", err)
	}
}

func TestVerifyRevocationListRejectsBadSignature(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	file, err := SignRevocationList(RevocationListPayload{
		GeneratedAt: "2026-05-28T00:00:00Z",
		Revoked: []RevokedLicense{{
			LicenseID: "lic_revoked",
			RevokedAt: "2026-05-28T00:00:00Z",
		}},
	}, privateKey)
	if err != nil {
		t.Fatalf("sign revocations: %v", err)
	}
	file.Payload.Revoked[0].LicenseID = "lic_tampered"
	if _, err := VerifyRevocationList(file, publicKey); err == nil {
		t.Fatal("expected tampered revocation list to fail")
	}
}

func TestVerifyFileLoadsPEMPublicKey(t *testing.T) {
	file, publicKey, privateKey := signedTestLicenseWithKeys(t, testPayload())
	_ = privateKey
	dir := t.TempDir()
	licensePath := filepath.Join(dir, "license.json")
	keyPath := filepath.Join(dir, "provider-public-key.pem")
	writeJSON(t, licensePath, file)
	writePublicKeyPEM(t, keyPath, publicKey)

	if _, err := VerifyFile(licensePath, keyPath, Options{
		RequiredMode: "full",
		Now:          time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("verify file: %v", err)
	}
}

func TestFingerprintStable(t *testing.T) {
	a := Fingerprint("machine", "mac", "salt")
	b := Fingerprint("machine", "mac", "salt")
	if a != b || a == "" {
		t.Fatal("expected stable non-empty fingerprint")
	}
	if FingerprintHash("machine", "mac", "salt") != "sha256:"+a {
		t.Fatal("expected fingerprint hash to include sha256 prefix")
	}
}

func testPayload() Payload {
	fingerprintHash := FingerprintHash("machine-id", "primary-mac", "all-macs", "salt")
	return Payload{
		LicenseID:  "lic_test",
		CustomerID: "cus_test",
		ServerID:   "srv_test",
		Plan:       "professional",
		Modes:      []string{"server", "full"},
		Features:   []string{"xdp", "nftables", "waf", "bot_defense"},
		MachineBinding: MachineBinding{
			MachineIDHash:   HashValue("machine-id"),
			PrimaryMACHash:  HashValue("primary-mac"),
			FingerprintHash: fingerprintHash,
		},
		IssuedAt:            "2026-01-01T00:00:00Z",
		ExpiresAt:           "2027-01-01T00:00:00Z",
		GraceDays:           7,
		UpdateChannel:       "stable",
		PolicyBundleVersion: "2026.05.1",
	}
}

func signedTestLicense(t *testing.T, payload Payload) (File, ed25519.PublicKey) {
	file, publicKey, _ := signedTestLicenseWithKeys(t, payload)
	return file, publicKey
}

func signedTestLicenseWithKeys(t *testing.T, payload Payload) (File, ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	canonical, err := CanonicalPayload(payload)
	if err != nil {
		t.Fatalf("canonical payload: %v", err)
	}
	signature := ed25519.Sign(privateKey, canonical)
	return File{Payload: payload, Signature: EncodeSignature(signature)}, publicKey, privateKey
}

func writeJSON(t *testing.T, path string, value File) {
	t.Helper()
	raw, err := CanonicalLicenseFile(value)
	if err != nil {
		t.Fatalf("marshal license: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write license: %v", err)
	}
}

func writeRevocationList(t *testing.T, path string, value RevocationListFile) {
	t.Helper()
	raw, err := CanonicalRevocationListFile(value)
	if err != nil {
		t.Fatalf("marshal revocations: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write revocations: %v", err)
	}
}

func writePublicKeyPEM(t *testing.T, path string, publicKey ed25519.PublicKey) {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	block := &pem.Block{Type: "PUBLIC KEY", Bytes: der}
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o644); err != nil {
		t.Fatalf("write public key: %v", err)
	}
}
