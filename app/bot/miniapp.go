package bot

import (
	_ "embed"
	"html/template"
	"net/http"
)

//go:embed miniapp/base.html
var baseTemplateContent string

//go:embed miniapp/index.html
var indexTemplateContent string

//go:embed miniapp/unauthorized.html
var unauthorizedTemplateContent string

// Pre-parsed templates
var (
	indexTemplate        = template.Must(template.New("index-page").Parse(baseTemplateContent + indexTemplateContent))
	unauthorizedTemplate = template.Must(template.New("unauthorized-page").Parse(baseTemplateContent + unauthorizedTemplateContent))
)

// MiniAppHandler serves the Telegram Mini App HTML page.
func MiniAppHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := indexTemplate.Execute(w, nil); err != nil {
		http.Error(w, "Error rendering template: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

// UnauthorizedHandler serves the authentication form for unauthorized users
func UnauthorizedHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)

	if err := unauthorizedTemplate.Execute(w, nil); err != nil {
		http.Error(w, "Error rendering template: "+err.Error(), http.StatusInternalServerError)
		return
	}
}
