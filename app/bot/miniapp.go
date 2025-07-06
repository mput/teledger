package bot

import (
	_ "embed"
	"net/http"
)

//go:embed miniapp/index.html
var miniAppHTML []byte

// MiniAppHandler serves the Telegram Mini App HTML page.
func MiniAppHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(miniAppHTML)
}
