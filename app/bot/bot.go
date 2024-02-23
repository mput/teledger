package bot

import (
	"fmt"
	"log/slog"
	"strings"
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


type Response func(b *gotgbot.Bot, ctx *ext.Context) (msg string, opts *gotgbot.SendMessageOpts, err error)

func WrapUserResponse (next Response, name string) handlers.Response {
	return func(b *gotgbot.Bot, ctx *ext.Context) error {
		start := time.Now()
		msg := ctx.EffectiveMessage
		resp, opts, err := next(b, ctx)
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


func (opts *Opts) Execute() error {

	b, err := gotgbot.NewBot(opts.Telegram.Token, nil)
	if err != nil {
		return fmt.Errorf("unable to create bot: %v", err)
	}

	defaultCommands := []gotgbot.BotCommand{
		{Command: "start", Description: "Start the bot"},
		{Command: "balance", Description: "Show balance"},
		{Command: "comment", Description: "Comment on the ledger"},
		{Command: "version", Description: "Show version"},
	}

	smcRes, err := b.SetMyCommands(defaultCommands, nil)

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


	dispatcher.AddHandler(handlers.NewCommand("start", WrapUserResponse(start, "start")))
	dispatcher.AddHandler(handlers.NewCommand("balance", WrapUserResponse(opts.bal, "balance")))
	dispatcher.AddHandler(handlers.NewCommand("version", WrapUserResponse(opts.vesrion, "version")))
	dispatcher.AddHandler(handlers.NewCommand("/", WrapUserResponse(opts.comment, "comment")))
	dispatcher.AddHandler(handlers.NewCommand("comment", WrapUserResponse(opts.comment, "comment")))

	dispatcher.AddHandler(handlers.NewMessage(nil, WrapUserResponse(echo, "echo")))


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

func start(b *gotgbot.Bot, ctx *ext.Context) (string, *gotgbot.SendMessageOpts, error) {
	return "Welcome to teledger bot!", nil, nil
}

func (opts *Opts) vesrion(_ *gotgbot.Bot, _ *ext.Context) (string, *gotgbot.SendMessageOpts, error) {
	return fmt.Sprintf("teledger v: %s", opts.Version), nil , nil
}

func (opts *Opts) bal(_ *gotgbot.Bot, _ *ext.Context) (string, *gotgbot.SendMessageOpts, error) {
	rs, err := repo.NewInMemoryRepo(opts.Github.URL, opts.Github.Token)

	if err != nil {
		werr := fmt.Errorf("unable to init repo: %w", err)
		return werr.Error(), nil, werr
	}

	ledger := NewLedger(rs, opts.Github.MainLedgerFile, true)

	balance, err := ledger.Execute("bal")

	if err != nil {
		werr := fmt.Errorf("unable to get balance: %v", err)
		return werr.Error(), nil, werr
	}

	return fmt.Sprintf("```%s```", balance), &gotgbot.SendMessageOpts{ParseMode: "MarkdownV2"}, nil
}

func (opts *Opts) comment(b *gotgbot.Bot, ctx *ext.Context) (string, *gotgbot.SendMessageOpts, error) {
	msg := ctx.EffectiveMessage

	// TODO: add comment prefix to every line
	timezoneName := "CET"
	loc, err := time.LoadLocation(timezoneName)
	if err != nil {
		return err.Error(), nil, err
	}

	lines := []string{fmt.Sprintf(";; %s", time.Now().In(loc).Format("2006-01-02 15:04:05 Monday"))}
	commitLine := ""

	for i, l := range strings.Split(msg.Text, "\n") {
		if i == 0 {
			l = strings.TrimSpace(l)
			sl := strings.SplitN(l, " ", 2)
			fmt.Println(sl)
			if len(sl) > 2 {
				panic(fmt.Errorf("unexpected strings.SplitN(l, \" \", 2) result: %v", sl))
			}
			if len(sl) == 2 {
				l = sl[1]
				commitLine = l
			} else {
				l = ""
			}
		}
		if l != "" {
			lines = append(lines, fmt.Sprintf(";; %s", l))
		}
	}

	if len(lines) == 1 {
		return "No comment provided", nil, nil
	}

	rs, err := repo.NewInMemoryRepo(opts.Github.URL, opts.Github.Token)

	if err != nil {
		werr := fmt.Errorf("unable to init repo: %w", err)
		return werr.Error(), nil, werr
	}

	f, err := rs.OpenForAppend(opts.Github.MainLedgerFile)

	if err != nil {
		werr := fmt.Errorf("unable to open main ledger file: %v", err)
		return werr.Error(), nil, werr
	}

	comment := strings.Join(lines, "\n")

	_, err = fmt.Fprintf(f, "\n%s\n", comment)

	if err != nil {
		werr := fmt.Errorf("unable to write main ledger file: %v", err)
		return werr.Error(), nil, werr
	}

	err = f.Close()

	if err != nil {
		werr := fmt.Errorf("unable to close main ledger file: %v", err)
		return werr.Error(), nil, werr
	}

	err = rs.CommitPush(fmt.Sprintf("comment: %s", commitLine), msg.From.Username, "teledger@example.com")

	if err != nil {
		werr := fmt.Errorf("unable to commit: %v", err)
		return werr.Error(), nil, werr
	}

	return fmt.Sprintf("Commented:\n```\n%s\n```", comment), &gotgbot.SendMessageOpts{ParseMode: "MarkdownV2"}, nil
}


func echo(b *gotgbot.Bot, ctx *ext.Context) (string, *gotgbot.SendMessageOpts, error) {
	msg := ctx.EffectiveMessage
	return fmt.Sprintf("Message received! (%s)", msg.Text), nil, nil
}
