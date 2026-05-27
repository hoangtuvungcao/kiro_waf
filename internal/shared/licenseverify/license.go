package licenseverify

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"
)

type File struct {
	Payload   Payload `json:"payload"`
	Signature string  `json:"signature"`
}

type Payload struct {
	LicenseID  string   `json:"license_id"`
	CustomerID string   `json:"customer_id"`
	ServerID   string   `json:"server_id"`
	Plan       string   `json:"plan"`
	Modes      []string `json:"allowed_modes"`
	Features   []string `json:"features"`
	ExpiresAt  string   `json:"expires_at"`
	GraceDays  int      `json:"grace_days"`
}

type Result struct {
	LicenseID string
	Valid     bool
	Expired   bool
}

func CheckFile(path string, requiredMode string) (Result, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Result{}, err
	}
	var file File
	if err := json.Unmarshal(raw, &file); err != nil {
		return Result{}, err
	}
	return Check(file, requiredMode, time.Now().UTC())
}

func Check(file File, requiredMode string, now time.Time) (Result, error) {
	if strings.TrimSpace(file.Payload.LicenseID) == "" {
		return Result{}, errors.New("license_id is required")
	}
	if strings.TrimSpace(file.Signature) == "" {
		return Result{}, errors.New("signature is required")
	}
	if !has(file.Payload.Modes, requiredMode) {
		return Result{}, errors.New("license does not allow requested mode")
	}
	exp, err := time.Parse(time.RFC3339, file.Payload.ExpiresAt)
	if err != nil {
		return Result{}, err
	}
	expired := now.After(exp.AddDate(0, 0, file.Payload.GraceDays))
	if expired {
		return Result{}, errors.New("license expired beyond grace period")
	}
	return Result{LicenseID: file.Payload.LicenseID, Valid: true, Expired: now.After(exp)}, nil
}

func Fingerprint(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func has(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
