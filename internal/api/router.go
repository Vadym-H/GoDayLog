package api

import (
	"net/http"

	"github.com/Vadym-H/GoDayLog/internal/api/handlers"
	"github.com/Vadym-H/GoDayLog/internal/api/middleware"
	"github.com/Vadym-H/GoDayLog/internal/config"
	"github.com/go-chi/chi/v5"
)

func NewRouter(cfg *config.Config, h *handlers.Handlers) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/telegram/miniapp", h.MiniApp)
		r.Post("/auth/telegram/widget", h.Widget)

		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(cfg.JWT))
			r.Get("/users/me", h.GetMe)
			r.Patch("/users/me", h.UpdateMe)
			r.Post("/log", h.SubmitLog)
			r.Get("/log/{id}", h.GetLogWithActivities)
			r.Post("/log/{id}/confirm", h.ConfirmLog)
			r.Post("/log/{id}/cancel", h.CancelLog)
			r.Get("/activity-logs", h.GetHistory)
			r.Delete("/activity-logs/{id}", h.DeleteActivity)
			r.Patch("/activity-logs/{id}", h.UpdateActivity)
			r.Get("/stats", h.GetStats)
			r.Post("/stats/analyse", h.AnalyseStats)
		})
	})

	return r
}
