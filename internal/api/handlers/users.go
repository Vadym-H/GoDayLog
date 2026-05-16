package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Vadym-H/GoDayLog/internal/api/apicontext"
	"github.com/Vadym-H/GoDayLog/internal/logger"
	"github.com/Vadym-H/GoDayLog/internal/services"
)

func (h *Handlers) GetMe(w http.ResponseWriter, r *http.Request) {
	log := logger.From(r.Context(), h.log)

	id, ok := apicontext.IdentityFromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	tz, err := h.userSvc.GetUserTimezone(r.Context(), id)
	if err != nil {
		log.Error("failed to get timezone", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	llmContext, err := h.userSvc.GetUserContext(r.Context(), id)
	if err != nil {
		log.Error("failed to get user context", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"timezone":    tz,
		"llm_context": llmContext,
	})
}

func (h *Handlers) UpdateMe(w http.ResponseWriter, r *http.Request) {
	log := logger.From(r.Context(), h.log)

	id, ok := apicontext.IdentityFromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body struct {
		Timezone   *string `json:"timezone"`
		LLMContext *string `json:"llm_context"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Timezone != nil {
		// Reject anything that is not a valid IANA zone. time.LoadLocation("")
		// silently returns UTC, which would let `{"timezone": ""}` clear the
		// stored zone — treat empty as invalid too.
		if *body.Timezone == "" {
			writeError(w, http.StatusBadRequest, "timezone must be a valid IANA zone")
			return
		}
		if _, err := time.LoadLocation(*body.Timezone); err != nil {
			writeError(w, http.StatusBadRequest, "timezone must be a valid IANA zone")
			return
		}
		if err := h.userSvc.UpdateUserTimezone(r.Context(), id, *body.Timezone); err != nil {
			log.Error("failed to update timezone", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}

	if body.LLMContext != nil {
		if err := h.userSvc.UpdateUserContext(r.Context(), id, *body.LLMContext); err != nil {
			var errTooLong *services.ErrContextTooLong
			if errors.As(err, &errTooLong) {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			log.Error("failed to update user context", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}

	writeOK(w)
}
