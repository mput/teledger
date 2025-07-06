package bot

import (
	"html/template"
	"net/http"
	"path/filepath"
)

// MiniAppHandler serves the Telegram Mini App HTML page.
func MiniAppHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Parse base template first, then the specific page template
	tmpl, err := template.ParseFiles(
		filepath.Join("app", "bot", "miniapp", "base.html"),
		filepath.Join("app", "bot", "miniapp", "index.html"),
	)
	if err != nil {
		http.Error(w, "Error loading templates: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tmpl.ExecuteTemplate(w, "base", nil); err != nil {
		http.Error(w, "Error rendering template: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

// UnauthorizedHandler serves the authentication form for unauthorized users
func UnauthorizedHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)

	// Parse base template first, then the specific page template
	tmpl, err := template.ParseFiles(
		filepath.Join("app", "bot", "miniapp", "base.html"),
		filepath.Join("app", "bot", "miniapp", "unauthorized.html"),
	)
	if err != nil {
		http.Error(w, "Error loading templates: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tmpl.ExecuteTemplate(w, "base", nil); err != nil {
		http.Error(w, "Error rendering template: "+err.Error(), http.StatusInternalServerError)
		return
	}
}
