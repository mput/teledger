package main

import (
	"fmt"
	"log"
	"os"

	"github.com/jessevdk/go-flags"
	"github.com/mput/teledger/app/bot"
)


var version = "dev"

func main() {
	fmt.Printf("teledger v:%s\n", version)
	opts := bot.Opts{}
	_, err := flags.Parse(&opts)
	if err != nil {
		// if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
		// 	log.Printf("[ERROR] %v", err)
		// }
		log.Printf("[ERROR] %v", err)
		os.Exit(1)
	}
	// log.Printf("[DEBUG] opts: %+v", opts)

	err = opts.Execute()
	if err != nil {
		log.Fatalf("[ERROR] unable to start bot: %v", err)
	}
}
