package bot

import (
	"fmt"
	"log"
	"log/slog"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/mput/teledger/app/repo"
)


type Opts struct {
	Telegram struct {
		Token string `long:"token" env:"TOKEN" required:"true" description:"telegram bot token"`
	} `group:"telegram" namespace:"telegram" env-namespace:"TELEGRAM"`

	Github struct {
		URL string `long:"url" env:"URL" required:"true" description:"github repo url"`
		Token string `long:"token" env:"TOKEN" required:"true" description:"fine-grained personal access tokens for repo with RW Contents scope"`
		MainLedgerFile string `long:"main-ledger-file" env:"MAIN_LEDGER_FILE" required:"true" description:"main ledger file path from the repo root"`
	} `group:"github" namespace:"github" env-namespace:"GITHUB"`

	URL string `long:"url" env:"URL" required:"true" description:"bot url"`
	Debug bool `long:"debug" env:"DEBUG" description:"debug mode"`
	Version string
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
	dispatcher.AddHandler(handlers.NewCommand("bal", opts.bal))
	dispatcher.AddHandler(handlers.NewCommand("version", opts.vesrion))
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
	log.Printf("[INFO] %s has been started...\n", b.Username)
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

func (opts *Opts) vesrion(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	log.Printf("[INFO] version request. user=%s\n", msg.From.Username)
	if _, err := b.SendMessage(msg.Chat.Id, fmt.Sprintf("teledger v: %s", opts.Version), nil); err != nil {
		return fmt.Errorf("unable to send message: %w", err)
	}
	return nil
}

func (opts *Opts) bal(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	log.Printf("[INFO] balance request. user=%s\n", msg.From.Username)

	repo, err := repo.NewInMemoryRepo(opts.Github.URL, opts.Github.Token)

	if err != nil {
		slog.Error("unable to init repo", "error", err)
		b.SendMessage(msg.Chat.Id, fmt.Sprintf("unable to init repo: %v", err), nil)
		return fmt.Errorf("unable to init repo: %w", err)
	}


	ledger := NewLedger(repo, opts.Github.MainLedgerFile, true)

	balance, err := ledger.Execute("bal")

	if err != nil {
		slog.Error("unable to get balance", "error", err)
		b.SendMessage(msg.Chat.Id, fmt.Sprintf("unable to get balance: %v", err), nil)
		return fmt.Errorf("unable to get balance: %w", err)
	}

	if _, err := b.SendMessage(msg.Chat.Id, fmt.Sprintf("```%s```", balance), &gotgbot.SendMessageOpts{ParseMode: "MarkdownV2"}); err != nil {
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
