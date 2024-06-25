package bot

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/mput/teledger/app/teledger"
	"github.com/mput/teledger/app/ledger"
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

	OpenAI struct {
		Token string `long:"token" env:"TOKEN" required:"true" description:"openai api token"`
	} `group:"openai" namespace:"openai" env-namespace:"OPENAI"`

	URL string `long:"url" env:"URL" required:"true" description:"bot url"`
	Debug bool `long:"debug" env:"DEBUG" description:"debug mode"`
	Version string
}

type Bot struct {
	opts *Opts
	teledger *teledger.Teledger
	bot *gotgbot.Bot
}

func NewBot(opts *Opts) (*Bot, error) {
	b, err := gotgbot.NewBot(opts.Telegram.Token, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create bot: %v", err)
	}

	rs := repo.NewInMemoryRepo(opts.Github.URL, opts.Github.Token)
	llmGenerator := ledger.NewOpenAITransactionGenerator(opts.OpenAI.Token)

	ldgr := ledger.NewLedger(rs, llmGenerator, opts.Github.MainLedgerFile, true)
	tel := teledger.NewTeledger(ldgr)

	return &Bot{
		opts: opts,
		teledger: tel,
		bot: b,
	}, nil

}

func (bot *Bot) Start() error {
	defaultCommands := []gotgbot.BotCommand{
		{Command: "balance", Description: "Show balance"},
		{Command: "version", Description: "Show version"},
	}
	smcRes, err := bot.bot.SetMyCommands(defaultCommands, nil)
	if err != nil {
		return fmt.Errorf("unable to set commands: %v", err)
	}
	slog.Info("commands has been set", "result", smcRes)

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
		// If an error is returned by a handler, log it and continue going.
		Error: func(_ *gotgbot.Bot, _ *ext.Context, err error) ext.DispatcherAction {
			slog.Error("unhandled error", "error", err.Error())
			return ext.DispatcherActionNoop
		},
		MaxRoutines: ext.DefaultMaxRoutines,
	})

	updater := ext.NewUpdater(dispatcher, nil)


	dispatcher.AddHandler(handlers.NewCommand("start", wrapUserResponse(start, "start")))
	dispatcher.AddHandler(handlers.NewCommand("version", wrapUserResponse(bot.vesrion, "version")))
	dispatcher.AddHandler(handlers.NewCommand("/", wrapUserResponse(bot.comment, "comment")))
	dispatcher.AddHandler(handlers.NewCommand("balance", wrapUserResponse(bot.bal, "balance")))
	dispatcher.AddHandler(handlers.NewMessage(nil, wrapUserResponse(bot.proposeTransaction, "propose-transaction")))



	// Start receiving updates.
	err = updater.StartPolling(bot.bot, &ext.PollingOpts{
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
	slog.Info("bot has been started", "bot-name", bot.bot.Username)
	updater.Idle()

	return nil
}


type response func(ctx *ext.Context) (msg string, opts *gotgbot.SendMessageOpts, err error)
func wrapUserResponse (next response, name string) handlers.Response {
	return func(b *gotgbot.Bot, ctx *ext.Context) error {
		start := time.Now()
		msg := ctx.EffectiveMessage
		resp, opts, err := next(ctx)
		if err != nil {
			slog.Error(
				"error in bot handler",
				"error", err,
				"duration", time.Since(start),
				"from",  msg.From.Username,
				"handler", name,
			)
		} else {
			slog.Info(
				"handler success",
				"duration", time.Since(start),
				"from",  msg.From.Username,
				"handler", name,
			)

		}
		if resp != "" {
			_, ierr := b.SendMessage(msg.Chat.Id, resp, opts)
			if ierr != nil {
				slog.Error(
					"unable to send response",
					"error", ierr,
					"duration", time.Since(start),
					"from",  msg.From.Username,
					"handler", name,
				)
			}
		}
		return err
	}
}

func start(_ *ext.Context) (string, *gotgbot.SendMessageOpts, error) {
	return "Welcome to teledger bot!", nil, nil
}

func (bot *Bot) vesrion(_ *ext.Context) (string, *gotgbot.SendMessageOpts, error) {
	return fmt.Sprintf("teledger v: %s", bot.opts.Version), nil , nil
}

func (bot *Bot) bal(_ *ext.Context) (string, *gotgbot.SendMessageOpts, error) {
	balance, err := bot.teledger.Balance()
	if err != nil {
		werr := fmt.Errorf("unable to get balance: %v", err)
		return werr.Error(), nil, werr
	}

	return fmt.Sprintf("```%s```", balance), &gotgbot.SendMessageOpts{ParseMode: "MarkdownV2"}, nil
}

func (bot *Bot) comment(ctx *ext.Context) (string, *gotgbot.SendMessageOpts, error) {
	msg := ctx.EffectiveMessage
	text := strings.TrimPrefix(msg.Text, "//")
	text = strings.TrimPrefix(text, "/comment")
	text = strings.TrimSpace(text)

	if text == "" {
		return "Empty comment!", nil, nil
	}

	comment, err := bot.teledger.AddComment(text)

	if err != nil {
		return fmt.Sprintf("Error: %v", err), nil, nil
	}

	return fmt.Sprintf("```\n%s\n```", comment), &gotgbot.SendMessageOpts{ParseMode: "MarkdownV2"}, nil
}

func (bot *Bot) proposeTransaction(ctx *ext.Context) (string, *gotgbot.SendMessageOpts, error) {
	msg := ctx.EffectiveMessage

	transaction, err := bot.teledger.ProposeTransaction(msg.Text)

	if err != nil {
		return fmt.Sprintf("Error: %v", err), nil, nil
	}

	return transaction, &gotgbot.SendMessageOpts{ParseMode: "MarkdownV2"}, nil
}
