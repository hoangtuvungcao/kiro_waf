package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"kiro_waf/internal/master/db"
)

// releasesDir là thư mục chứa các file binary đã build sẵn.
const releasesDir = "/usr/local/share/kiro/releases"

// downloadInfoResponse chứa thông tin phiên bản và checksum.
type downloadInfoResponse struct {
	Version      string `json:"version"`
	ClientSHA256 string `json:"client_sha256"`
	XDPSHA256    string `json:"xdp_sha256"`
}

// RegisterDownloadRoutes đăng ký các route download cho client binary.
// Các route này yêu cầu license key hợp lệ trong header X-License-Key.
func RegisterDownloadRoutes(mux *http.ServeMux, database *db.DB) {
	mux.HandleFunc("/api/v1/download/client-waf", handleDownloadClientWAF(database))
	mux.HandleFunc("/api/v1/download/xdp-filter", handleDownloadXDPFilter(database))
	mux.HandleFunc("/api/v1/download/info", handleDownloadInfo(database))
}

// validateLicenseKey kiểm tra license key từ header X-License-Key.
// Trả về true nếu license hợp lệ và đang active.
func validateLicenseKey(r *http.Request, database *db.DB) bool {
	key := r.Header.Get("X-License-Key")
	if key == "" {
		return false
	}

	ctx := r.Context()
	license, err := database.GetLicenseByKeyCtx(ctx, key)
	if err != nil {
		log.Printf("download: db error validating license: %v", err)
		return false
	}

	if license == nil {
		return false
	}

	// Kiểm tra license đang active và chưa hết hạn.
	// Community plan có ExpiresAt = zero (vô thời hạn) → luôn hợp lệ.
	now := time.Now().UTC()
	return license.Status == "active" && (license.ExpiresAt.IsZero() || license.ExpiresAt.After(now))
}

// handleDownloadClientWAF phục vụ file binary kiro-client-waf.
// GET /api/v1/download/client-waf
// Yêu cầu: X-License-Key header hợp lệ.
func handleDownloadClientWAF(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if !validateLicenseKey(r, database) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "invalid or expired license key",
			})
			return
		}

		filePath := filepath.Join(releasesDir, "kiro-client-waf")
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "binary not available",
			})
			return
		}

		// Ghi log download event
		clientIP := extractClientIP(r)
		licenseKey := r.Header.Get("X-License-Key")
		log.Printf("download: client-waf requested by license=%s ip=%s", licenseKey[:8]+"...", clientIP)

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", "attachment; filename=kiro-client-waf")
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFile(w, r, filePath)
	}
}

// handleDownloadXDPFilter phục vụ file XDP object.
// GET /api/v1/download/xdp-filter
// Yêu cầu: X-License-Key header hợp lệ.
func handleDownloadXDPFilter(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if !validateLicenseKey(r, database) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "invalid or expired license key",
			})
			return
		}

		filePath := filepath.Join(releasesDir, "xdp_filter.o")
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "xdp object not available",
			})
			return
		}

		// Ghi log download event
		clientIP := extractClientIP(r)
		licenseKey := r.Header.Get("X-License-Key")
		log.Printf("download: xdp-filter requested by license=%s ip=%s", licenseKey[:8]+"...", clientIP)

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", "attachment; filename=xdp_filter.o")
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFile(w, r, filePath)
	}
}

// handleDownloadInfo trả về thông tin phiên bản và SHA-256 checksum.
// GET /api/v1/download/info
// Yêu cầu: X-License-Key header hợp lệ.
func handleDownloadInfo(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if !validateLicenseKey(r, database) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "invalid or expired license key",
			})
			return
		}

		// Đọc version từ file VERSION nếu có
		version := "1.0.0"
		versionFile := filepath.Join(releasesDir, "VERSION")
		if data, err := os.ReadFile(versionFile); err == nil {
			v := string(data)
			// Trim whitespace
			for len(v) > 0 && (v[len(v)-1] == '\n' || v[len(v)-1] == '\r' || v[len(v)-1] == ' ') {
				v = v[:len(v)-1]
			}
			if v != "" {
				version = v
			}
		}

		// Tính SHA-256 cho client binary
		clientSHA256 := computeFileSHA256(filepath.Join(releasesDir, "kiro-client-waf"))

		// Tính SHA-256 cho XDP object
		xdpSHA256 := computeFileSHA256(filepath.Join(releasesDir, "xdp_filter.o"))

		// Ghi log
		clientIP := extractClientIP(r)
		licenseKey := r.Header.Get("X-License-Key")
		log.Printf("download: info requested by license=%s ip=%s", licenseKey[:8]+"...", clientIP)

		writeJSON(w, http.StatusOK, downloadInfoResponse{
			Version:      version,
			ClientSHA256: clientSHA256,
			XDPSHA256:    xdpSHA256,
		})
	}
}

// computeFileSHA256 tính SHA-256 hash của file. Trả về chuỗi rỗng nếu lỗi.
func computeFileSHA256(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}
