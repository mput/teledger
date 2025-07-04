package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/jessevdk/go-flags"
	"github.com/mput/teledger/app/bot"
	"golang.ngrok.com/ngrok/v2"
)

// injected with ldflags
var version = "dev"

func main() {
	fmt.Printf("teledger v:%s\n", version)
	opts := bot.Opts{}
	opts.Version = version
	_, err := flags.Parse(&opts)
	if err != nil {
		fmt.Printf("[ERROR] %v", err)
		os.Exit(1)
	}

	// Validate BaseURL is provided in production mode
	if !isDevelopmentMode() && opts.BaseURL == "" {
		fmt.Printf("[ERROR] BASE_URL is required in production mode")
		os.Exit(1)
	}

	nbot, err := bot.NewBot(&opts)
	if err != nil {
		slog.Error("unable to create bot", "err", err)
	}

	// Setup HTTP handler
	mux := http.NewServeMux()
	mux.HandleFunc("/bot/miniapp", bot.MiniAppHandler)

	// Check if we're in development mode and should use ngrok
	if isDevelopmentMode() {
		// In development, start ngrok first to get the URL, then start bot
		ngrokReady := make(chan error, 1)
		go func() {
			ngrokReady <- startNgrokServer(opts.Port, nbot, mux)
		}()

		// Wait for ngrok to be ready or fail
		if err := <-ngrokReady; err != nil {
			slog.Error("failed to start ngrok server", "err", err)
			os.Exit(1)
		}
	} else {
		// In production, start regular server
		go func() {
			startRegularServer(opts.Port, mux)
		}()
	}

	// TODO: handle signals
	err = nbot.Start()
	if err != nil {
		slog.Error("unable to start bot", "err", err)
	}
}

// isDevelopmentMode checks if we're running in development mode
func isDevelopmentMode() bool {
	devMode := os.Getenv("DEV_MODE")
	return strings.ToLower(devMode) == "true"
}

// startNgrokServer starts the HTTP server with ngrok tunnel
func startNgrokServer(port string, nbot *bot.Bot, handler http.Handler) error {
	ctx := context.Background()

	// Get ngrok authtoken from environment
	authtoken := os.Getenv("NGROK_AUTHTOKEN")
	if authtoken == "" {
		return fmt.Errorf("NGROK_AUTHTOKEN environment variable is required for development mode")
	}

	// Create ngrok agent first
	agent, err := ngrok.NewAgent(ngrok.WithAuthtoken(authtoken))
	if err != nil {
		return fmt.Errorf("failed to create ngrok agent: %w", err)
	}

	// Create listener using the agent
	listener, err := agent.Listen(ctx)
	if err != nil {
		return fmt.Errorf("failed to create ngrok listener: %w", err)
	}

	ngrokURL := listener.URL().String()
	slog.Info("ngrok tunnel established", "url", ngrokURL, "port", port)
	slog.Info("MiniApp available at", "url", ngrokURL+"/bot/miniapp")

	// Update bot's BaseURL to use ngrok URL
	nbot.SetBaseURL(ngrokURL)

	// ngrok is ready, start serving (this will block)
	go func() {
		err := http.Serve(listener, handler)
		if err != nil {
			slog.Error("ngrok HTTP server failed", "err", err)
		}
	}()

	return nil
}

// startRegularServer starts the HTTP server on localhost
func startRegularServer(port string, handler http.Handler) {
	slog.Info("HTTP server started", "port", port)
	err := http.ListenAndServe(":"+port, handler)
	if err != nil {
		slog.Error("HTTP server failed", "err", err)
	}
}
