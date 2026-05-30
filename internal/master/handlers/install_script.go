package handlers

import (
	"embed"
	"net/http"
)

//go:embed install_script_embed.sh
var installScriptFS embed.FS

// HandleInstallScript serves the client install script at GET /install.
// This allows users to install with a single command:
//
//	sudo bash -c "$(curl -fsSL https://firewall.vpsgen.com/install)" -- --license-key KIRO-XXXX
//
// Or download and run:
//
//	curl -fsSL https://firewall.vpsgen.com/install -o install.sh
//	sudo bash install.sh --license-key KIRO-XXXX
func HandleInstallScript() http.HandlerFunc {
	// Read the embedded script at startup.
	scriptContent, err := installScriptFS.ReadFile("install_script_embed.sh")
	if err != nil {
		// Fallback: serve a helpful error message as a shell script.
		scriptContent = []byte("#!/bin/bash\necho 'ERROR: Install script not available. Contact support@vpsgen.com'\nexit 1\n")
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
		w.Header().Set("Content-Disposition", "inline; filename=\"install-kiro-waf.sh\"")
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.WriteHeader(http.StatusOK)
		w.Write(scriptContent)
	}
}
