package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Vadym-H/GoDayLog/internal/ai"
	"github.com/Vadym-H/GoDayLog/internal/config"
	storage "github.com/Vadym-H/GoDayLog/internal/storage/postgres"
	"github.com/Vadym-H/GoDayLog/internal/telegram/bot"
)

const (
	envLocal = "local"
	envDev   = "dev"
	envProd  = "prod"
)

func main() {
	// Bootstrap a debug logger so config errors land in the same JSON stream.
	bootstrap := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
	}))

	cfg := config.MustLoad(bootstrap)

	log := setupLogger(cfg.Env)
	log.Info("starting server", slog.String("env", cfg.Env))
	log.Debug("debug messages are enabled")

	dbCtx, cancelDB := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelDB()

	db, err := storage.New(dbCtx, log, cfg.Database.DatabaseUrl)
	if err != nil {
		log.Error("failed to connect to database", slog.Any("error", err))
		panic(err)
	}
	defer db.Close()

	aiClient := ai.New(cfg.LLM)

	tgBot, err := bot.New(cfg.Telegram.Token, log, db, aiClient)
	if err != nil {
		log.Error("failed to initialize telegram bot", slog.Any("error", err))
		panic(err)
	}

	appCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	tgBot.Start(appCtx)
}

func setupLogger(env string) *slog.Logger {
	opts := &slog.HandlerOptions{AddSource: true}

	switch env {
	case envLocal, envDev:
		opts.Level = slog.LevelDebug
	default:
		opts.Level = slog.LevelInfo
	}

	return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}
