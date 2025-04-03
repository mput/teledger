package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/jessevdk/go-flags"
	"github.com/mput/teledger/app/bot"
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

	nbot, err := bot.NewBot(&opts)
	if err != nil {
		slog.Error("unable to create bot", "err", err)
	}

	// TODO: handle signals
	err = nbot.Start()
	if err != nil {
		slog.Error("unable to start bot", "err", err)
	}

}
