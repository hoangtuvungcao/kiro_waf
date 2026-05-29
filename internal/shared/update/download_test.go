package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadURLSupportsHTTPAndFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("artifact"))
	}))
	defer server.Close()
	dir := t.TempDir()
	result, err := DownloadURL(context.Background(), server.URL+"/artifact.bin", filepath.Join(dir, "artifact.bin"), DownloadOptions{})
	if err != nil {
		t.Fatalf("download http: %v", err)
	}
	if result.SizeBytes != int64(len("artifact")) || result.SHA256 == "" {
		t.Fatalf("unexpected result: %#v", result)
	}
	source := filepath.Join(dir, "source.txt")
	if err := os.WriteFile(source, []byte("local"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	local, err := DownloadURL(context.Background(), "file://"+source, filepath.Join(dir, "local.txt"), DownloadOptions{})
	if err != nil {
		t.Fatalf("download file: %v", err)
	}
	if local.SizeBytes != int64(len("local")) {
		t.Fatalf("local result = %#v", local)
	}
}

func TestFetchManifestAndArtifactsDownloadsArtifact(t *testing.T) {
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/manifest.json":
			_, _ = w.Write([]byte(`{
  "payload": {
    "product": "kiro_waf",
    "version": "1.0.1",
    "channel": "stable",
    "released_at": "2026-05-28T00:00:00Z",
    "min_agent_version": "1.0.0",
    "artifacts": [
      {
        "name": "kiro-agent.bin",
        "url": "` + serverURL + `/kiro-agent.bin",
        "sha256": "sha256:unused"
      }
    ],
    "rollback": {"supported": true, "keep_previous_versions": 2}
  },
  "signature": "ed25519:placeholder"
}`))
		case "/kiro-agent.bin":
			_, _ = w.Write([]byte("agent bytes"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	serverURL = server.URL
	result, err := FetchManifestAndArtifacts(context.Background(), server.URL+"/manifest.json", DownloadOptions{OutputDir: t.TempDir()})
	if err != nil {
		t.Fatalf("fetch manifest/artifacts: %v", err)
	}
	if result.Manifest.Path == "" || len(result.Artifacts) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	raw, err := os.ReadFile(result.Artifacts[0].Path)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	if string(raw) != "agent bytes" {
		t.Fatalf("artifact = %q", raw)
	}
}
