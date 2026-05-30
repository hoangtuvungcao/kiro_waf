package handlers

import (
	"html/template"
	"net/http"
	"strings"
	"time"

	"kiro_waf/pkg/version"
)

// DocsHandler serves the public documentation site at /docs.
// It provides a sidebar navigation with table of contents,
// a language switcher (Vietnamese + English), and custom error pages.
type DocsHandler struct {
	languages     []string
	defaultLang   string
	indexTemplate *template.Template
	errorTemplate *template.Template
	contentPages  map[string]map[string]DocsContentPage // [lang][pageID]
}

// DocsPageData holds the data passed to the docs page template.
type DocsPageData struct {
	Lang          string
	Languages     []LangOption
	Sections      []DocsSection
	Title         string
	Content       template.HTML
	LastUpdated   string
	Version       string
	CurrentPath   string
	IsError       bool
	ErrorMessage  string
	PageID        string
}

// DocsContentPage represents a single documentation content page.
type DocsContentPage struct {
	Title   string
	Content template.HTML
}

// LangOption represents a language option in the switcher.
type LangOption struct {
	Code   string
	Label  string
	Active bool
}

// DocsSection represents a section in the sidebar navigation.
type DocsSection struct {
	ID    string
	Title string
	Items []DocsSectionItem
}

// DocsSectionItem represents a single item in a docs section.
type DocsSectionItem struct {
	ID     string
	Title  string
	Path   string
	Active bool
}

// NewDocsHandler creates a new DocsHandler with the documentation templates.
func NewDocsHandler() *DocsHandler {
	h := &DocsHandler{
		languages:   []string{"vi", "en"},
		defaultLang: "vi",
	}

	h.indexTemplate = template.Must(template.New("docs").Parse(docsIndexHTML))
	h.errorTemplate = template.Must(template.New("docs-error").Parse(docsErrorHTML))
	h.contentPages = buildContentPages()

	return h
}

// RegisterDocsRoutes registers the /docs routes on the given mux.
func RegisterDocsRoutes(mux *http.ServeMux) {
	handler := NewDocsHandler()
	mux.HandleFunc("/docs", handler.HandleDocs())
	mux.HandleFunc("/docs/", handler.HandleDocs())
}

// HandleDocs returns an http.HandlerFunc that serves the documentation site.
func (h *DocsHandler) HandleDocs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse language from path: /docs/{lang}/...
		lang := h.parseLang(r.URL.Path)

		// Build page data.
		data := h.buildPageData(lang, r.URL.Path)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.Header().Set("X-Content-Type-Options", "nosniff")

		if err := h.indexTemplate.Execute(w, data); err != nil {
			// If template execution fails, serve the error page.
			h.serveErrorPage(w, lang)
		}
	}
}

// serveErrorPage renders a custom error page when docs are unavailable.
func (h *DocsHandler) serveErrorPage(w http.ResponseWriter, lang string) {
	data := DocsPageData{
		Lang:      lang,
		Languages: h.buildLangOptions(lang),
		IsError:   true,
	}

	if lang == "en" {
		data.ErrorMessage = "Documentation is temporarily unavailable. Please try again later."
		data.Title = "Documentation Unavailable"
	} else {
		data.ErrorMessage = "Tài liệu tạm thời không khả dụng. Vui lòng thử lại sau."
		data.Title = "Tài Liệu Không Khả Dụng"
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	h.errorTemplate.Execute(w, data)
}

// HandleDocsUnavailable returns a handler that serves the custom error page
// when documentation content is not available.
func (h *DocsHandler) HandleDocsUnavailable() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lang := h.parseLang(r.URL.Path)
		h.serveErrorPage(w, lang)
	}
}

// parseLang extracts the language code from the URL path.
// Expected paths: /docs, /docs/, /docs/vi, /docs/en, /docs/vi/section
func (h *DocsHandler) parseLang(path string) string {
	// Strip /docs prefix.
	path = strings.TrimPrefix(path, "/docs")
	path = strings.TrimPrefix(path, "/")

	if path == "" {
		return h.defaultLang
	}

	// First segment is the language code.
	parts := strings.SplitN(path, "/", 2)
	for _, lang := range h.languages {
		if parts[0] == lang {
			return lang
		}
	}

	return h.defaultLang
}

// parsePageID extracts the page identifier from the URL path.
// Expected paths: /docs/en/quick-start -> "quick-start"
func (h *DocsHandler) parsePageID(path string) string {
	path = strings.TrimPrefix(path, "/docs")
	path = strings.TrimPrefix(path, "/")

	if path == "" {
		return ""
	}

	parts := strings.SplitN(path, "/", 2)
	// Check if first part is a language code.
	for _, lang := range h.languages {
		if parts[0] == lang {
			if len(parts) > 1 && parts[1] != "" {
				return parts[1]
			}
			return ""
		}
	}

	return ""
}

// buildLangOptions creates the language switcher options.
func (h *DocsHandler) buildLangOptions(activeLang string) []LangOption {
	labels := map[string]string{
		"vi": "Tiếng Việt",
		"en": "English",
	}

	options := make([]LangOption, 0, len(h.languages))
	for _, lang := range h.languages {
		options = append(options, LangOption{
			Code:   lang,
			Label:  labels[lang],
			Active: lang == activeLang,
		})
	}
	return options
}

// buildPageData constructs the full page data for the docs template.
func (h *DocsHandler) buildPageData(lang, path string) DocsPageData {
	pageID := h.parsePageID(path)
	sections := h.getSections(lang)

	// Mark active item in sidebar.
	for i := range sections {
		for j := range sections[i].Items {
			if sections[i].Items[j].ID == pageID {
				sections[i].Items[j].Active = true
			}
		}
	}

	data := DocsPageData{
		Lang:        lang,
		Languages:   h.buildLangOptions(lang),
		Sections:    sections,
		CurrentPath: path,
		LastUpdated: time.Now().Format("2006-01-02"),
		Version:     version.Short(),
		PageID:      pageID,
	}

	// Set title and content based on page.
	if pageID != "" {
		if langPages, ok := h.contentPages[lang]; ok {
			if page, ok := langPages[pageID]; ok {
				data.Title = page.Title
				data.Content = page.Content
			}
		}
	}

	// Default title if no page content found.
	if data.Title == "" {
		if lang == "en" {
			data.Title = "Kiro WAF Documentation"
		} else {
			data.Title = "Tài Liệu Kiro WAF"
		}
	}

	return data
}

// getSections returns the sidebar navigation sections for the given language.
func (h *DocsHandler) getSections(lang string) []DocsSection {
	if lang == "en" {
		return []DocsSection{
			{
				ID:    "getting-started",
				Title: "Getting Started",
				Items: []DocsSectionItem{
					{ID: "quick-start", Title: "Quick Start", Path: "/docs/en/quick-start"},
					{ID: "installation", Title: "Installation", Path: "/docs/en/installation"},
					{ID: "requirements", Title: "System Requirements", Path: "/docs/en/requirements"},
				},
			},
			{
				ID:    "configuration",
				Title: "Configuration",
				Items: []DocsSectionItem{
					{ID: "config-reference", Title: "Configuration Reference", Path: "/docs/en/config-reference"},
					{ID: "yaml-options", Title: "YAML Options", Path: "/docs/en/yaml-options"},
					{ID: "advanced-config", Title: "Advanced Configuration", Path: "/docs/en/advanced-config"},
				},
			},
			{
				ID:    "cli",
				Title: "CLI Reference",
				Items: []DocsSectionItem{
					{ID: "cli", Title: "CLI Overview", Path: "/docs/en/cli"},
					{ID: "cli/version", Title: "version", Path: "/docs/en/cli/version"},
					{ID: "cli/license", Title: "license", Path: "/docs/en/cli/license"},
					{ID: "cli/status", Title: "status", Path: "/docs/en/cli/status"},
					{ID: "cli/health", Title: "health", Path: "/docs/en/cli/health"},
					{ID: "cli/preflight", Title: "preflight", Path: "/docs/en/cli/preflight"},
					{ID: "cli/mode", Title: "mode", Path: "/docs/en/cli/mode"},
					{ID: "cli/install", Title: "install", Path: "/docs/en/cli/install"},
					{ID: "cli/update", Title: "update", Path: "/docs/en/cli/update"},
					{ID: "cli/incident", Title: "incident", Path: "/docs/en/cli/incident"},
					{ID: "cli/pilot", Title: "pilot", Path: "/docs/en/cli/pilot"},
					{ID: "cli/report", Title: "report", Path: "/docs/en/cli/report"},
				},
			},
			{
				ID:    "operation",
				Title: "Operation",
				Items: []DocsSectionItem{
					{ID: "monitoring", Title: "Monitoring", Path: "/docs/en/monitoring"},
					{ID: "updates", Title: "Updates & OTA", Path: "/docs/en/updates"},
					{ID: "backup", Title: "Backup & Recovery", Path: "/docs/en/backup"},
				},
			},
			{
				ID:    "troubleshooting",
				Title: "Troubleshooting",
				Items: []DocsSectionItem{
					{ID: "common-issues", Title: "Common Issues", Path: "/docs/en/common-issues"},
					{ID: "faq", Title: "FAQ", Path: "/docs/en/faq"},
				},
			},
		}
	}

	// Vietnamese (default)
	return []DocsSection{
		{
			ID:    "getting-started",
			Title: "Bắt Đầu",
			Items: []DocsSectionItem{
				{ID: "quick-start", Title: "Bắt Đầu Nhanh", Path: "/docs/vi/quick-start"},
				{ID: "installation", Title: "Cài Đặt", Path: "/docs/vi/installation"},
				{ID: "requirements", Title: "Yêu Cầu Hệ Thống", Path: "/docs/vi/requirements"},
			},
		},
		{
			ID:    "configuration",
			Title: "Cấu Hình",
			Items: []DocsSectionItem{
				{ID: "config-reference", Title: "Tham Chiếu Cấu Hình", Path: "/docs/vi/config-reference"},
				{ID: "yaml-options", Title: "Tùy Chọn YAML", Path: "/docs/vi/yaml-options"},
				{ID: "advanced-config", Title: "Cấu Hình Nâng Cao", Path: "/docs/vi/advanced-config"},
			},
		},
		{
			ID:    "cli",
			Title: "CLI Reference",
			Items: []DocsSectionItem{
				{ID: "cli", Title: "Tổng Quan CLI", Path: "/docs/vi/cli"},
				{ID: "cli/version", Title: "version", Path: "/docs/vi/cli/version"},
				{ID: "cli/license", Title: "license", Path: "/docs/vi/cli/license"},
				{ID: "cli/status", Title: "status", Path: "/docs/vi/cli/status"},
				{ID: "cli/health", Title: "health", Path: "/docs/vi/cli/health"},
				{ID: "cli/preflight", Title: "preflight", Path: "/docs/vi/cli/preflight"},
				{ID: "cli/mode", Title: "mode", Path: "/docs/vi/cli/mode"},
				{ID: "cli/install", Title: "install", Path: "/docs/vi/cli/install"},
				{ID: "cli/update", Title: "update", Path: "/docs/vi/cli/update"},
				{ID: "cli/incident", Title: "incident", Path: "/docs/vi/cli/incident"},
				{ID: "cli/pilot", Title: "pilot", Path: "/docs/vi/cli/pilot"},
				{ID: "cli/report", Title: "report", Path: "/docs/vi/cli/report"},
			},
		},
		{
			ID:    "operation",
			Title: "Vận Hành",
			Items: []DocsSectionItem{
				{ID: "monitoring", Title: "Giám Sát", Path: "/docs/vi/monitoring"},
				{ID: "updates", Title: "Cập Nhật & OTA", Path: "/docs/vi/updates"},
				{ID: "backup", Title: "Sao Lưu & Khôi Phục", Path: "/docs/vi/backup"},
			},
		},
		{
			ID:    "troubleshooting",
			Title: "Xử Lý Sự Cố",
			Items: []DocsSectionItem{
				{ID: "common-issues", Title: "Lỗi Thường Gặp", Path: "/docs/vi/common-issues"},
				{ID: "faq", Title: "Câu Hỏi Thường Gặp", Path: "/docs/vi/faq"},
			},
		},
	}
}
