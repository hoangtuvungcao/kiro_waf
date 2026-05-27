package provider

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kiro_waf/internal/shared/config"
	"kiro_waf/internal/shared/licenseverify"
	"kiro_waf/internal/shared/storage"
)

type IssueRequest struct {
	LicenseID           string
	CustomerID          string
	ServerID            string
	Plan                string
	MachineIDHash       string
	PrimaryMACHash      string
	FingerprintHash     string
	ValidDays           int
	UpdateChannel       string
	PolicyBundleVersion string
	Now                 time.Time
}

type IssueResult struct {
	License     licenseverify.File
	LicensePath string
}

type ActivationRecord struct {
	LicenseID       string `json:"license_id"`
	CustomerID      string `json:"customer_id"`
	ServerID        string `json:"server_id"`
	Plan            string `json:"plan"`
	FingerprintHash string `json:"fingerprint_hash"`
	ActivatedAt     string `json:"activated_at"`
}

func EnsureKeyPair(privateKeyPath string, publicKeyPath string, force bool) (ed25519.PublicKey, error) {
	if !force {
		if publicKey, err := loadExistingKeyPair(privateKeyPath, publicKeyPath); err == nil {
			return publicKey, nil
		}
	}

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	if err := writePrivateKeyPEM(privateKeyPath, privateKey); err != nil {
		return nil, err
	}
	if err := writePublicKeyPEM(publicKeyPath, publicKey); err != nil {
		return nil, err
	}
	return publicKey, nil
}

func IssueLicense(cfg config.ProviderConfig, req IssueRequest) (IssueResult, error) {
	privateKey, err := LoadPrivateKeyFile(cfg.Provider.SigningKeyFile)
	if err != nil {
		return IssueResult{}, err
	}
	file, err := BuildSignedLicense(cfg, req, privateKey)
	if err != nil {
		return IssueResult{}, err
	}

	licensePath := filepath.Join(cfg.Storage.RootDir, "licenses", file.Payload.LicenseID+".json")
	if err := storage.WriteJSONAtomic(licensePath, file); err != nil {
		return IssueResult{}, err
	}
	if err := writeProviderRecords(cfg, file); err != nil {
		return IssueResult{}, err
	}
	return IssueResult{License: file, LicensePath: licensePath}, nil
}

func BuildSignedLicense(cfg config.ProviderConfig, req IssueRequest, privateKey ed25519.PrivateKey) (licenseverify.File, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return licenseverify.File{}, fmt.Errorf("ed25519 private key must be %d bytes", ed25519.PrivateKeySize)
	}
	req = defaultIssueRequest(req)
	if err := validateIssueRequest(cfg, req); err != nil {
		return licenseverify.File{}, err
	}
	plan := cfg.Licenses.Plans[req.Plan]
	expiresAt := req.Now.AddDate(0, 0, req.ValidDays)
	payload := licenseverify.Payload{
		LicenseID:  req.LicenseID,
		CustomerID: req.CustomerID,
		ServerID:   req.ServerID,
		Plan:       req.Plan,
		Modes:      append([]string(nil), plan.AllowedModes...),
		Features:   append([]string(nil), plan.Features...),
		MachineBinding: licenseverify.MachineBinding{
			MachineIDHash:   req.MachineIDHash,
			PrimaryMACHash:  req.PrimaryMACHash,
			FingerprintHash: req.FingerprintHash,
		},
		IssuedAt:            req.Now.Format(time.RFC3339),
		ExpiresAt:           expiresAt.Format(time.RFC3339),
		GraceDays:           cfg.Licenses.DefaultGraceDays,
		UpdateChannel:       req.UpdateChannel,
		PolicyBundleVersion: req.PolicyBundleVersion,
	}
	canonical, err := licenseverify.CanonicalPayload(payload)
	if err != nil {
		return licenseverify.File{}, err
	}
	signature := ed25519.Sign(privateKey, canonical)
	return licenseverify.File{Payload: payload, Signature: licenseverify.EncodeSignature(signature)}, nil
}

func ExportAgentFiles(agentDir string, licenseFile licenseverify.File, publicKeyPath string) error {
	if strings.TrimSpace(agentDir) == "" {
		return errors.New("agent output directory is required")
	}
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		return err
	}
	if err := storage.WriteJSONAtomic(filepath.Join(agentDir, "license.json"), licenseFile); err != nil {
		return err
	}
	publicKey, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(agentDir, "provider-public-key.pem"), publicKey, 0o644)
}

func LoadPrivateKeyFile(path string) (ed25519.PrivateKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, errors.New("private key must be PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	privateKey, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not Ed25519")
	}
	return privateKey, nil
}

func loadExistingKeyPair(privateKeyPath string, publicKeyPath string) (ed25519.PublicKey, error) {
	privateKey, err := LoadPrivateKeyFile(privateKeyPath)
	if err != nil {
		return nil, err
	}
	publicKey, ok := privateKey.Public().(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("private key public half is not Ed25519")
	}
	if _, err := os.Stat(publicKeyPath); errors.Is(err, os.ErrNotExist) {
		if err := writePublicKeyPEM(publicKeyPath, publicKey); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	return publicKey, nil
}

func writePrivateKeyPEM(path string, privateKey ed25519.PrivateKey) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("private key path is required")
	}
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return err
	}
	block := &pem.Block{Type: "PRIVATE KEY", Bytes: der}
	return writeFileCreatingDir(path, pem.EncodeToMemory(block), 0o600)
}

func writePublicKeyPEM(path string, publicKey ed25519.PublicKey) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("public key path is required")
	}
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return err
	}
	block := &pem.Block{Type: "PUBLIC KEY", Bytes: der}
	return writeFileCreatingDir(path, pem.EncodeToMemory(block), 0o644)
}

func writeFileCreatingDir(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, perm)
}

func defaultIssueRequest(req IssueRequest) IssueRequest {
	if req.Now.IsZero() {
		req.Now = time.Now().UTC()
	}
	if req.ValidDays == 0 {
		req.ValidDays = 365
	}
	if req.UpdateChannel == "" {
		req.UpdateChannel = "stable"
	}
	if req.PolicyBundleVersion == "" {
		req.PolicyBundleVersion = "dev"
	}
	return req
}

func validateIssueRequest(cfg config.ProviderConfig, req IssueRequest) error {
	if strings.TrimSpace(req.LicenseID) == "" {
		return errors.New("license id is required")
	}
	if strings.TrimSpace(req.CustomerID) == "" {
		return errors.New("customer id is required")
	}
	if strings.TrimSpace(req.ServerID) == "" {
		return errors.New("server id is required")
	}
	if _, ok := cfg.Licenses.Plans[req.Plan]; !ok {
		return fmt.Errorf("unknown license plan %q", req.Plan)
	}
	if strings.TrimSpace(req.FingerprintHash) == "" {
		return errors.New("fingerprint hash is required")
	}
	if req.ValidDays <= 0 {
		return errors.New("valid days must be positive")
	}
	return nil
}

func writeProviderRecords(cfg config.ProviderConfig, file licenseverify.File) error {
	payload := file.Payload
	now := payload.IssuedAt
	root := cfg.Storage.RootDir
	if err := storage.WriteJSONAtomic(filepath.Join(root, "customers", payload.CustomerID+".json"), map[string]any{
		"customer_id": payload.CustomerID,
		"updated_at":  now,
	}); err != nil {
		return err
	}
	if err := storage.WriteJSONAtomic(filepath.Join(root, "servers", payload.ServerID+".json"), map[string]any{
		"server_id":        payload.ServerID,
		"customer_id":      payload.CustomerID,
		"fingerprint_hash": payload.MachineBinding.FingerprintHash,
		"updated_at":       now,
	}); err != nil {
		return err
	}
	activationPath := filepath.Join(root, "activations", payload.IssuedAt[:7]+".jsonl")
	return storage.AppendJSONL(activationPath, ActivationRecord{
		LicenseID:       payload.LicenseID,
		CustomerID:      payload.CustomerID,
		ServerID:        payload.ServerID,
		Plan:            payload.Plan,
		FingerprintHash: payload.MachineBinding.FingerprintHash,
		ActivatedAt:     payload.IssuedAt,
	})
}
