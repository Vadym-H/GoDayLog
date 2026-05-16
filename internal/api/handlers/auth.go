package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/Vadym-H/GoDayLog/internal/api/telegram"
	"github.com/Vadym-H/GoDayLog/internal/domain"
	"github.com/Vadym-H/GoDayLog/internal/logger"
	"github.com/golang-jwt/jwt/v5"
)

type authResponse struct {
	Token string `json:"token"`
}

func (h *Handlers) MiniApp(w http.ResponseWriter, r *http.Request) {
	log := logger.From(r.Context(), h.log)

	var body struct {
		InitData string `json:"init_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.InitData == "" {
		writeError(w, http.StatusBadRequest, "init_data is required")
		return
	}

	tgID, err := telegram.ValidateMiniApp(body.InitData, h.cfg.Telegram.Token)
	if err != nil {
		if errors.Is(err, telegram.ErrAuthExpired) {
			writeError(w, http.StatusUnauthorized, "auth data expired")
			return
		}
		log.Warn("miniapp auth failed", "error", err)
		writeError(w, http.StatusUnauthorized, "invalid auth data")
		return
	}

	token, err := h.issueJWT(tgID)
	if err != nil {
		log.Error("failed to issue jwt", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	id := domain.Identity{Provider: "telegram", ExternalID: tgID}
	if _, _, err := h.userSvc.RegisterUser(r.Context(), id); err != nil {
		log.Error("failed to register user", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, authResponse{Token: token})
}

func (h *Handlers) Widget(w http.ResponseWriter, r *http.Request) {
	log := logger.From(r.Context(), h.log)

	// Accept any field Telegram sends (id, first_name, last_name, username,
	// photo_url, auth_date, hash, ...). A fixed struct would silently drop
	// unknown fields and break HMAC validation, since the hash is computed
	// over the full set Telegram sent.
	var body map[string]string
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	params := url.Values{}
	for k, v := range body {
		if v == "" {
			continue
		}
		params.Set(k, v)
	}

	tgID, err := telegram.ValidateWidget(params, h.cfg.Telegram.Token)
	if err != nil {
		if errors.Is(err, telegram.ErrAuthExpired) {
			writeError(w, http.StatusUnauthorized, "auth data expired")
			return
		}
		log.Warn("widget auth failed", "error", err)
		writeError(w, http.StatusUnauthorized, "invalid auth data")
		return
	}

	token, err := h.issueJWT(tgID)
	if err != nil {
		log.Error("failed to issue jwt", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	id := domain.Identity{Provider: "telegram", ExternalID: tgID}
	if _, _, err := h.userSvc.RegisterUser(r.Context(), id); err != nil {
		log.Error("failed to register user", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, authResponse{Token: token})
}

func (h *Handlers) issueJWT(tgID string) (string, error) {
	expiry := time.Duration(h.cfg.JWT.ExpiryHours) * time.Hour
	claims := jwt.RegisteredClaims{
		Subject:   tgID,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.cfg.JWT.Secret))
}
