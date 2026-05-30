# Web Assets

This directory contains web assets (HTML templates, CSS, JavaScript, images) for the Kiro WAF Admin UI and public-facing pages.

## Structure

```
web/
├── templates/       # HTML templates (Go html/template)
│   ├── admin/       # Admin dashboard templates
│   └── *.html       # Public page templates (homepage, challenge, etc.)
├── static/
│   ├── css/         # Stylesheets (kiro.css, kiro-brand.css)
│   ├── js/          # JavaScript (chart.min.js, kiro-charts.js)
│   └── img/         # Images (kiro-logo.svg, favicon.svg)
└── README.md
```

## Notes

- All CSS is served from a single unified file (`kiro.css`, <100KB uncompressed)
- No external CDN dependencies — all assets are bundled locally
- Chart.js is bundled at `static/js/chart.min.js` (<50KB gzipped)
- Templates use Go's `html/template` package with `embed.FS`
