package bot

import (
	_ "embed"
	"html/template"
	"net/http"
)

const (
	MiniAppRoutePath = "/bot/miniapp"
)

//go:embed miniapp/base.html
var baseTemplateContent string

//go:embed miniapp/index.html
var indexTemplateContent string

//go:embed miniapp/unauthorized.html
var unauthorizedTemplateContent string

//go:embed miniapp/statistics.html
var statisticsTemplateContent string

// Pre-parsed templates
var (
	indexTemplate        = template.Must(template.New("index-page").Parse(baseTemplateContent + indexTemplateContent))
	unauthorizedTemplate = template.Must(template.New("unauthorized-page").Parse(baseTemplateContent + unauthorizedTemplateContent))
	statisticsTemplate   = template.Must(template.New("statistics-page").Parse(baseTemplateContent + statisticsTemplateContent))
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

// StatisticsHandler serves the statistics page
func StatisticsHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := statisticsTemplate.Execute(w, nil); err != nil {
		http.Error(w, "Error rendering template: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

func (bot *Bot) NewMiniAppMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc(MiniAppRoutePath, MiniAppHandler)
	mux.HandleFunc(MiniAppRoutePath+"/statistics", StatisticsHandler)
	return mux
}
