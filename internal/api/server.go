package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/Vadym-H/GoDayLog/internal/config"
)

type Server struct {
	http *http.Server
	log  *slog.Logger
}

func NewServer(cfg *config.Config, log *slog.Logger, h http.Handler) *Server {
	return &Server{
		http: &http.Server{
			Addr:         cfg.Server.Address,
			Handler:      h,
			ReadTimeout:  cfg.Server.Timeout,
			WriteTimeout: cfg.Server.Timeout,
			IdleTimeout:  cfg.Server.IdleTimeout,
		},
		log: log,
	}
}

func (s *Server) Run(ctx context.Context) {
	go func() {
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.log.Error("server listen error", slog.Any("error", err))
		}
	}()
	s.log.Info("api server started", slog.String("addr", s.http.Addr))

	<-ctx.Done()

	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.http.Shutdown(shutCtx); err != nil {
		s.log.Error("graceful shutdown failed", slog.Any("error", err))
	}
	s.log.Info("api server stopped")
}
