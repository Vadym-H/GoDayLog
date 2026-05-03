package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Vadym-H/GoDayLog/internal/ai"
	"github.com/Vadym-H/GoDayLog/internal/api/apicontext"
	"github.com/Vadym-H/GoDayLog/internal/domain"
	"github.com/Vadym-H/GoDayLog/internal/logger"
	"github.com/Vadym-H/GoDayLog/internal/storage"
	"github.com/go-chi/chi/v5"
)

type activityJSON struct {
	Description     string     `json:"description"`
	Tag             string     `json:"tag"`
	ActivityType    string     `json:"activity_type"`
	DurationMinutes *int       `json:"duration_minutes,omitempty"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
}

func (h *Handlers) SubmitLog(w http.ResponseWriter, r *http.Request) {
	log := logger.From(r.Context(), h.log)

	id, ok := apicontext.IdentityFromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Text == "" {
		writeError(w, http.StatusBadRequest, "text is required")
		return
	}

	messageID, err := h.messageSvc.SaveMessage(r.Context(), id, "", body.Text)
	if err != nil {
		log.Error("failed to save message", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	activities, err := h.aiProc.ExtractActivities(r.Context(), id, messageID, body.Text)
	if err != nil {
		if errors.Is(err, ai.ErrDailyBudgetExceeded) {
			writeError(w, http.StatusTooManyRequests, "daily budget exceeded")
			return
		}
		if errors.Is(err, ai.ErrInputTooLong) {
			writeError(w, http.StatusBadRequest, "input too long")
			return
		}
		log.Error("failed to extract activities", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if len(activities) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "could not extract activities from the provided text")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message_id": messageID,
		"activities": domainToJSON(activities),
	})
}

func (h *Handlers) ConfirmLog(w http.ResponseWriter, r *http.Request) {
	log := logger.From(r.Context(), h.log)

	messageID := chi.URLParam(r, "id")
	if messageID == "" {
		writeError(w, http.StatusBadRequest, "missing message id")
		return
	}

	var body struct {
		Activities []activityJSON `json:"activities"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Activities) == 0 {
		writeError(w, http.StatusBadRequest, "activities are required")
		return
	}

	activities := jsonToDomain(body.Activities)

	if err := h.aiProc.SaveActivities(r.Context(), messageID, activities); err != nil {
		if errors.Is(err, storage.ErrMessageFailed) {
			writeError(w, http.StatusConflict, "session expired, please submit your log again")
			return
		}
		if errors.Is(err, storage.ErrMessageNotFound) {
			writeError(w, http.StatusNotFound, "message not found")
			return
		}
		log.Error("failed to save activities", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeOK(w)
}

func (h *Handlers) CancelLog(w http.ResponseWriter, r *http.Request) {
	log := logger.From(r.Context(), h.log)

	messageID := chi.URLParam(r, "id")
	if messageID == "" {
		writeError(w, http.StatusBadRequest, "missing message id")
		return
	}

	if err := h.aiProc.CancelReview(r.Context(), messageID); err != nil {
		log.Error("failed to cancel review", "error", err)
	}

	writeOK(w)
}

func domainToJSON(activities []domain.Activity) []activityJSON {
	out := make([]activityJSON, len(activities))
	for i, a := range activities {
		out[i] = activityJSON{
			Description:     a.Description,
			Tag:             a.Tag,
			ActivityType:    a.ActivityType,
			DurationMinutes: a.DurationMinutes,
			StartedAt:       a.StartedAt,
			CompletedAt:     a.CompletedAt,
		}
	}
	return out
}

func jsonToDomain(items []activityJSON) []domain.Activity {
	out := make([]domain.Activity, len(items))
	for i, a := range items {
		out[i] = domain.Activity{
			Description:     a.Description,
			Tag:             a.Tag,
			ActivityType:    a.ActivityType,
			DurationMinutes: a.DurationMinutes,
			StartedAt:       a.StartedAt,
			CompletedAt:     a.CompletedAt,
		}
	}
	return out
}
