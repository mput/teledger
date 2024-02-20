package bot

import (
	"fmt"
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


type Response func(b *gotgbot.Bot, ctx *ext.Context) (string, error)

func WrapUserResponse (next Response) handlers.Response {
	return func(b *gotgbot.Bot, ctx *ext.Context) error {
		msg := ctx.EffectiveMessage
		resp, err := next(b, ctx)
		if err != nil {
			slog.Error("error in handler", "error", err)
		}
		if resp != "" {
			_, ierr := b.SendMessage(msg.Chat.Id, resp, &gotgbot.SendMessageOpts{ParseMode: "MarkdownV2"})
			if ierr != nil {
				slog.Error("unable to send response", "error", ierr)
			}
		}
		return err
	}
}


func (opts *Opts) Execute() error {

	b, err := gotgbot.NewBot(opts.Telegram.Token, nil)
	if err != nil {
		return fmt.Errorf("unable to create bot: %v", err)
	}

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
		// If an error is returned by a handler, log it and continue going.
		Error: func(_ *gotgbot.Bot, _ *ext.Context, err error) ext.DispatcherAction {
			slog.Error("unhandled error", "error", err.Error())
			return ext.DispatcherActionNoop
		},
		MaxRoutines: ext.DefaultMaxRoutines,
	})

	updater := ext.NewUpdater(dispatcher, nil)


	dispatcher.AddHandler(handlers.NewCommand("start", WrapUserResponse(start)))
	dispatcher.AddHandler(handlers.NewCommand("bal", WrapUserResponse(opts.bal)))
	dispatcher.AddHandler(handlers.NewCommand("version", WrapUserResponse(opts.vesrion)))
	dispatcher.AddHandler(handlers.NewCommand("/", WrapUserResponse(opts.comment)))

	dispatcher.AddHandler(handlers.NewMessage(nil, WrapUserResponse(echo)))

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
	slog.Info("bot has been started", "bot-name", b.Username)
	updater.Idle()

	return nil
}

func start(b *gotgbot.Bot, ctx *ext.Context) (string, error) {
	return "Welcome to teledger bot!", nil
}

func (opts *Opts) vesrion(b *gotgbot.Bot, ctx *ext.Context) (string, error) {
	return fmt.Sprintf("teledger v: %s", opts.Version), nil
}

func (opts *Opts) bal(b *gotgbot.Bot, ctx *ext.Context) (string, error) {
	rs, err := repo.NewInMemoryRepo(opts.Github.URL, opts.Github.Token)

	if err != nil {
		werr := fmt.Errorf("unable to init repo: %w", err)
		return werr.Error(), werr
	}

	ledger := NewLedger(rs, opts.Github.MainLedgerFile, true)

	balance, err := ledger.Execute("bal")

	if err != nil {
		werr := fmt.Errorf("unable to get balance: %v", err)
		return werr.Error(), werr
	}

	return fmt.Sprintf("```%s```", balance), nil
}

func (opts *Opts) comment(b *gotgbot.Bot, ctx *ext.Context) (string, error) {
	msg := ctx.EffectiveMessage

	rs, err := repo.NewInMemoryRepo(opts.Github.URL, opts.Github.Token)

	if err != nil {
		werr := fmt.Errorf("unable to init repo: %w", err)
		return werr.Error(), werr
	}

	f, err := rs.OpenForAppend(opts.Github.MainLedgerFile)

	if err != nil {
		werr := fmt.Errorf("unable to open main ledger file: %v", err)
		return werr.Error(), werr
	}

	// TODO: add comment prefix to every line
	comment := fmt.Sprintf(";; %s\n", msg.Text)
	_, err = fmt.Fprint(f, comment)

	if err != nil {
		werr := fmt.Errorf("unable to write main ledger file: %v", err)
		return werr.Error(), werr
	}

	err = f.Close()

	if err != nil {
		werr := fmt.Errorf("unable to close main ledger file: %v", err)
		return werr.Error(), werr
	}

	err = rs.CommitPush("new comment", msg.From.Username, "teledger@example.com")

	if err != nil {
		werr := fmt.Errorf("unable to commit: %v", err)
		return werr.Error(), werr
	}

	return fmt.Sprintf("Comment added:\n```%s```", comment), nil
}


func echo(b *gotgbot.Bot, ctx *ext.Context) (string, error) {
	msg := ctx.EffectiveMessage
	return fmt.Sprintf("Message received! (%s)", msg.Text), nil
}
