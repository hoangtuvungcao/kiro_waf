package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type DownloadOptions struct {
	OutputDir string
	Timeout   time.Duration
	Client    *http.Client
}

type DownloadResult struct {
	URL       string `json:"url"`
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256,omitempty"`
}

type FetchResult struct {
	Manifest  DownloadResult   `json:"manifest"`
	Artifacts []DownloadResult `json:"artifacts"`
}

func DownloadURL(ctx context.Context, rawURL string, outputPath string, opts DownloadOptions) (DownloadResult, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return DownloadResult{}, errors.New("download URL is required")
	}
	if strings.TrimSpace(outputPath) == "" {
		return DownloadResult{}, errors.New("download output path is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return DownloadResult{}, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(outputPath), ".tmp-download-*")
	if err != nil {
		return DownloadResult{}, err
	}
	tmpName := tmp.Name()
	size, err := copyURL(ctx, tmp, rawURL, opts)
	closeErr := tmp.Close()
	if err != nil {
		_ = os.Remove(tmpName)
		return DownloadResult{}, err
	}
	if closeErr != nil {
		_ = os.Remove(tmpName)
		return DownloadResult{}, closeErr
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		_ = os.Remove(tmpName)
		return DownloadResult{}, err
	}
	if err := os.Rename(tmpName, outputPath); err != nil {
		_ = os.Remove(tmpName)
		return DownloadResult{}, err
	}
	sum, err := ArtifactSHA256(outputPath)
	if err != nil {
		return DownloadResult{}, err
	}
	return DownloadResult{URL: rawURL, Path: outputPath, SizeBytes: size, SHA256: sum}, nil
}

func FetchManifestAndArtifacts(ctx context.Context, manifestURL string, opts DownloadOptions) (FetchResult, error) {
	if strings.TrimSpace(opts.OutputDir) == "" {
		return FetchResult{}, errors.New("download output directory is required")
	}
	manifestPath := filepath.Join(opts.OutputDir, "manifest.json")
	manifest, err := DownloadURL(ctx, manifestURL, manifestPath, opts)
	if err != nil {
		return FetchResult{}, err
	}
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return FetchResult{}, err
	}
	var file ManifestFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return FetchResult{}, err
	}
	result := FetchResult{Manifest: manifest}
	for _, artifact := range file.Payload.Artifacts {
		if strings.TrimSpace(artifact.URL) == "" {
			return FetchResult{}, fmt.Errorf("artifact %q has empty URL", artifact.Name)
		}
		name := safeDownloadName(firstNonEmpty(artifact.Name, filepath.Base(artifact.URL)))
		downloaded, err := DownloadURL(ctx, artifact.URL, filepath.Join(opts.OutputDir, name), opts)
		if err != nil {
			return FetchResult{}, err
		}
		result.Artifacts = append(result.Artifacts, downloaded)
	}
	return result, nil
}

func copyURL(ctx context.Context, dst io.Writer, rawURL string, opts DownloadOptions) (int64, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return 0, err
	}
	switch parsed.Scheme {
	case "file":
		return copyFileURL(dst, parsed)
	case "":
		return copyLocalPath(dst, rawURL)
	case "http", "https":
		client := opts.Client
		if client == nil {
			client = http.DefaultClient
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return 0, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return 0, err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return 0, fmt.Errorf("download failed: %s", resp.Status)
		}
		return io.Copy(dst, resp.Body)
	default:
		return 0, fmt.Errorf("unsupported download URL scheme %q", parsed.Scheme)
	}
}

func copyFileURL(dst io.Writer, parsed *url.URL) (int64, error) {
	if parsed.Path == "" {
		return 0, errors.New("file URL path is empty")
	}
	return copyLocalPath(dst, parsed.Path)
}

func copyLocalPath(dst io.Writer, path string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return io.Copy(dst, f)
}

func safeDownloadName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == string(os.PathSeparator) {
		return "artifact.bin"
	}
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, string(os.PathSeparator), "_")
	return name
}
