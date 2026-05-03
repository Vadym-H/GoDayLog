package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Vadym-H/GoDayLog/internal/ai"
	"github.com/Vadym-H/GoDayLog/internal/api/apicontext"
	"github.com/Vadym-H/GoDayLog/internal/domain"
	"github.com/Vadym-H/GoDayLog/internal/logger"
	"github.com/Vadym-H/GoDayLog/internal/services"
)

func (h *Handlers) GetStats(w http.ResponseWriter, r *http.Request) {
	log := logger.From(r.Context(), h.log)

	id, ok := apicontext.IdentityFromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	q, err := h.buildStatsQuery(r, id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	report, err := h.statsSvc.GetStats(r.Context(), q)
	if err != nil {
		log.Error("failed to get stats", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, report)
}

func (h *Handlers) AnalyseStats(w http.ResponseWriter, r *http.Request) {
	log := logger.From(r.Context(), h.log)

	id, ok := apicontext.IdentityFromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	q, err := h.buildStatsQuery(r, id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	report, err := h.statsSvc.GetStats(r.Context(), q)
	if err != nil {
		log.Error("failed to get stats for analysis", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	userContext, err := h.userSvc.GetUserContext(r.Context(), id)
	if err != nil {
		log.Warn("could not fetch user context for analysis, proceeding without it", "error", err)
		userContext = ""
	}

	analysis, err := h.statsAnalyser.Analyse(r.Context(), q, report, userContext)
	if err != nil {
		if err == ai.ErrDailyBudgetExceeded {
			writeError(w, http.StatusTooManyRequests, "daily budget exceeded")
			return
		}
		if err == ai.ErrInputTooLong {
			writeError(w, http.StatusBadRequest, "input too long")
			return
		}
		log.Error("stats analysis failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"analysis": analysis})
}

func (h *Handlers) buildStatsQuery(r *http.Request, id domain.Identity) (services.StatsQuery, error) {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	if fromStr == "" || toStr == "" {
		return services.StatsQuery{}, fmt.Errorf("from and to query params are required (format: 2006-01-02)")
	}

	tz, err := h.userSvc.GetUserTimezone(r.Context(), id)
	if err != nil {
		tz = "UTC"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}

	from, err := time.ParseInLocation("2006-01-02", fromStr, loc)
	if err != nil {
		return services.StatsQuery{}, fmt.Errorf("invalid from date, use format 2006-01-02")
	}
	to, err := time.ParseInLocation("2006-01-02", toStr, loc)
	if err != nil {
		return services.StatsQuery{}, fmt.Errorf("invalid to date, use format 2006-01-02")
	}
	to = to.Add(24 * time.Hour) // end of day, exclusive

	userID, err := h.userSvc.GetUserID(r.Context(), id)
	if err != nil {
		return services.StatsQuery{}, fmt.Errorf("user not found")
	}

	return services.StatsQuery{
		UserID:   userID,
		From:     from,
		To:       to,
		Timezone: tz,
	}, nil
}
