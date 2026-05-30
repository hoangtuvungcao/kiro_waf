package handlers

import (
	"net/http"
)

// notFoundHTML is the branded 404 page HTML with inline styles using Kiro CSS variables.
var notFoundHTML = []byte(`<!DOCTYPE html>
<html lang="vi">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>404 - Kiro WAF</title>
<style>
:root {
  --kiro-primary: #6366f1;
  --kiro-surface: #1e1e2e;
  --kiro-text-primary: #e2e8f0;
  --kiro-text-secondary: #94a3b8;
  --kiro-bg: #0f0f1a;
  --kiro-border: #2d2d3f;
}
* { margin: 0; padding: 0; box-sizing: border-box; }
body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  background: var(--kiro-bg);
  color: var(--kiro-text-primary);
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
}
.container {
  text-align: center;
  padding: 2rem;
  max-width: 480px;
}
.logo {
  width: 64px;
  height: 64px;
  margin: 0 auto 2rem;
  background: var(--kiro-surface);
  border: 1px solid var(--kiro-border);
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
}
.logo svg {
  width: 36px;
  height: 36px;
}
.code {
  font-size: 6rem;
  font-weight: 700;
  color: var(--kiro-primary);
  line-height: 1;
  margin-bottom: 1rem;
}
.message {
  font-size: 1.25rem;
  color: var(--kiro-text-secondary);
  margin-bottom: 2rem;
}
.back-link {
  display: inline-block;
  color: var(--kiro-primary);
  text-decoration: none;
  font-size: 1rem;
  padding: 0.75rem 1.5rem;
  border: 1px solid var(--kiro-border);
  border-radius: 8px;
  background: var(--kiro-surface);
  transition: border-color 0.2s;
}
.back-link:hover {
  border-color: var(--kiro-primary);
}
</style>
</head>
<body>
<div class="container">
  <div class="logo">
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" stroke="#6366f1"/>
    </svg>
  </div>
  <div class="code">404</div>
  <p class="message">Trang kh&#244;ng t&#7891;n t&#7841;i</p>
  <a href="/" class="back-link">&larr; V&#7873; trang ch&#7911;</a>
</div>
</body>
</html>`)

// HandleNotFound returns an http.HandlerFunc that serves a branded 404 page.
func HandleNotFound() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusNotFound)
		w.Write(notFoundHTML)
	}
}
