package middleware

import (
	"net/http"

	"github.com/Vadym-H/GoDayLog/internal/logger"
)

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := logger.WithRequestID(r.Context(), logger.NewRequestID())
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
