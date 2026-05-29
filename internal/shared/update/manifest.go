package update

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"kiro_waf/internal/shared/licenseverify"
)

const signaturePrefix = "ed25519:"

type ManifestFile struct {
	Payload   ManifestPayload `json:"payload"`
	Signature string          `json:"signature"`
}

type ManifestPayload struct {
	Product         string         `json:"product"`
	Version         string         `json:"version"`
	Channel         string         `json:"channel"`
	ReleasedAt      string         `json:"released_at"`
	MinAgentVersion string         `json:"min_agent_version"`
	Artifacts       []Artifact     `json:"artifacts"`
	PolicyBundle    *PolicyBundle  `json:"policy_bundle,omitempty"`
	Rollback        RollbackPolicy `json:"rollback"`
	Release         *ReleaseInfo   `json:"release,omitempty"`
	Notes           []string       `json:"notes,omitempty"`
}

type Artifact struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	SHA256    string `json:"sha256"`
	Signature string `json:"signature,omitempty"`
}

type PolicyBundle struct {
	Version string `json:"version"`
	URL     string `json:"url"`
	SHA256  string `json:"sha256"`
}

type RollbackPolicy struct {
	Supported            bool `json:"supported"`
	KeepPreviousVersions int  `json:"keep_previous_versions"`
}

type ReleaseInfo struct {
	Changelog     []string           `json:"changelog"`
	MigrationNote string             `json:"migration_note"`
	RollbackNote  string             `json:"rollback_note"`
	Compatibility []CompatibilityRow `json:"compatibility"`
}

type CompatibilityRow struct {
	Target string `json:"target"`
	Status string `json:"status"`
	Notes  string `json:"notes,omitempty"`
}

type VerifyOptions struct {
	Product                  string
	Channel                  string
	CurrentVersion           string
	AllowDowngrade           bool
	RequireRollback          bool
	RequireArtifacts         bool
	RequireReleaseMetadata   bool
	RequireArtifactSignature bool
	Now                      time.Time
}

type VerifyResult struct {
	Product       string
	Version       string
	Channel       string
	Artifacts     []Artifact
	RollbackReady bool
	Release       *ReleaseInfo
}

func SignManifest(payload ManifestPayload, privateKey ed25519.PrivateKey) (ManifestFile, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return ManifestFile{}, fmt.Errorf("ed25519 private key must be %d bytes", ed25519.PrivateKeySize)
	}
	if err := ValidatePayload(payload); err != nil {
		return ManifestFile{}, err
	}
	canonical, err := CanonicalPayload(payload)
	if err != nil {
		return ManifestFile{}, err
	}
	signature := ed25519.Sign(privateKey, canonical)
	return ManifestFile{Payload: payload, Signature: licenseverify.EncodeSignature(signature)}, nil
}

func VerifyFile(manifestPath string, publicKeyPath string, opts VerifyOptions) (VerifyResult, error) {
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return VerifyResult{}, err
	}
	var file ManifestFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return VerifyResult{}, err
	}
	publicKey, err := licenseverify.LoadPublicKeyFile(publicKeyPath)
	if err != nil {
		return VerifyResult{}, err
	}
	return Verify(file, publicKey, opts)
}

func Verify(file ManifestFile, publicKey ed25519.PublicKey, opts VerifyOptions) (VerifyResult, error) {
	if len(publicKey) != ed25519.PublicKeySize {
		return VerifyResult{}, fmt.Errorf("ed25519 public key must be %d bytes", ed25519.PublicKeySize)
	}
	if err := ValidatePayload(file.Payload); err != nil {
		return VerifyResult{}, err
	}
	if err := verifySignature(file, publicKey); err != nil {
		return VerifyResult{}, err
	}
	if opts.Product != "" && file.Payload.Product != opts.Product {
		return VerifyResult{}, fmt.Errorf("manifest product %q does not match %q", file.Payload.Product, opts.Product)
	}
	if opts.Channel != "" && file.Payload.Channel != opts.Channel {
		return VerifyResult{}, fmt.Errorf("manifest channel %q does not match %q", file.Payload.Channel, opts.Channel)
	}
	if opts.RequireRollback && !file.Payload.Rollback.Supported {
		return VerifyResult{}, errors.New("manifest does not support rollback")
	}
	if opts.RequireArtifacts && len(file.Payload.Artifacts) == 0 {
		return VerifyResult{}, errors.New("manifest contains no artifacts")
	}
	if opts.RequireReleaseMetadata {
		if err := ValidateReleaseInfo(file.Payload.Release); err != nil {
			return VerifyResult{}, err
		}
	}
	for i, artifact := range file.Payload.Artifacts {
		if opts.RequireArtifactSignature && strings.TrimSpace(artifact.Signature) == "" {
			return VerifyResult{}, fmt.Errorf("manifest artifacts[%d]: signature is required", i)
		}
		if strings.TrimSpace(artifact.Signature) != "" {
			if err := VerifyArtifactSignature(artifact, publicKey); err != nil {
				return VerifyResult{}, fmt.Errorf("manifest artifacts[%d]: %w", i, err)
			}
		}
	}
	if opts.CurrentVersion != "" && !opts.AllowDowngrade && compareDottedVersion(file.Payload.Version, opts.CurrentVersion) < 0 {
		return VerifyResult{}, errors.New("manifest version is a downgrade")
	}
	return VerifyResult{
		Product:       file.Payload.Product,
		Version:       file.Payload.Version,
		Channel:       file.Payload.Channel,
		Artifacts:     append([]Artifact(nil), file.Payload.Artifacts...),
		RollbackReady: file.Payload.Rollback.Supported,
		Release:       cloneReleaseInfo(file.Payload.Release),
	}, nil
}

func ValidatePayload(payload ManifestPayload) error {
	if strings.TrimSpace(payload.Product) == "" {
		return errors.New("manifest payload.product is required")
	}
	if strings.TrimSpace(payload.Version) == "" {
		return errors.New("manifest payload.version is required")
	}
	if strings.TrimSpace(payload.Channel) == "" {
		return errors.New("manifest payload.channel is required")
	}
	if strings.TrimSpace(payload.ReleasedAt) == "" {
		return errors.New("manifest payload.released_at is required")
	}
	if _, err := time.Parse(time.RFC3339, payload.ReleasedAt); err != nil {
		return fmt.Errorf("manifest payload.released_at invalid: %w", err)
	}
	if payload.Rollback.KeepPreviousVersions < 0 {
		return errors.New("manifest rollback.keep_previous_versions must not be negative")
	}
	for i, artifact := range payload.Artifacts {
		if err := validateArtifact(artifact); err != nil {
			return fmt.Errorf("manifest artifacts[%d]: %w", i, err)
		}
	}
	if payload.PolicyBundle != nil {
		if strings.TrimSpace(payload.PolicyBundle.Version) == "" {
			return errors.New("manifest policy_bundle.version is required")
		}
		if strings.TrimSpace(payload.PolicyBundle.URL) == "" {
			return errors.New("manifest policy_bundle.url is required")
		}
		if strings.TrimSpace(payload.PolicyBundle.SHA256) == "" {
			return errors.New("manifest policy_bundle.sha256 is required")
		}
	}
	if payload.Release != nil {
		if err := ValidateReleaseInfo(payload.Release); err != nil {
			return err
		}
	}
	return nil
}

func ValidateReleaseInfo(release *ReleaseInfo) error {
	if release == nil {
		return errors.New("manifest release metadata is required")
	}
	if len(release.Changelog) == 0 {
		return errors.New("manifest release.changelog is required")
	}
	for i, item := range release.Changelog {
		if strings.TrimSpace(item) == "" {
			return fmt.Errorf("manifest release.changelog[%d] is empty", i)
		}
	}
	if strings.TrimSpace(release.MigrationNote) == "" {
		return errors.New("manifest release.migration_note is required")
	}
	if strings.TrimSpace(release.RollbackNote) == "" {
		return errors.New("manifest release.rollback_note is required")
	}
	if len(release.Compatibility) == 0 {
		return errors.New("manifest release.compatibility is required")
	}
	for i, row := range release.Compatibility {
		if strings.TrimSpace(row.Target) == "" {
			return fmt.Errorf("manifest release.compatibility[%d].target is required", i)
		}
		switch strings.TrimSpace(row.Status) {
		case "pass", "warn", "unsupported", "not_tested":
		default:
			return fmt.Errorf("manifest release.compatibility[%d].status must be pass, warn, unsupported, or not_tested", i)
		}
	}
	return nil
}

func CanonicalPayload(payload ManifestPayload) ([]byte, error) {
	return json.Marshal(payload)
}

func CanonicalManifestFile(file ManifestFile) ([]byte, error) {
	return json.MarshalIndent(file, "", "  ")
}

func VerifyArtifactFile(artifact Artifact, artifactPath string) error {
	if err := validateArtifact(artifact); err != nil {
		return err
	}
	f, err := os.Open(artifactPath)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	want := normalizeSHA256(artifact.SHA256)
	if got != want {
		return fmt.Errorf("artifact checksum mismatch: got sha256:%s want sha256:%s", got, want)
	}
	return nil
}

func VerifyArtifactFileWithPublicKey(artifact Artifact, artifactPath string, publicKey ed25519.PublicKey) error {
	if err := VerifyArtifactFile(artifact, artifactPath); err != nil {
		return err
	}
	if strings.TrimSpace(artifact.Signature) == "" {
		return nil
	}
	return VerifyArtifactSignature(artifact, publicKey)
}

func ArtifactSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

func SignArtifact(artifact Artifact, privateKey ed25519.PrivateKey) (string, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return "", fmt.Errorf("ed25519 private key must be %d bytes", ed25519.PrivateKeySize)
	}
	if err := validateArtifact(Artifact{Name: artifact.Name, URL: firstNonEmpty(artifact.URL, "file://artifact"), SHA256: artifact.SHA256}); err != nil {
		return "", err
	}
	canonical, err := canonicalArtifactSignaturePayload(artifact)
	if err != nil {
		return "", err
	}
	return licenseverify.EncodeSignature(ed25519.Sign(privateKey, canonical)), nil
}

func VerifyArtifactSignature(artifact Artifact, publicKey ed25519.PublicKey) error {
	if len(publicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("ed25519 public key must be %d bytes", ed25519.PublicKeySize)
	}
	signatureText := strings.TrimSpace(artifact.Signature)
	if !strings.HasPrefix(signatureText, signaturePrefix) {
		return errors.New("artifact signature must use ed25519:<base64> format")
	}
	signature, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(signatureText, signaturePrefix))
	if err != nil {
		return err
	}
	canonical, err := canonicalArtifactSignaturePayload(artifact)
	if err != nil {
		return err
	}
	if !ed25519.Verify(publicKey, canonical, signature) {
		return errors.New("artifact signature verification failed")
	}
	return nil
}

func verifySignature(file ManifestFile, publicKey ed25519.PublicKey) error {
	signatureText := strings.TrimSpace(file.Signature)
	if !strings.HasPrefix(signatureText, signaturePrefix) {
		return errors.New("manifest signature must use ed25519:<base64> format")
	}
	signature, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(signatureText, signaturePrefix))
	if err != nil {
		return err
	}
	canonical, err := CanonicalPayload(file.Payload)
	if err != nil {
		return err
	}
	if !ed25519.Verify(publicKey, canonical, signature) {
		return errors.New("manifest signature verification failed")
	}
	return nil
}

func validateArtifact(artifact Artifact) error {
	if strings.TrimSpace(artifact.Name) == "" {
		return errors.New("name is required")
	}
	if strings.TrimSpace(artifact.URL) == "" {
		return errors.New("url is required")
	}
	if strings.TrimSpace(artifact.SHA256) == "" {
		return errors.New("sha256 is required")
	}
	return nil
}

func canonicalArtifactSignaturePayload(artifact Artifact) ([]byte, error) {
	payload := struct {
		Name   string `json:"name"`
		SHA256 string `json:"sha256"`
	}{
		Name:   strings.TrimSpace(artifact.Name),
		SHA256: "sha256:" + normalizeSHA256(artifact.SHA256),
	}
	return json.Marshal(payload)
}

func normalizeSHA256(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "sha256:")
	return strings.ToLower(value)
}

func cloneReleaseInfo(release *ReleaseInfo) *ReleaseInfo {
	if release == nil {
		return nil
	}
	cloned := *release
	cloned.Changelog = append([]string(nil), release.Changelog...)
	cloned.Compatibility = append([]CompatibilityRow(nil), release.Compatibility...)
	return &cloned
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
