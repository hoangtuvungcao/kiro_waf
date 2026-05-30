// Package admin cung cấp embedded HTML templates và hàm render cho admin dashboard.
// Tất cả templates được embed trực tiếp vào binary (không CDN), thiết kế tông tối,
// responsive, và typography dễ đọc.
package admin

import (
	"embed"
	"html/template"
	"io"
	"net/http"
	"time"

	"kiro_waf/internal/master/models"
)

//go:embed *.html
var templateFS embed.FS

// pageTemplates maps page names to their parsed template sets.
var pageTemplates map[string]*template.Template

var funcMap = template.FuncMap{
	"slice": func(s string, start, end int) string {
		if start >= len(s) {
			return ""
		}
		if end > len(s) {
			end = len(s)
		}
		return s[start:end]
	},
}

func init() {
	pageTemplates = make(map[string]*template.Template)

	pages := []string{
		"dashboard.html",
		"licenses.html",
		"license_form.html",
		"releases.html",
		"release_form.html",
		"heartbeats.html",
	}

	for _, page := range pages {
		t := template.Must(
			template.New("").Funcs(funcMap).ParseFS(templateFS, "layout.html", page),
		)
		pageTemplates[page] = t
	}
}

// DashboardStats holds aggregate statistics for the dashboard overview.
type DashboardStats struct {
	TotalLicenses      int
	ActiveLicenses     int
	SuspendedLicenses  int
	RevokedLicenses    int
	ExpiredLicenses    int
	ActiveNodes        int
	RecentHeartbeats   int
	SystemStatus       string // "healthy", "degraded", "error"
	SystemStatusDetail string
}

// PageData is the base data structure passed to all admin templates.
type PageData struct {
	Title        string
	Active       string // active nav item: "dashboard", "licenses", "releases", "heartbeats"
	FlashSuccess string
	FlashError   string
	FlashWarning string
	FlashInfo    string
}

// DashboardData holds data for the dashboard overview page.
type DashboardData struct {
	PageData
	Stats            DashboardStats
	RecentHeartbeats []models.Heartbeat
}

// LicensesData holds data for the license list page.
type LicensesData struct {
	PageData
	Licenses []models.License
}

// LicenseFormData holds data for the license create/edit form.
type LicenseFormData struct {
	PageData
	IsEdit  bool
	License models.License
}

// ReleasesData holds data for the releases list page.
type ReleasesData struct {
	PageData
	Releases []models.Release
}

// ReleaseFormData holds data for the release create form.
type ReleaseFormData struct {
	PageData
}

// HeartbeatsData holds data for the heartbeat log page.
type HeartbeatsData struct {
	PageData
	Heartbeats []models.Heartbeat
}

// RenderDashboard renders the admin dashboard overview page.
func RenderDashboard(w io.Writer, data *DashboardData) error {
	data.Active = "dashboard"
	if data.Title == "" {
		data.Title = "Tổng quan"
	}
	return pageTemplates["dashboard.html"].ExecuteTemplate(w, "layout", data)
}

// RenderLicenses renders the license list page.
func RenderLicenses(w io.Writer, data *LicensesData) error {
	data.Active = "licenses"
	if data.Title == "" {
		data.Title = "Quản lý License"
	}
	return pageTemplates["licenses.html"].ExecuteTemplate(w, "layout", data)
}

// RenderLicenseForm renders the license create/edit form.
func RenderLicenseForm(w io.Writer, data *LicenseFormData) error {
	data.Active = "licenses"
	if data.Title == "" {
		if data.IsEdit {
			data.Title = "Sửa License"
		} else {
			data.Title = "Tạo License"
		}
	}
	return pageTemplates["license_form.html"].ExecuteTemplate(w, "layout", data)
}

// RenderReleases renders the releases list page.
func RenderReleases(w io.Writer, data *ReleasesData) error {
	data.Active = "releases"
	if data.Title == "" {
		data.Title = "Quản lý Phát Hành"
	}
	return pageTemplates["releases.html"].ExecuteTemplate(w, "layout", data)
}

// RenderReleaseForm renders the release create form.
func RenderReleaseForm(w io.Writer, data *ReleaseFormData) error {
	data.Active = "releases"
	if data.Title == "" {
		data.Title = "Đăng Phát Hành"
	}
	return pageTemplates["release_form.html"].ExecuteTemplate(w, "layout", data)
}

// RenderHeartbeats renders the heartbeat log page.
func RenderHeartbeats(w io.Writer, data *HeartbeatsData) error {
	data.Active = "heartbeats"
	if data.Title == "" {
		data.Title = "Heartbeat Log"
	}
	return pageTemplates["heartbeats.html"].ExecuteTemplate(w, "layout", data)
}

// FlashFromRequest extracts flash message query parameters from the request URL.
// Supported params: flash_success, flash_error, flash_warning, flash_info.
func FlashFromRequest(r *http.Request) PageData {
	return PageData{
		FlashSuccess: r.URL.Query().Get("flash_success"),
		FlashError:   r.URL.Query().Get("flash_error"),
		FlashWarning: r.URL.Query().Get("flash_warning"),
		FlashInfo:    r.URL.Query().Get("flash_info"),
	}
}

// RedirectWithFlash returns a redirect URL with a flash message query parameter.
func RedirectWithFlash(baseURL, flashType, message string) string {
	return baseURL + "?" + flashType + "=" + template.URLQueryEscaper(message)
}

// IsRecentHeartbeat returns true if the heartbeat was received within the given duration.
func IsRecentHeartbeat(t time.Time, within time.Duration) bool {
	return time.Since(t) <= within
}
