package bot

import (
	_ "embed"
	"html/template"
	"log/slog"
	"net/http"
	"os"
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

// AuthMiddleware checks for sid cookie and redirects to unauthorized page if not present
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if os.Getenv("DEV_MODE_SKIP_AUTH") != "" {
			next(w, r)
			return
		}

		_, err := r.Cookie("sid")
		if err != nil {
			UnauthorizedHandler(w, r)
			return
		}
		next(w, r)
	}
}

// AuthHandler sets the sid cookie with Telegram WebApp initData
func AuthHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	initData := r.PostForm.Get("initData")

	slog.Info("miniapp auth request", "has_init_data", initData != "")

	if initData == "" {
		http.Error(w, "Missing initData", http.StatusBadRequest)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "sid",
		Value:    initData,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("HX-Location", r.Header.Get("HX-Current-URL"))
	w.WriteHeader(http.StatusNoContent)
}

func (bot *Bot) NewMiniAppMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc(MiniAppRoutePath+"/auth", AuthHandler)
	mux.HandleFunc(MiniAppRoutePath, AuthMiddleware(MiniAppHandler))
	mux.HandleFunc(MiniAppRoutePath+"/statistics", AuthMiddleware(StatisticsHandler))
	return mux
}
