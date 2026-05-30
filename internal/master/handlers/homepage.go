package handlers

import (
	"net/http"

	"kiro_waf/internal/master/templates"
)

// HandleHomepage returns an http.HandlerFunc that serves the public homepage.
// The homepage displays Kiro branding, protection service description, and
// contact information. It contains NO references to /admin or /api/ endpoints
// and NO JavaScript that could expose backend information.
// For non-root paths that don't match any other route, it serves a branded 404 page.
func HandleHomepage() http.HandlerFunc {
	notFound := HandleNotFound()

	return func(w http.ResponseWriter, r *http.Request) {
		// Serve branded 404 for any path that isn't exactly "/".
		if r.URL.Path != "/" {
			notFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
		w.Write(templates.HomepageHTML)
	}
}
