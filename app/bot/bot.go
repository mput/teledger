package bot

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"log/slog"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/mput/teledger/app/ledger"
	"github.com/mput/teledger/app/repo"
	"github.com/mput/teledger/app/teledger"
)

type Opts struct {
	Telegram struct {
		Token string `long:"token" env:"TOKEN" required:"true" description:"telegram bot token"`
	} `group:"telegram" namespace:"telegram" env-namespace:"TELEGRAM"`

	Github struct {
		URL   string `long:"url" env:"URL" required:"true" description:"github repo url"`
		Token string `long:"token" env:"TOKEN" required:"true" description:"fine-grained personal access tokens for repo with RW Contents scope"`
	} `group:"github" namespace:"github" env-namespace:"GITHUB"`

	OpenAI struct {
		Token string `long:"token" env:"TOKEN" required:"true" description:"openai api token"`
	} `group:"openai" namespace:"openai" env-namespace:"OPENAI"`

	// URL string `long:"url" env:"URL" required:"true" description:"bot url"`
	Version string
}

type Bot struct {
	opts     *Opts
	teledger *teledger.Teledger
	bot      *gotgbot.Bot
}

func NewBot(opts *Opts) (*Bot, error) {
	b, err := gotgbot.NewBot(opts.Telegram.Token, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create bot: %v", err)
	}

	rs := repo.NewInMemoryRepo(opts.Github.URL, opts.Github.Token)
	llmGenerator := ledger.NewOpenAITransactionGenerator(opts.OpenAI.Token)

	ldgr := ledger.NewLedger(rs, llmGenerator)
	tel := teledger.NewTeledger(ldgr)

	err = tel.Init()
	if err != nil {
		return nil, fmt.Errorf("unable to init teledger: %v", err)
	}

	return &Bot{
		opts:     opts,
		teledger: tel,
		bot:      b,
	}, nil

}

func (bot *Bot) Start() error {
	defaultCommands := []gotgbot.BotCommand{
		{Command: "reports", Description: "Show available reports"},
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

	dispatcher.AddHandler(handlers.NewCommand("reports", wrapUserResponse(bot.showAvailableReports, "reports")))
	dispatcher.AddHandler(handlers.NewCallback(isReportCallback, wrapUserResponse(bot.showReport, "show-report")))

	dispatcher.AddHandler(handlers.NewCommand("start", wrapUserResponse(start, "start")))
	dispatcher.AddHandler(handlers.NewCommand("version", wrapUserResponse(bot.vesrion, "version")))

	// these handlers should be at the end, as they are less specific
	dispatcher.AddHandler(handlers.NewCommand("/", wrapUserResponse(bot.comment, "comment")))
	dispatcher.AddHandler(handlers.NewMessage(nil, wrapUserResponse(bot.proposeTransaction, "propose-transaction")))
	dispatcher.AddHandler(handlers.NewCallback(isConfirmCallback, bot.confirmTransaction))

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
		return fmt.Errorf("failed to start polling: %v", err)
	}
	slog.Info("bot has been started", "bot-name", bot.bot.Username)
	updater.Idle()

	return nil
}

type response func(ctx *ext.Context) (msg string, opts *gotgbot.SendMessageOpts, err error)

func wrapUserResponse(next response, name string) handlers.Response {
	return func(b *gotgbot.Bot, ctx *ext.Context) error {
		start := time.Now()
		msg := ctx.EffectiveMessage
		resp, opts, err := next(ctx)
		if err != nil {
			slog.Error(
				"error in bot handler",
				"error", err,
				"duration", time.Since(start),
				"from", msg.From.Username,
				"handler", name,
			)
		} else {
			slog.Info(
				"handler success",
				"duration", time.Since(start),
				"from", msg.From.Username,
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
					"from", msg.From.Username,
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
	return fmt.Sprintf("teledger v: %s", bot.opts.Version),
		&gotgbot.SendMessageOpts{
			DisableNotification: true,
		},
		nil
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

	return fmt.Sprintf("```\n%s\n```", comment), &gotgbot.SendMessageOpts{
		ParseMode: "MarkdownV2",
		DisableNotification: true,
	}, nil
}

//go:embed templates/propose_transaction.html
var proposeTemplateS string
var proposeTemplate = template.Must(template.New("letter").Parse(proposeTemplateS))

func (bot *Bot) proposeTransaction(ctx *ext.Context) (string, *gotgbot.SendMessageOpts, error) {
	msg := ctx.EffectiveMessage

	pendTr := bot.teledger.ProposeTransaction(msg.Text)

	var buf bytes.Buffer
	err := proposeTemplate.Execute(&buf, pendTr)
	if err != nil {
		return "", nil, fmt.Errorf("unable to execute template: %v", err)
	}

	inlineKeyboard := [][]gotgbot.InlineKeyboardButton{}

	if key := pendTr.PendingKey; key != "" {
		inlineKeyboard = append(inlineKeyboard, []gotgbot.InlineKeyboardButton{
			{
				Text:         "✅ Confirm",
				CallbackData: fmt.Sprintf("%s%s",confirmPrefix, key),
			},
		})
	}

	return buf.String(), &gotgbot.SendMessageOpts{
		ParseMode: "HTML",
		// ReplyParameters: &gotgbot.ReplyParameters{MessageId: msg.MessageId},
		DisableNotification: true,
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{
			InlineKeyboard: inlineKeyboard,
		},

	}, nil
}

func (bot *Bot) showAvailableReports(_ *ext.Context) (string, *gotgbot.SendMessageOpts, error) {
	reports := bot.teledger.Ledger.Config.Reports

	inlineKeyboard := [][]gotgbot.InlineKeyboardButton{}

	for _, report := range reports {
		inlineKeyboard = append(inlineKeyboard, []gotgbot.InlineKeyboardButton{
			{
				Text:         report.Title,
				CallbackData: fmt.Sprintf("report:%s", report.Title),
			},
		})
	}

	opts := &gotgbot.SendMessageOpts{
		ParseMode: "MarkdownV2",
		DisableNotification: true,
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{
			InlineKeyboard: inlineKeyboard,
		},
	}

	return "Available reports:", opts, nil
}

func isReportCallback(cb *gotgbot.CallbackQuery) bool {
	return strings.HasPrefix(cb.Data, "report:")
}

const confirmPrefix = "cf:"
const deletePrefix =  "rm:"

func isConfirmCallback(cb *gotgbot.CallbackQuery) bool {
	return strings.HasPrefix(cb.Data, confirmPrefix)
}

func (bot *Bot) confirmTransaction(_ *gotgbot.Bot, ctx *ext.Context) error {
	cq := ctx.CallbackQuery

	_, _, err := bot.bot.EditMessageReplyMarkup(
		&gotgbot.EditMessageReplyMarkupOpts{
			MessageId: cq.Message.GetMessageId(),
			ChatId: cq.Message.GetChat().Id,
			InlineMessageId: cq.InlineMessageId,
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{},
		},
	)

	if err != nil {
		slog.Error("unable to edit inline keyboard", "error", err)
	}


	key := strings.TrimPrefix(cq.Data, confirmPrefix)
	pendTr, err := bot.teledger.ConfirmTransaction(key)

	var newMessageContent bytes.Buffer
	if err == nil {
		err2 := proposeTemplate.Execute(&newMessageContent, pendTr)
		if err2 != nil {
			err = err2
		}

	}

	if err != nil {
		_, _ = bot.bot.AnswerCallbackQuery(cq.Id, &gotgbot.AnswerCallbackQueryOpts{
			ShowAlert: true,
			Text: fmt.Sprintf("🛑️ Error!\n%s", err)  ,
		})

		_, _, _ = bot.bot.EditMessageReplyMarkup(
			&gotgbot.EditMessageReplyMarkupOpts{
				MessageId: cq.Message.GetMessageId(),
				ChatId: cq.Message.GetChat().Id,
				InlineMessageId: cq.InlineMessageId,
				ReplyMarkup: gotgbot.InlineKeyboardMarkup{},
			},
		)

		return nil
	}


	_, _, err = bot.bot.EditMessageText(
		newMessageContent.String(),
		&gotgbot.EditMessageTextOpts{
			MessageId: cq.Message.GetMessageId(),
			ChatId: cq.Message.GetChat().Id,
			InlineMessageId: cq.InlineMessageId,
			ParseMode: "HTML",
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					[]gotgbot.InlineKeyboardButton{
						{
							Text:         "🛑 Delete",
							CallbackData: fmt.Sprint(deletePrefix, key),
						},
					},
				},
			},
		},
	)

	if err != nil {
		slog.Error("unable to edit message", "error", err)
	}

	_, err = bot.bot.AnswerCallbackQuery(cq.Id, &gotgbot.AnswerCallbackQueryOpts{
		Text: "✔️ confirmed",
	})

	if err != nil {
		slog.Error("unable to answer callback query", "error", err)
	}
	return nil
}

func (bot *Bot) showReport(ctx *ext.Context) (string, *gotgbot.SendMessageOpts, error) {
	cq := ctx.CallbackQuery
	_, err := bot.bot.AnswerCallbackQuery(cq.Id, &gotgbot.AnswerCallbackQueryOpts{
		Text: "✔️",
	})

	if err != nil {
		slog.Error("unable to answer callback query", "error", err)
	}

	reportTitle := strings.TrimPrefix(cq.Data, "report:")
	report, err := bot.teledger.Report(reportTitle)

	if err != nil {
		return fmt.Sprintf("Error: %v", err), nil, nil
	}

	return fmt.Sprintf("```\n%s\n```", report), &gotgbot.SendMessageOpts{
		ParseMode: "MarkdownV2",
		DisableNotification: true,
	}, nil
}
