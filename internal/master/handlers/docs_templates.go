package handlers

// docsIndexHTML is the main documentation page template with sidebar navigation
// and language switcher. It uses the Kiro brand design tokens for consistent styling.
const docsIndexHTML = `<!DOCTYPE html>
<html lang="{{.Lang}}">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <link rel="icon" href="/static/img/favicon.svg" type="image/svg+xml">
    <link rel="stylesheet" href="/static/css/kiro.css">
    <style>
        .docs-layout {
            display: flex;
            min-height: 100vh;
            background: var(--kiro-background, #0f0f1a);
            color: var(--kiro-text-primary, #f0f0f0);
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
        }

        .docs-sidebar {
            width: 280px;
            min-width: 280px;
            background: var(--kiro-surface, #1a1a2e);
            border-right: 1px solid var(--kiro-border, #2a2a3e);
            padding: 1.5rem 0;
            position: fixed;
            top: 0;
            left: 0;
            bottom: 0;
            overflow-y: auto;
            z-index: 100;
        }

        .docs-sidebar-header {
            padding: 0 1.5rem 1.5rem;
            border-bottom: 1px solid var(--kiro-border, #2a2a3e);
            margin-bottom: 1rem;
        }

        .docs-sidebar-logo {
            display: flex;
            align-items: center;
            gap: 0.75rem;
            text-decoration: none;
            color: var(--kiro-text-primary, #f0f0f0);
        }

        .docs-sidebar-logo img {
            width: 32px;
            height: 32px;
        }

        .docs-sidebar-logo span {
            font-size: 1.1rem;
            font-weight: 600;
        }

        .docs-nav-section {
            margin-bottom: 1.5rem;
        }

        .docs-nav-section-title {
            padding: 0.5rem 1.5rem;
            font-size: 0.75rem;
            font-weight: 700;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            color: var(--kiro-text-secondary, #a0a0b0);
        }

        .docs-nav-item {
            display: block;
            padding: 0.5rem 1.5rem 0.5rem 2rem;
            color: var(--kiro-text-secondary, #a0a0b0);
            text-decoration: none;
            font-size: 0.9rem;
            transition: color 0.2s, background 0.2s, border-left 0.2s;
            border-left: 3px solid transparent;
        }

        .docs-nav-item:hover {
            color: var(--kiro-text-primary, #f0f0f0);
            background: rgba(13, 148, 136, 0.1);
        }

        .docs-nav-item.active {
            color: var(--kiro-primary, #0d9488);
            border-left-color: var(--kiro-primary, #0d9488);
            background: rgba(13, 148, 136, 0.08);
        }

        .docs-main {
            flex: 1;
            margin-left: 280px;
            padding: 2rem 3rem;
            max-width: 900px;
        }

        .docs-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 2rem;
            padding-bottom: 1rem;
            border-bottom: 1px solid var(--kiro-border, #2a2a3e);
        }

        .docs-header h1 {
            font-size: 1.75rem;
            font-weight: 700;
            color: var(--kiro-text-primary, #f0f0f0);
            margin: 0;
        }

        .lang-switcher {
            display: flex;
            gap: 0.25rem;
            background: var(--kiro-surface, #1a1a2e);
            border: 1px solid var(--kiro-border, #2a2a3e);
            border-radius: 6px;
            padding: 0.25rem;
        }

        .lang-switcher a {
            padding: 0.4rem 0.75rem;
            border-radius: 4px;
            text-decoration: none;
            font-size: 0.85rem;
            font-weight: 500;
            color: var(--kiro-text-secondary, #a0a0b0);
            transition: background 0.2s, color 0.2s;
        }

        .lang-switcher a:hover {
            color: var(--kiro-text-primary, #f0f0f0);
        }

        .lang-switcher a.active {
            background: var(--kiro-primary, #0d9488);
            color: #fff;
        }

        .docs-content {
            line-height: 1.7;
        }

        .docs-content h2 {
            font-size: 1.4rem;
            margin-top: 2rem;
            margin-bottom: 0.75rem;
            color: var(--kiro-text-primary, #f0f0f0);
        }

        .docs-content p {
            margin-bottom: 1rem;
            color: var(--kiro-text-secondary, #a0a0b0);
        }

        .docs-welcome-card {
            background: var(--kiro-surface, #1a1a2e);
            border: 1px solid var(--kiro-border, #2a2a3e);
            border-radius: 8px;
            padding: 2rem;
            margin-bottom: 2rem;
            backdrop-filter: blur(12px);
        }

        .docs-welcome-card h2 {
            margin-top: 0;
            color: var(--kiro-primary, #0d9488);
        }

        .docs-footer {
            margin-top: 3rem;
            padding-top: 1.5rem;
            border-top: 1px solid var(--kiro-border, #2a2a3e);
        }

        .docs-version {
            font-size: 0.8rem;
            color: var(--kiro-text-secondary, #a0a0b0);
        }

        .docs-content h3 {
            font-size: 1.15rem;
            margin-top: 1.5rem;
            margin-bottom: 0.5rem;
            color: var(--kiro-text-primary, #f0f0f0);
        }

        .docs-content code {
            background: rgba(13, 148, 136, 0.1);
            border: 1px solid var(--kiro-border, #2a2a3e);
            border-radius: 4px;
            padding: 0.15rem 0.4rem;
            font-size: 0.85rem;
            font-family: 'JetBrains Mono', 'Fira Code', monospace;
            color: var(--kiro-accent, #14b8a6);
        }

        .docs-content pre {
            background: var(--kiro-surface, #1a1a2e);
            border: 1px solid var(--kiro-border, #2a2a3e);
            border-radius: 8px;
            padding: 1rem 1.25rem;
            overflow-x: auto;
            margin-bottom: 1.5rem;
        }

        .docs-content pre code {
            background: none;
            border: none;
            padding: 0;
            font-size: 0.85rem;
            color: var(--kiro-text-primary, #f0f0f0);
        }

        .docs-content table {
            width: 100%;
            border-collapse: collapse;
            margin-bottom: 1.5rem;
            font-size: 0.9rem;
        }

        .docs-content th {
            background: var(--kiro-surface, #1a1a2e);
            border: 1px solid var(--kiro-border, #2a2a3e);
            padding: 0.6rem 0.8rem;
            text-align: left;
            font-weight: 600;
            color: var(--kiro-text-primary, #f0f0f0);
        }

        .docs-content td {
            border: 1px solid var(--kiro-border, #2a2a3e);
            padding: 0.6rem 0.8rem;
            color: var(--kiro-text-secondary, #a0a0b0);
        }

        .docs-content ul, .docs-content ol {
            margin-bottom: 1rem;
            padding-left: 1.5rem;
            color: var(--kiro-text-secondary, #a0a0b0);
        }

        .docs-content li {
            margin-bottom: 0.4rem;
        }

        .docs-content strong {
            color: var(--kiro-text-primary, #f0f0f0);
        }

        @media (max-width: 768px) {
            .docs-sidebar {
                position: relative;
                width: 100%;
                min-width: unset;
                border-right: none;
                border-bottom: 1px solid var(--kiro-border, #2a2a3e);
            }

            .docs-layout {
                flex-direction: column;
            }

            .docs-main {
                margin-left: 0;
                padding: 1.5rem 1rem;
            }
        }
    </style>
</head>
<body>
    <div class="docs-layout">
        <aside class="docs-sidebar">
            <div class="docs-sidebar-header">
                <a href="/docs" class="docs-sidebar-logo">
                    <img src="/static/img/kiro-logo.svg" alt="Kiro WAF">
                    <span>Kiro WAF Docs</span>
                </a>
            </div>
            <nav>
                {{range .Sections}}
                <div class="docs-nav-section">
                    <div class="docs-nav-section-title">{{.Title}}</div>
                    {{range .Items}}
                    <a href="{{.Path}}" class="docs-nav-item{{if .Active}} active{{end}}">{{.Title}}</a>
                    {{end}}
                </div>
                {{end}}
            </nav>
        </aside>
        <main class="docs-main">
            <div class="docs-header">
                <h1>{{.Title}}</h1>
                <div class="lang-switcher">
                    {{range .Languages}}
                    <a href="/docs/{{.Code}}" class="{{if .Active}}active{{end}}">{{.Label}}</a>
                    {{end}}
                </div>
            </div>
            <div class="docs-content">
                {{if .Content}}
                {{.Content}}
                {{else if eq .Lang "en"}}
                <div class="docs-welcome-card">
                    <h2>Welcome to Kiro WAF Documentation</h2>
                    <p>Kiro WAF is a high-performance Web Application Firewall combining XDP/eBPF packet filtering with a Go reverse proxy for comprehensive DDoS protection.</p>
                    <p>Use the sidebar navigation to browse installation guides, configuration reference, troubleshooting tips, and frequently asked questions.</p>
                </div>
                {{else}}
                <div class="docs-welcome-card">
                    <h2>Chào Mừng Đến Với Tài Liệu Kiro WAF</h2>
                    <p>Kiro WAF là tường lửa ứng dụng web hiệu năng cao kết hợp lọc gói tin XDP/eBPF với reverse proxy Go để bảo vệ DDoS toàn diện.</p>
                    <p>Sử dụng thanh điều hướng bên trái để duyệt hướng dẫn cài đặt, tham chiếu cấu hình, mẹo xử lý sự cố, và câu hỏi thường gặp.</p>
                </div>
                {{end}}
            </div>
            <footer class="docs-footer">
                <span class="docs-version">{{if eq .Lang "en"}}Version: {{.Version}} | Last updated: {{.LastUpdated}}{{else}}Phiên bản: {{.Version}} | Cập nhật: {{.LastUpdated}}{{end}}</span>
            </footer>
        </main>
    </div>
</body>
</html>`

// docsErrorHTML is the custom error page template displayed when documentation
// content is unavailable. This satisfies requirement 10.7 which mandates a
// custom error page instead of a generic server error.
const docsErrorHTML = `<!DOCTYPE html>
<html lang="{{.Lang}}">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <link rel="icon" href="/static/img/favicon.svg" type="image/svg+xml">
    <link rel="stylesheet" href="/static/css/kiro.css">
    <style>
        .docs-error-page {
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: center;
            min-height: 100vh;
            background: var(--kiro-background, #0f0f1a);
            color: var(--kiro-text-primary, #f0f0f0);
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            padding: 2rem;
            text-align: center;
        }

        .docs-error-card {
            background: var(--kiro-surface, #1a1a2e);
            border: 1px solid var(--kiro-border, #2a2a3e);
            border-radius: 12px;
            padding: 3rem;
            max-width: 500px;
            width: 100%;
            backdrop-filter: blur(12px);
        }

        .docs-error-icon {
            font-size: 3rem;
            margin-bottom: 1.5rem;
        }

        .docs-error-card h1 {
            font-size: 1.5rem;
            margin-bottom: 1rem;
            color: var(--kiro-warning, #f59e0b);
        }

        .docs-error-card p {
            color: var(--kiro-text-secondary, #a0a0b0);
            line-height: 1.6;
            margin-bottom: 1.5rem;
        }

        .docs-error-back {
            display: inline-block;
            padding: 0.75rem 1.5rem;
            background: var(--kiro-primary, #0d9488);
            color: #fff;
            text-decoration: none;
            border-radius: 6px;
            font-weight: 500;
            transition: opacity 0.2s;
        }

        .docs-error-back:hover {
            opacity: 0.9;
        }

        .lang-switcher {
            display: flex;
            gap: 0.25rem;
            background: var(--kiro-surface, #1a1a2e);
            border: 1px solid var(--kiro-border, #2a2a3e);
            border-radius: 6px;
            padding: 0.25rem;
            margin-bottom: 2rem;
        }

        .lang-switcher a {
            padding: 0.4rem 0.75rem;
            border-radius: 4px;
            text-decoration: none;
            font-size: 0.85rem;
            font-weight: 500;
            color: var(--kiro-text-secondary, #a0a0b0);
            transition: background 0.2s, color 0.2s;
        }

        .lang-switcher a.active {
            background: var(--kiro-primary, #0d9488);
            color: #fff;
        }
    </style>
</head>
<body>
    <div class="docs-error-page">
        <div class="lang-switcher">
            {{range .Languages}}
            <a href="/docs/{{.Code}}" class="{{if .Active}}active{{end}}">{{.Label}}</a>
            {{end}}
        </div>
        <div class="docs-error-card">
            <div class="docs-error-icon">📄</div>
            <h1>{{.Title}}</h1>
            <p>{{.ErrorMessage}}</p>
            <a href="/" class="docs-error-back">{{if eq .Lang "en"}}Back to Home{{else}}Về Trang Chủ{{end}}</a>
        </div>
    </div>
</body>
</html>`
