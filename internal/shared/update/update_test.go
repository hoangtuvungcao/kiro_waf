package update

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestVerifyRejectsBadManifestSignature(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	file, err := SignManifest(testPayload(), privateKey)
	if err != nil {
		t.Fatalf("sign manifest: %v", err)
	}
	file.Payload.Version = "9.9.9"
	if _, err := Verify(file, publicKey, VerifyOptions{Product: "kiro_waf", Channel: "stable"}); err == nil {
		t.Fatal("expected bad manifest signature to fail")
	}
}

func TestVerifyArtifactRejectsBadChecksum(t *testing.T) {
	path := filepath.Join(t.TempDir(), "artifact.bin")
	if err := os.WriteFile(path, []byte("tampered"), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	err := VerifyArtifactFile(Artifact{Name: "artifact.bin", URL: "file://" + path, SHA256: "sha256:deadbeef"}, path)
	if err == nil {
		t.Fatal("expected checksum mismatch")
	}
}

func TestVerifyArtifactAcceptsMatchingChecksum(t *testing.T) {
	path := filepath.Join(t.TempDir(), "artifact.bin")
	if err := os.WriteFile(path, []byte("ok"), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	sum, err := ArtifactSHA256(path)
	if err != nil {
		t.Fatalf("artifact sha256: %v", err)
	}
	if err := VerifyArtifactFile(Artifact{Name: "artifact.bin", URL: "file://" + path, SHA256: sum}, path); err != nil {
		t.Fatalf("verify artifact: %v", err)
	}
}

func TestVerifyReleaseMetadataAndArtifactSignature(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	artifactPath := filepath.Join(t.TempDir(), "artifact.bin")
	if err := os.WriteFile(artifactPath, []byte("ok"), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	sum, err := ArtifactSHA256(artifactPath)
	if err != nil {
		t.Fatalf("artifact sha256: %v", err)
	}
	payload := testPayload()
	payload.Artifacts[0].SHA256 = sum
	payload.Release = testReleaseInfo()
	signature, err := SignArtifact(payload.Artifacts[0], privateKey)
	if err != nil {
		t.Fatalf("sign artifact: %v", err)
	}
	payload.Artifacts[0].Signature = signature
	file, err := SignManifest(payload, privateKey)
	if err != nil {
		t.Fatalf("sign manifest: %v", err)
	}
	result, err := Verify(file, publicKey, VerifyOptions{
		Product:                  "kiro_waf",
		Channel:                  "stable",
		RequireReleaseMetadata:   true,
		RequireArtifactSignature: true,
	})
	if err != nil {
		t.Fatalf("verify release manifest: %v", err)
	}
	if result.Release == nil || len(result.Release.Compatibility) != 1 {
		t.Fatalf("release metadata missing: %#v", result.Release)
	}
	if err := VerifyArtifactFileWithPublicKey(result.Artifacts[0], artifactPath, publicKey); err != nil {
		t.Fatalf("verify signed artifact: %v", err)
	}
}

func TestVerifyRejectsMissingReleaseMetadataWhenRequired(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	file, err := SignManifest(testPayload(), privateKey)
	if err != nil {
		t.Fatalf("sign manifest: %v", err)
	}
	if _, err := Verify(file, publicKey, VerifyOptions{RequireReleaseMetadata: true}); err == nil {
		t.Fatal("expected missing release metadata to fail")
	}
}

func TestApplyFileWithRollbackRestoresTargetOnHealthFailure(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "kiro-agent")
	artifact := filepath.Join(dir, "new-agent")
	if err := os.WriteFile(target, []byte("old-version"), 0o755); err != nil {
		t.Fatalf("write target: %v", err)
	}
	if err := os.WriteFile(artifact, []byte("new-version"), 0o755); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	healthErr := errors.New("agent crashed")
	result, err := ApplyFileWithRollback(ApplyOptions{
		TargetPath:   target,
		ArtifactPath: artifact,
		StateDir:     filepath.Join(dir, "state"),
		Version:      "1.0.1",
		Now:          time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC),
		HealthCheck: func() error {
			return healthErr
		},
	})
	if !errors.Is(err, healthErr) {
		t.Fatalf("error = %v, want health error", err)
	}
	if !result.RolledBack {
		t.Fatalf("result = %#v, want rolled back", result)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(got) != "old-version" {
		t.Fatalf("target = %q, want old-version", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "state", pendingUpdateFile)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pending file should be removed after rollback, err=%v", err)
	}
}

func testReleaseInfo() *ReleaseInfo {
	return &ReleaseInfo{
		Changelog:     []string{"add release management checks"},
		MigrationNote: "no migration required",
		RollbackNote:  "use kiro-agent --update-rollback if health fails",
		Compatibility: []CompatibilityRow{{
			Target: "ubuntu_22.04_amd64",
			Status: "pass",
			Notes:  "lab smoke passed",
		}},
	}
}

func TestVerifyRejectsDowngradeByDefault(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	payload := testPayload()
	payload.Version = "1.0.0"
	file, err := SignManifest(payload, privateKey)
	if err != nil {
		t.Fatalf("sign manifest: %v", err)
	}
	if _, err := Verify(file, publicKey, VerifyOptions{CurrentVersion: "1.1.0"}); err == nil {
		t.Fatal("expected downgrade to fail")
	}
}

func testPayload() ManifestPayload {
	return ManifestPayload{
		Product:         "kiro_waf",
		Version:         "1.0.1",
		Channel:         "stable",
		ReleasedAt:      "2026-05-28T00:00:00Z",
		MinAgentVersion: "1.0.0",
		Artifacts: []Artifact{{
			Name:   "kiro-agent_linux_amd64.tar.gz",
			URL:    "https://provider.example.com/updates/stable/kiro-agent_linux_amd64.tar.gz",
			SHA256: "sha256:abcdef",
		}},
		Rollback: RollbackPolicy{
			Supported:            true,
			KeepPreviousVersions: 2,
		},
	}
}
