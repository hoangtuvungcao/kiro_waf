// Package templates chứa embedded HTML templates cho Master_Server.
// Bao gồm: homepage, admin dashboard, challenge pages.
// Tất cả templates được embedded (không CDN), thiết kế tông tối nhất quán.
package templates

import "embed"

//go:embed homepage.html
var homepageFS embed.FS

// HomepageHTML holds the pre-read homepage content for fast serving.
// The homepage contains NO references to /admin or /api/ endpoints
// and NO JavaScript that could expose backend information.
var HomepageHTML []byte

func init() {
	data, err := homepageFS.ReadFile("homepage.html")
	if err != nil {
		panic("templates: failed to read embedded homepage.html: " + err.Error())
	}
	HomepageHTML = data
}
