package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Vadym-H/GoDayLog/internal/ai"
	"github.com/Vadym-H/GoDayLog/internal/api"
	"github.com/Vadym-H/GoDayLog/internal/api/handlers"
	"github.com/Vadym-H/GoDayLog/internal/config"
	"github.com/Vadym-H/GoDayLog/internal/services"
	storage "github.com/Vadym-H/GoDayLog/internal/storage/postgres"
)

const (
	envLocal = "local"
	envDev   = "dev"
	envProd  = "prod"
)

func main() {
	bootstrap := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
	}))

	cfg := config.MustLoad(bootstrap)

	if cfg.JWT.Secret == "" {
		bootstrap.Error("JWT_SECRET is required for the API server")
		os.Exit(1)
	}

	log := setupLogger(cfg.Env)
	log.Info("starting api server", slog.String("env", cfg.Env))

	dbCtx, cancelDB := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelDB()

	db, err := storage.New(dbCtx, log, cfg.Database.DatabaseUrl)
	if err != nil {
		log.Error("failed to connect to database", slog.Any("error", err))
		panic(err)
	}
	defer db.Close()

	aiClient := ai.New(log, cfg.LLM)
	limits := cfg.LLM.LLMLimits
	proLimits := cfg.LLM.ProLimits

	userRepo := storage.NewUserRepo(db)
	messageRepo := storage.NewMessageRepo(db)
	activityLogRepo := storage.NewActivityLogRepo(db)
	aiRequestLogRepo := storage.NewAIRequestLogRepo(db)
	statsRepo := storage.NewStatsRepo(db)
	statsAnalysisRepo := storage.NewStatsAnalysisRepo(db)

	userService := services.NewUserService(log, userRepo, userRepo, limits, proLimits)
	messageService := services.NewMessageService(log, messageRepo)
	limiter := ai.NewLimiterMiddleware(aiClient, log, limits)
	aiProcessor := services.NewMessageAIProcessor(log, limiter, messageRepo, userRepo, activityLogRepo, aiRequestLogRepo, limits, proLimits)
	statsService := services.NewStatsService(log, statsRepo)
	statsAnalyser := services.NewStatsAnalyser(log, statsAnalysisRepo, aiClient, aiRequestLogRepo, userRepo, limits, proLimits)
	activityLogService := services.NewActivityLogService(log, activityLogRepo, activityLogRepo, messageRepo, userRepo)

	h := handlers.New(log, cfg, userService, messageService, aiProcessor, statsService, statsAnalyser, activityLogService)
	router := api.NewRouter(cfg, h)
	server := api.NewServer(cfg, log, router)

	appCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server.Run(appCtx)
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
