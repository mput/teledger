package bot

import (
	"fmt"
	"net/http"
)

// MiniAppHandler serves a simple helloworld HTML page for the Telegram Mini App.
func MiniAppHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, `<!DOCTYPE html><html><head><title>Hello</title></head><body><h1>hello world!</h1></body></html>`)
}
