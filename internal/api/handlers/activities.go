package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/Vadym-H/GoDayLog/internal/api/apicontext"
	"github.com/Vadym-H/GoDayLog/internal/domain"
	"github.com/Vadym-H/GoDayLog/internal/logger"
	"github.com/Vadym-H/GoDayLog/internal/storage"
	"github.com/go-chi/chi/v5"
)

type activityLogJSON struct {
	ID              string     `json:"id"`
	MessageID       string     `json:"message_id"`
	Description     string     `json:"description"`
	Tag             string     `json:"tag"`
	ActivityType    string     `json:"activity_type"`
	DurationMinutes *int       `json:"duration_minutes,omitempty"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

func activityLogToJSON(a domain.ActivityLog) activityLogJSON {
	return activityLogJSON{
		ID:              a.ID,
		MessageID:       a.MessageID,
		Description:     a.Description,
		Tag:             a.Tag,
		ActivityType:    a.ActivityType,
		DurationMinutes: a.DurationMinutes,
		StartedAt:       a.StartedAt,
		CompletedAt:     a.CompletedAt,
		CreatedAt:       a.CreatedAt,
	}
}

func (h *Handlers) GetHistory(w http.ResponseWriter, r *http.Request) {
	log := logger.From(r.Context(), h.log)

	id, ok := apicontext.IdentityFromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	limit := 20
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	activities, err := h.activitySvc.GetHistory(r.Context(), id, limit, offset)
	if err != nil {
		log.Error("failed to get activity history", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	out := make([]activityLogJSON, len(activities))
	for i, a := range activities {
		out[i] = activityLogToJSON(a)
	}

	writeJSON(w, http.StatusOK, map[string]any{"activities": out})
}

func (h *Handlers) GetLogWithActivities(w http.ResponseWriter, r *http.Request) {
	log := logger.From(r.Context(), h.log)

	id, ok := apicontext.IdentityFromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	messageID := chi.URLParam(r, "id")
	if messageID == "" {
		writeError(w, http.StatusBadRequest, "missing message id")
		return
	}

	result, err := h.activitySvc.GetMessageWithActivities(r.Context(), id, messageID)
	if err != nil {
		if errors.Is(err, storage.ErrMessageNotFound) {
			writeError(w, http.StatusNotFound, "message not found")
			return
		}
		log.Error("failed to get message with activities", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	activities := make([]activityLogJSON, len(result.Activities))
	for i, a := range result.Activities {
		activities[i] = activityLogToJSON(a)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message": map[string]any{
			"id":         result.Message.ID,
			"text":       result.Message.Text,
			"status":     result.Message.Status,
			"created_at": result.Message.CreatedAt,
		},
		"activities": activities,
	})
}

func (h *Handlers) DeleteActivity(w http.ResponseWriter, r *http.Request) {
	log := logger.From(r.Context(), h.log)

	id, ok := apicontext.IdentityFromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	activityID := chi.URLParam(r, "id")
	if activityID == "" {
		writeError(w, http.StatusBadRequest, "missing activity id")
		return
	}

	if err := h.activitySvc.DeleteActivity(r.Context(), id, activityID); err != nil {
		if errors.Is(err, storage.ErrActivityNotFound) {
			writeError(w, http.StatusNotFound, "activity not found")
			return
		}
		log.Error("failed to delete activity", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeOK(w)
}

func (h *Handlers) UpdateActivity(w http.ResponseWriter, r *http.Request) {
	log := logger.From(r.Context(), h.log)

	id, ok := apicontext.IdentityFromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	activityID := chi.URLParam(r, "id")
	if activityID == "" {
		writeError(w, http.StatusBadRequest, "missing activity id")
		return
	}

	var body struct {
		Description     *string    `json:"description"`
		Tag             *string    `json:"tag"`
		ActivityType    *string    `json:"activity_type"`
		DurationMinutes *int       `json:"duration_minutes"`
		StartedAt       *time.Time `json:"started_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	fields := domain.UpdateActivityFields{
		Description:     body.Description,
		Tag:             body.Tag,
		ActivityType:    body.ActivityType,
		DurationMinutes: body.DurationMinutes,
		StartedAt:       body.StartedAt,
	}

	updated, err := h.activitySvc.UpdateActivity(r.Context(), id, activityID, fields)
	if err != nil {
		if errors.Is(err, storage.ErrActivityNotFound) {
			writeError(w, http.StatusNotFound, "activity not found")
			return
		}
		log.Error("failed to update activity", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, activityLogToJSON(updated))
}
