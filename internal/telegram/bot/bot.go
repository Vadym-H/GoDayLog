package bot

import (
	"context"
	"log/slog"

	"github.com/Vadym-H/GoDayLog/internal/ai"
	"github.com/Vadym-H/GoDayLog/internal/logger"
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

func New(token string, log *slog.Logger, db *storage.Storage, aiClient *ai.Client) (*Bot, error) {
	userRepo := storage.NewUserRepo(db)
	messageRepo := storage.NewMessageRepo(db)
	activityLogRepo := storage.NewActivityLogRepo(db)

	userService := services.NewUserService(log, userRepo, userRepo)
	messageService := services.NewMessageService(log, messageRepo)
	aiProcessor := services.NewMessageAIProcessor(log, aiClient, messageRepo, userRepo, activityLogRepo)

	b := &Bot{
		log:        log,
		tgHandlers: tghandlers.New(log, userService, messageService, aiProcessor),
	}

	tg, err := tgbot.New(token, tgbot.WithDefaultHandler(b.withRequestID(b.handleMessage)))
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
		log.Warn("failed to set bot commands", slog.Any("error", err))
	}

	b.tg.RegisterHandler(tgbot.HandlerTypeMessageText, "/start", tgbot.MatchTypeExact, b.withRequestID(b.tgHandlers.HandleStart))
	b.tg.RegisterHandler(tgbot.HandlerTypeMessageText, "/log", tgbot.MatchTypeExact, b.withRequestID(b.tgHandlers.HandleLog))
	b.tg.RegisterHandler(tgbot.HandlerTypeMessageText, "/stats", tgbot.MatchTypeExact, b.withRequestID(b.tgHandlers.HandleStats))
	b.tg.RegisterHandler(tgbot.HandlerTypeMessageText, "/help", tgbot.MatchTypeExact, b.withRequestID(b.tgHandlers.HandleHelp))
	b.tg.RegisterHandler(tgbot.HandlerTypeCallbackQueryData, "action:", tgbot.MatchTypePrefix, b.withRequestID(b.tgHandlers.HandleMenuAction))

	return b, nil
}

func (b *Bot) Start(ctx context.Context) {
	b.log.Info("telegram bot started")
	b.tg.Start(ctx)
	b.log.Info("telegram bot stopped")
}

// withRequestID stamps a fresh request ID onto ctx before dispatching to any handler.
// Every entry point (commands + default) goes through this so all log lines in a
// request share the same request_id.
func (b *Bot) withRequestID(h func(context.Context, *tgbot.Bot, *models.Update)) func(context.Context, *tgbot.Bot, *models.Update) {
	return func(ctx context.Context, bot *tgbot.Bot, update *models.Update) {
		ctx = logger.WithRequestID(ctx, logger.NewRequestID())
		h(ctx, bot, update)
	}
}

func (b *Bot) handleMessage(ctx context.Context, bot *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	if b.tgHandlers.HandlePendingContextInput(ctx, bot, update) {
		return
	}

	if b.tgHandlers.HandlePendingTagInput(ctx, bot, update) {
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
		logger.From(ctx, b.log).Error("failed to send default response", slog.Any("error", err))
	}
}
