package handlers

import (
	"errors"
	"net/http"

	"github.com/Vadym-H/GoDayLog/internal/ai"
	"github.com/Vadym-H/GoDayLog/internal/api/apicontext"
	"github.com/Vadym-H/GoDayLog/internal/logger"
)

func (h *Handlers) SubmitVoiceLog(w http.ResponseWriter, r *http.Request) {
	log := logger.From(r.Context(), h.log)

	id, ok := apicontext.IdentityFromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	maxBytes := int64(h.cfg.LLM.LLMLimits.MaxAudioBytes)
	if maxBytes > 0 {
		r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	file, header, err := r.FormFile("audio")
	if err != nil {
		writeError(w, http.StatusBadRequest, "audio field is required")
		return
	}
	defer file.Close()

	text, err := h.transcribe.Transcribe(r.Context(), id, file, header.Size, header.Header.Get("Content-Type"))
	if err != nil {
		switch {
		case errors.Is(err, ai.ErrAudioTooLarge):
			writeError(w, http.StatusRequestEntityTooLarge, "audio file too large")
		case errors.Is(err, ai.ErrAudioBudgetExceeded):
			writeError(w, http.StatusTooManyRequests, "daily audio budget exceeded")
		case errors.Is(err, ai.ErrUnsupportedAudioType):
			writeError(w, http.StatusUnsupportedMediaType, "unsupported audio type")
		default:
			log.Error("failed to transcribe audio", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	messageID, err := h.messageSvc.SaveMessage(r.Context(), id, "", text)
	if err != nil {
		log.Error("failed to save message", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	activities, err := h.aiProc.ExtractActivities(r.Context(), id, messageID, text)
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
