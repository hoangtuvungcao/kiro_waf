package licenseverify

import (
	"testing"
	"time"
)

func TestCheckLicenseAllowedMode(t *testing.T) {
	file := File{
		Payload: Payload{
			LicenseID: "lic_test",
			Modes:     []string{"server", "full"},
			ExpiresAt: "2027-01-01T00:00:00Z",
			GraceDays: 7,
		},
		Signature: "ed25519:test",
	}
	res, err := Check(file, "full", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("check license: %v", err)
	}
	if !res.Valid {
		t.Fatal("expected valid license")
	}
}

func TestCheckLicenseRejectsMode(t *testing.T) {
	file := File{
		Payload: Payload{
			LicenseID: "lic_test",
			Modes:     []string{"server"},
			ExpiresAt: "2027-01-01T00:00:00Z",
		},
		Signature: "ed25519:test",
	}
	if _, err := Check(file, "full", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("expected mode rejection")
	}
}

func TestFingerprintStable(t *testing.T) {
	a := Fingerprint("machine", "mac", "salt")
	b := Fingerprint("machine", "mac", "salt")
	if a != b || a == "" {
		t.Fatal("expected stable non-empty fingerprint")
	}
}
