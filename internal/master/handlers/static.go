package handlers

import (
	"net/http"

	webstatic "kiro_waf/web/static"
)

// RegisterStaticRoutes registers the /static/ file server route on the given mux.
// Static assets (CSS, JS, images) are embedded in the binary and served at /static/.
func RegisterStaticRoutes(mux *http.ServeMux) {
	fileServer := http.FileServer(http.FS(webstatic.FS))
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))
}
