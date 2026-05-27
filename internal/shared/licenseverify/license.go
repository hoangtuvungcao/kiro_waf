package licenseverify

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const signaturePrefix = "ed25519:"

type File struct {
	Payload   Payload `json:"payload"`
	Signature string  `json:"signature"`
}

type Payload struct {
	LicenseID           string         `json:"license_id"`
	CustomerID          string         `json:"customer_id"`
	ServerID            string         `json:"server_id"`
	Plan                string         `json:"plan"`
	Modes               []string       `json:"allowed_modes"`
	Features            []string       `json:"features"`
	MachineBinding      MachineBinding `json:"machine_binding"`
	IssuedAt            string         `json:"issued_at"`
	ExpiresAt           string         `json:"expires_at"`
	GraceDays           int            `json:"grace_days"`
	UpdateChannel       string         `json:"update_channel"`
	PolicyBundleVersion string         `json:"policy_bundle_version"`
}

type MachineBinding struct {
	MachineIDHash   string `json:"machine_id_hash"`
	PrimaryMACHash  string `json:"primary_mac_hash"`
	FingerprintHash string `json:"fingerprint_hash"`
}

type Options struct {
	RequiredMode           string
	RequiredFeatures       []string
	MachineFingerprintHash string
	DisableGracePeriod     bool
	Now                    time.Time
}

type Result struct {
	LicenseID string
	ServerID  string
	Plan      string
	Valid     bool
	Expired   bool
	Features  []string
}

func VerifyFile(licensePath string, publicKeyPath string, opts Options) (Result, error) {
	raw, err := os.ReadFile(licensePath)
	if err != nil {
		return Result{}, err
	}
	var file File
	if err := json.Unmarshal(raw, &file); err != nil {
		return Result{}, err
	}
	publicKey, err := LoadPublicKeyFile(publicKeyPath)
	if err != nil {
		return Result{}, err
	}
	return Verify(file, publicKey, opts)
}

func LoadPublicKeyFile(path string) (ed25519.PublicKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParsePublicKey(raw)
}

func ParsePublicKey(raw []byte) (ed25519.PublicKey, error) {
	trimmed := strings.TrimSpace(string(raw))
	if strings.HasPrefix(trimmed, signaturePrefix) {
		encoded := strings.TrimPrefix(trimmed, signaturePrefix)
		key, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, err
		}
		if len(key) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("ed25519 public key must be %d bytes", ed25519.PublicKeySize)
		}
		return ed25519.PublicKey(key), nil
	}

	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, errors.New("public key must be PEM or ed25519:<base64>")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("public key is not Ed25519")
	}
	return key, nil
}

func Verify(file File, publicKey ed25519.PublicKey, opts Options) (Result, error) {
	if opts.Now.IsZero() {
		opts.Now = time.Now().UTC()
	}
	if strings.TrimSpace(file.Payload.LicenseID) == "" {
		return Result{}, errors.New("license_id is required")
	}
	if strings.TrimSpace(file.Payload.ServerID) == "" {
		return Result{}, errors.New("server_id is required")
	}
	if strings.TrimSpace(file.Signature) == "" {
		return Result{}, errors.New("signature is required")
	}
	if len(publicKey) != ed25519.PublicKeySize {
		return Result{}, fmt.Errorf("ed25519 public key must be %d bytes", ed25519.PublicKeySize)
	}
	if err := verifySignature(file, publicKey); err != nil {
		return Result{}, err
	}
	if opts.RequiredMode != "" && !has(file.Payload.Modes, opts.RequiredMode) {
		return Result{}, errors.New("license does not allow requested mode")
	}
	for _, feature := range opts.RequiredFeatures {
		if !has(file.Payload.Features, feature) {
			return Result{}, fmt.Errorf("license does not allow required feature %q", feature)
		}
	}
	if err := verifyMachineBinding(file.Payload.MachineBinding, opts.MachineFingerprintHash); err != nil {
		return Result{}, err
	}

	exp, err := time.Parse(time.RFC3339, file.Payload.ExpiresAt)
	if err != nil {
		return Result{}, fmt.Errorf("invalid expires_at: %w", err)
	}
	if file.Payload.GraceDays < 0 {
		return Result{}, errors.New("grace_days must not be negative")
	}
	expired := opts.Now.After(exp)
	expiryDeadline := exp
	if !opts.DisableGracePeriod {
		expiryDeadline = exp.AddDate(0, 0, file.Payload.GraceDays)
	}
	if opts.Now.After(expiryDeadline) {
		if opts.DisableGracePeriod {
			return Result{}, errors.New("license expired")
		}
		return Result{}, errors.New("license expired beyond grace period")
	}

	return Result{
		LicenseID: file.Payload.LicenseID,
		ServerID:  file.Payload.ServerID,
		Plan:      file.Payload.Plan,
		Valid:     true,
		Expired:   expired,
		Features:  append([]string(nil), file.Payload.Features...),
	}, nil
}

func CanonicalPayload(payload Payload) ([]byte, error) {
	return json.Marshal(payload)
}

func CanonicalLicenseFile(file File) ([]byte, error) {
	return json.MarshalIndent(file, "", "  ")
}

func EncodeSignature(signature []byte) string {
	return signaturePrefix + base64.StdEncoding.EncodeToString(signature)
}

func Fingerprint(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func FingerprintHash(parts ...string) string {
	return "sha256:" + Fingerprint(parts...)
}

func HashValue(value string) string {
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func verifySignature(file File, publicKey ed25519.PublicKey) error {
	signatureText := strings.TrimSpace(file.Signature)
	if !strings.HasPrefix(signatureText, signaturePrefix) {
		return errors.New("signature must use ed25519:<base64> format")
	}
	encoded := strings.TrimPrefix(signatureText, signaturePrefix)
	signature, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return err
	}
	payload, err := CanonicalPayload(file.Payload)
	if err != nil {
		return err
	}
	if !ed25519.Verify(publicKey, payload, signature) {
		return errors.New("license signature verification failed")
	}
	return nil
}

func verifyMachineBinding(binding MachineBinding, actualFingerprintHash string) error {
	if actualFingerprintHash == "" {
		return nil
	}
	if strings.TrimSpace(binding.FingerprintHash) == "" {
		return errors.New("license machine_binding.fingerprint_hash is required")
	}
	if normalizeHash(binding.FingerprintHash) != normalizeHash(actualFingerprintHash) {
		return errors.New("license machine binding does not match this server")
	}
	return nil
}

func normalizeHash(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "sha256:") {
		return value
	}
	return "sha256:" + value
}

func has(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
