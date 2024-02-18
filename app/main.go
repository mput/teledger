package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/jessevdk/go-flags"
	"github.com/mput/teledger/app/bot"
)


var version = "dev"

func setupLogger(debug bool) {
	opts := &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }

	if debug {
		opts.Level = slog.LevelDebug
		opts.AddSource = true
	}

	textHandler := slog.NewTextHandler(os.Stdout, opts)
	logger := slog.New(textHandler)

	slog.SetDefault(logger)
}

func main() {
	fmt.Printf("teledger v:%s\n", version)
	opts := bot.Opts{}
	opts.Version = version
	_, err := flags.Parse(&opts)
	if err != nil {
		fmt.Printf("[ERROR] %v", err)
		os.Exit(1)
	}
	setupLogger(opts.Debug)

	err = opts.Execute()
	if err != nil {
		slog.Error("unable to start bot: %v", err)
	}
}
