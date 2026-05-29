package update

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeSHA256(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"abc123", "abc123"},
		{"sha256:abc123", "abc123"},
		{"  sha256:ABC123  ", "abc123"},
		{"SHA256:DEADBEEF", "deadbeef"},
		{"", ""},
	}

	for _, tc := range tests {
		got := normalizeSHA256(tc.input)
		if got != tc.expected {
			t.Errorf("normalizeSHA256(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestComputeSHA256(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "testfile")

	content := []byte("hello world")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := computeSHA256(path)
	if err != nil {
		t.Fatal(err)
	}

	h := sha256.Sum256(content)
	expected := hex.EncodeToString(h[:])

	if got != expected {
		t.Errorf("computeSHA256 = %q, want %q", got, expected)
	}
}

func TestComputeSHA256_FileNotFound(t *testing.T) {
	_, err := computeSHA256("/nonexistent/path/file")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestCheck_NoUpdate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/update/check" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var req updateCheckRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.Component != "kiro-client-waf" {
			t.Errorf("unexpected component: %s", req.Component)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(updateCheckResponse{UpdateAvailable: false})
	}))
	defer server.Close()

	err := Check(server.URL, "kiro-client-waf", "stable", "1.0.0")
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
}

func TestCheck_UpdateAvailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(updateCheckResponse{
			UpdateAvailable: true,
			Release: &releaseInfo{
				Version:     "2.0.0",
				ArtifactURL: "https://example.com/artifact.bin",
				SHA256:      "sha256:abcdef1234567890",
				Notes:       "Bug fixes",
			},
		})
	}))
	defer server.Close()

	err := Check(server.URL, "kiro-client-waf", "stable", "1.0.0")
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
}

func TestCheck_EmptyMasterURL(t *testing.T) {
	err := Check("", "kiro-client-waf", "stable", "1.0.0")
	if err == nil {
		t.Error("expected error for empty master URL")
	}
}

func TestCheck_EmptyComponent(t *testing.T) {
	err := Check("http://localhost", "", "stable", "1.0.0")
	if err == nil {
		t.Error("expected error for empty component")
	}
}

func TestApply_EmptyParams(t *testing.T) {
	tests := []struct {
		name        string
		masterURL   string
		component   string
		binaryPath  string
		serviceName string
	}{
		{"empty master URL", "", "comp", "/bin/test", "svc"},
		{"empty component", "http://localhost", "", "/bin/test", "svc"},
		{"empty binary path", "http://localhost", "comp", "", "svc"},
		{"empty service name", "http://localhost", "comp", "/bin/test", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Apply(tc.masterURL, tc.component, "stable", "1.0.0", tc.binaryPath, tc.serviceName)
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestApply_NoUpdateAvailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(updateCheckResponse{UpdateAvailable: false})
	}))
	defer server.Close()

	err := Apply(server.URL, "kiro-client-waf", "stable", "1.0.0", "/usr/local/bin/kiro-client-waf", "kiro-client-waf")
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
}

func TestApply_SHA256Mismatch(t *testing.T) {
	dir := t.TempDir()
	binaryPath := filepath.Join(dir, "kiro-client-waf")
	if err := os.WriteFile(binaryPath, []byte("current binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	artifactContent := []byte("new binary content")

	// Artifact server serves the binary.
	artifactServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(artifactContent)
	}))
	defer artifactServer.Close()

	// Master server returns update with wrong SHA-256.
	masterServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(updateCheckResponse{
			UpdateAvailable: true,
			Release: &releaseInfo{
				Version:     "2.0.0",
				ArtifactURL: artifactServer.URL + "/artifact.bin",
				SHA256:      "sha256:0000000000000000000000000000000000000000000000000000000000000000",
				Notes:       "test",
			},
		})
	}))
	defer masterServer.Close()

	err := Apply(masterServer.URL, "kiro-client-waf", "stable", "1.0.0", binaryPath, "kiro-client-waf")
	if err == nil {
		t.Fatal("expected SHA-256 mismatch error")
	}
	if got := err.Error(); !contains(got, "SHA-256 mismatch") {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify original binary is still intact.
	content, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "current binary" {
		t.Errorf("original binary was modified: %q", string(content))
	}

	// Verify download file was cleaned up.
	downloadPath := binaryPath + ".download"
	if _, err := os.Stat(downloadPath); !os.IsNotExist(err) {
		t.Error("download file was not cleaned up")
	}
}

func TestApply_SHA256Match_HealthFail(t *testing.T) {
	// This test verifies the SHA-256 verification passes but we can't easily
	// test the systemctl restart/health check in unit tests. The SHA-256
	// verification logic is the critical path tested here.
	dir := t.TempDir()
	binaryPath := filepath.Join(dir, "kiro-client-waf")
	if err := os.WriteFile(binaryPath, []byte("current binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	artifactContent := []byte("new binary content")
	h := sha256.Sum256(artifactContent)
	correctHash := hex.EncodeToString(h[:])

	// Artifact server serves the binary.
	artifactServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(artifactContent)
	}))
	defer artifactServer.Close()

	// Master server returns update with correct SHA-256.
	masterServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(updateCheckResponse{
			UpdateAvailable: true,
			Release: &releaseInfo{
				Version:     "2.0.0",
				ArtifactURL: artifactServer.URL + "/artifact.bin",
				SHA256:      fmt.Sprintf("sha256:%s", correctHash),
				Notes:       "test",
			},
		})
	}))
	defer masterServer.Close()

	// Apply will fail at the systemctl restart step (not available in test env),
	// but the SHA-256 verification and atomic replace should succeed.
	err := Apply(masterServer.URL, "kiro-client-waf", "stable", "1.0.0", binaryPath, "kiro-client-waf")
	// We expect an error because systemctl is not available in test environment.
	if err == nil {
		t.Log("Apply succeeded (systemctl available in test env)")
	} else {
		// The error should be about restart/health, not SHA-256.
		if contains(err.Error(), "SHA-256 mismatch") {
			t.Fatalf("unexpected SHA-256 mismatch error: %v", err)
		}
	}
}

func TestRollback_EmptyParams(t *testing.T) {
	err := Rollback("", "svc")
	if err == nil {
		t.Error("expected error for empty binary path")
	}

	err = Rollback("/bin/test", "")
	if err == nil {
		t.Error("expected error for empty service name")
	}
}

func TestRollback_NoBackup(t *testing.T) {
	dir := t.TempDir()
	binaryPath := filepath.Join(dir, "kiro-client-waf")

	err := Rollback(binaryPath, "kiro-client-waf")
	if err == nil {
		t.Error("expected error when no backup exists")
	}
	if !contains(err.Error(), "no backup found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDownloadArtifact(t *testing.T) {
	content := []byte("artifact binary data")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(content)
	}))
	defer server.Close()

	dir := t.TempDir()
	outputPath := filepath.Join(dir, "downloaded.bin")

	err := downloadArtifact(server.URL+"/artifact.bin", outputPath)
	if err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Errorf("downloaded content mismatch: got %q, want %q", got, content)
	}
}

func TestDownloadArtifact_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	dir := t.TempDir()
	outputPath := filepath.Join(dir, "downloaded.bin")

	err := downloadArtifact(server.URL+"/artifact.bin", outputPath)
	if err == nil {
		t.Error("expected error for server error response")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
