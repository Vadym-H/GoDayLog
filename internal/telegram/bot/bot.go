package bot

import (
	"context"
	"log/slog"

	"github.com/Vadym-H/GoDayLog/internal/services"
	storage "github.com/Vadym-H/GoDayLog/internal/storage/postgres"
	"github.com/Vadym-H/GoDayLog/internal/telegram/tghandlers"
	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Bot struct {
	tg         *tgbot.Bot
	log        *slog.Logger
	tgHandlers *tghandlers.TgHandlers
}

func New(token string, log *slog.Logger, storage *storage.Storage) (*Bot, error) {
	const op = "telegram.bot.New"

	userService := services.NewUserService(log, storage, storage)
	messageService := services.NewMessageService(log, storage)

	b := &Bot{
		log:        log,
		tgHandlers: tghandlers.New(log, userService, messageService),
	}

	tg, err := tgbot.New(token, tgbot.WithDefaultHandler(b.handleMessage))
	if err != nil {
		return nil, err
	}

	b.tg = tg

	_, err = b.tg.SetMyCommands(context.Background(), &tgbot.SetMyCommandsParams{
		Commands: []models.BotCommand{
			{Command: "start", Description: "Open home"},
			{Command: "log", Description: "Log an activity"},
			{Command: "stats", Description: "Today stats"},
			{Command: "help", Description: "How to use the bot"},
		},
	})
	if err != nil {
		log.Error("failed to set bot commands",
			slog.String("op", op),
			slog.String("error", err.Error()),
		)
	}

	b.tg.RegisterHandler(tgbot.HandlerTypeMessageText, "/start", tgbot.MatchTypeExact, b.tgHandlers.HandleStart)
	b.tg.RegisterHandler(tgbot.HandlerTypeMessageText, "/log", tgbot.MatchTypeExact, b.tgHandlers.HandleLog)
	b.tg.RegisterHandler(tgbot.HandlerTypeMessageText, "/stats", tgbot.MatchTypeExact, b.tgHandlers.HandleStats)
	b.tg.RegisterHandler(tgbot.HandlerTypeMessageText, "/help", tgbot.MatchTypeExact, b.tgHandlers.HandleHelp)
	b.tg.RegisterHandler(tgbot.HandlerTypeCallbackQueryData, "action:", tgbot.MatchTypePrefix, b.tgHandlers.HandleMenuAction)

	return b, nil
}

func (b *Bot) Start(ctx context.Context) {
	const op = "telegram.bot.Start"

	b.log.Info("telegram bot started", slog.String("op", op))
	b.tg.Start(ctx)
	b.log.Info("telegram bot stopped", slog.String("op", op))
}

func (b *Bot) handleMessage(ctx context.Context, bot *tgbot.Bot, update *models.Update) {
	const op = "telegram.bot.handleMessage"

	if update.Message == nil {
		return
	}

	if b.tgHandlers.HandlePendingContextInput(ctx, bot, update) {
		return
	}

	if b.tgHandlers.HandlePendingLogInput(ctx, bot, update) {
		return
	}

	_, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Use /start",
	})
	if err != nil {
		b.log.Error("failed to send default response",
			slog.String("op", op),
			slog.String("error", err.Error()),
		)
	}
}
