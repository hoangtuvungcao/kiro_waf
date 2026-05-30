// Package static provides embedded web static assets (CSS, JS, images)
// for the Kiro WAF master server.
package static

import "embed"

// FS contains all static assets under web/static/ (css/, js/, img/).
// These are embedded into the binary at compile time.
//
//go:embed all:css all:img all:js
var FS embed.FS
