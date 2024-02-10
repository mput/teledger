package bot

import (
	"fmt"
	"log"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)


type Opts struct {
	Telegram struct {
		Token string `long:"token" env:"TOKEN" required:"true" description:"telegram bot token"`
	} `group:"telegram" namespace:"telegram" env-namespace:"TELEGRAM"`

	URL string `long:"url" env:"URL" required:"true" description:"bot url"`
	Debug bool `long:"debug" env:"DEBUG" description:"debug mode"`
}


func (opts *Opts) Execute() error {

	b, err := gotgbot.NewBot(opts.Telegram.Token, nil)
	if err != nil {
		return fmt.Errorf("unable to create bot: %v", err)
	}

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
		// If an error is returned by a handler, log it and continue going.
		Error: func(_ *gotgbot.Bot, _ *ext.Context, err error) ext.DispatcherAction {
			log.Println("an error occurred while handling update:", err.Error())
			return ext.DispatcherActionNoop
		},
		MaxRoutines: ext.DefaultMaxRoutines,
	})

	updater := ext.NewUpdater(dispatcher, nil)


	dispatcher.AddHandler(handlers.NewCommand("start", start))
	dispatcher.AddHandler(handlers.NewMessage(nil, message))

	// Start receiving updates.
	err = updater.StartPolling(b, &ext.PollingOpts{
		DropPendingUpdates: true,
		GetUpdatesOpts: &gotgbot.GetUpdatesOpts{
			Timeout: 9,
			RequestOpts: &gotgbot.RequestOpts{
				Timeout: time.Second * 10,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to start polling: %v",  err)
	}
	log.Printf("[INFO] %s has been started...\n", b.User.Username)
	// Idle, to keep updates coming in, and avoid bot stopping.
	updater.Idle()

	return nil
}

func start(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	log.Printf("[INFO] %s has started a chat with the bot\n", msg.From.Username)
	if _, err := b.SendMessage(msg.Chat.Id, "Welcome to teledger bot!", nil); err != nil {
		return fmt.Errorf("unable to send message: %w", err)
	}
	return nil
}

func message(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	log.Printf("[INFO] %s has sent a message to the bot\n", msg.From.Username)
	if _, err := b.SendMessage(msg.Chat.Id, fmt.Sprintf("Message received! (%s)", msg.Text), nil); err != nil {
		return fmt.Errorf("unable to send message: %w", err)
	}
	return nil
}
