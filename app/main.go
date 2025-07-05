package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jessevdk/go-flags"
	"github.com/lmittmann/tint"
	"github.com/mput/teledger/app/bot"
	"golang.ngrok.com/ngrok/v2"
)

// injected with ldflags
var version = "dev"

func main() {
	// Initialize colorful logging
	handler := tint.NewHandler(os.Stderr, &tint.Options{
		Level:      slog.LevelDebug,
		TimeFormat: "02.01.2006 15:04:05",
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)

	opts := bot.Opts{}
	opts.Version = version
	_, err := flags.Parse(&opts)
	if err != nil {
		slog.Error("flag parsing failed", "error", err)
		os.Exit(1)
	}

	slog.Info("teledger started", "version", version, "port", opts.Port, "dev_mode", isDevelopmentMode(), "base_url", opts.BaseURL, "github_repo", opts.Github.URL, "github_branch", opts.Github.Branch)

	// Validate BaseURL is provided in production mode
	if !isDevelopmentMode() && opts.BaseURL == "" {
		slog.Error("BASE_URL is required in production mode")
		os.Exit(1)
	}

	nbot, err := bot.NewBot(&opts)
	if err != nil {
		slog.Error("unable to create bot", "error", err)
		os.Exit(1)
	}

	mux := nbot.WebHandler()

	// Check if we're in development mode and should use ngrok
	if isDevelopmentMode() {
		// In development, start ngrok first to get the URL, then start bot
		ngrokReady := make(chan error, 1)
		go func() {
			ngrokReady <- startNgrokServer(opts.Port, nbot, mux, opts.BaseURL)
		}()

		// Wait for ngrok to be ready or fail
		if err := <-ngrokReady; err != nil {
			slog.Error("failed to start ngrok server", "error", err)
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
		slog.Error("unable to start bot", "error", err)
		os.Exit(1)
	}
}

// isDevelopmentMode checks if we're running in development mode
func isDevelopmentMode() bool {
	devMode := os.Getenv("DEV_MODE")
	return strings.EqualFold(devMode, "true")
}

// startNgrokServer starts the HTTP server with ngrok tunnel
func startNgrokServer(port string, nbot *bot.Bot, handler http.Handler, baseURL string) error {
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

	// Create listener using the agent with optional domain
	var listener ngrok.EndpointListener
	if baseURL != "" {
		// Extract domain from BASE_URL (remove protocol if present)
		domain := strings.TrimPrefix(strings.TrimPrefix(baseURL, "https://"), "http://")
		slog.Info("starting ngrok with custom domain", "domain", domain)
		listener, err = agent.Listen(ctx, ngrok.WithURL(domain))
	} else {
		listener, err = agent.Listen(ctx)
	}

	if err != nil {
		return fmt.Errorf("failed to create ngrok listener: %w", err)
	}

	ngrokURL := listener.URL().String()
	slog.Info("ngrok tunnel established", "url", ngrokURL, "port", port)

	// Update bot's BaseURL to use ngrok URL
	nbot.SetBaseURL(ngrokURL)

	// ngrok is ready, start serving (this will block)
	go func() {
		server := &http.Server{
			Handler:      handler,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
		}
		err := server.Serve(listener)
		if err != nil {
			slog.Error("ngrok HTTP server failed", "error", err)
		}
	}()

	return nil
}

// startRegularServer starts the HTTP server on localhost
func startRegularServer(port string, handler http.Handler) {
	slog.Info("HTTP server started", "port", port)
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}
	err := server.ListenAndServe()
	if err != nil {
		slog.Error("HTTP server failed", "error", err)
	}
}
