package middleware

import (
	"net/http"
	"strings"

	"github.com/Vadym-H/GoDayLog/internal/api/apicontext"
	"github.com/Vadym-H/GoDayLog/internal/config"
	"github.com/Vadym-H/GoDayLog/internal/domain"
	"github.com/golang-jwt/jwt/v5"
)

func Auth(cfg config.JWTConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				unauthorized(w)
				return
			}
			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(cfg.Secret), nil
			})
			if err != nil || !token.Valid {
				unauthorized(w)
				return
			}

			sub, err := token.Claims.GetSubject()
			if err != nil || sub == "" {
				unauthorized(w)
				return
			}

			id := domain.Identity{Provider: "telegram", ExternalID: sub}
			ctx := apicontext.WithIdentity(r.Context(), id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
}
