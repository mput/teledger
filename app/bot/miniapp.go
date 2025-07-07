package bot

import (
	_ "embed"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"time"
)

const (
	MiniAppRoutePath = "/bot/miniapp"
)

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// LoggingMiddleware logs HTTP requests and responses
func LoggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap the response writer to capture status code
		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // default status
		}

		// Log request
		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
			"user_agent", r.UserAgent(),
		)

		// Call the next handler
		next(wrapped, r)

		// Log response
		duration := time.Since(start)
		slog.Info("http response",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.statusCode,
			"duration_ms", duration.Milliseconds(),
		)
	}
}

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
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
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

// NewMiniAppHandler returns the mux wrapped with logging middleware
func (bot *Bot) NewMiniAppHandler() http.Handler {
	mux := bot.NewMiniAppMux()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		LoggingMiddleware(mux.ServeHTTP)(w, r)
	})
}
