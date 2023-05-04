package upload

import (
	"net/http"
	"time"

	"github.com/OdyseeTeam/odysee-api/pkg/logging"

	"github.com/go-chi/chi/v5/middleware"
)

type JSONLogger struct {
	logger logging.KVLogger
}

func (jl *JSONLogger) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		latency := time.Since(start)

		entry := []any{
			"status", ww.Status(),
			"method", r.Method,
			// "requestURI", r.RequestURI,
			"remote_addr", r.RemoteAddr,
			"latency", latency.String(),
			"bytes_out", ww.BytesWritten(),
		}

		jl.logger.Debug(r.URL.Path, entry...)
	})
}
