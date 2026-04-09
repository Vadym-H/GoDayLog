package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	//init config
	cfg := config.MustLoad()

	//init logger
	log := setupLogger(cfg.Env)
	log.Info("Starting server...", slog.String("env", cfg.Env))
	log.Debug("Debug message are enabled")

	log.Info("DatabaseURL", slog.String("url", cfg.Database.DatabaseUrl))

	//init storage
	dbCtx, cancelDB := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelDB()

	db, err := storage.New(dbCtx, log, cfg.Database.DatabaseUrl)
	if err != nil {
		log.Error("failed to connect to database", slog.String("error", err.Error()))
		panic(err)
	}
	defer db.Close()

	//init bot
	tgBot, err := bot.New(cfg.Telegram.Token, log, db)
	if err != nil {
		log.Error("failed to initialize telegram bot", slog.String("error", err.Error()))
		panic(err)
	}

	appCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	//run server
	tgBot.Start(appCtx)
}

func setupLogger(env string) *slog.Logger {
	var log *slog.Logger
	switch env {
	case envLocal:
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		)
	case envDev:
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		)
	case envProd:
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
		)
	default: // Handles envProd and any unknown environment
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
		)
	}
	return log
}
