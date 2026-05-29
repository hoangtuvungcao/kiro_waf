package handlers

import (
	"net/http"

	"kiro_waf/master-server/templates"
)

// HandleHomepage returns an http.HandlerFunc that serves the public homepage.
// The homepage displays Kiro branding, protection service description, and
// contact information. It contains NO references to /admin or /api/ endpoints
// and NO JavaScript that could expose backend information.
func HandleHomepage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only serve the homepage at the exact root path.
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
		w.Write(templates.HomepageHTML)
	}
}
