package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"igaming-proxy/internal/audit"
)

// responseRecorder captures status code and bytes written.
type responseRecorder struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.bytesWritten += n
	return n, err
}

// NewRequestID generates or propagates a request ID and stores it in context.
func NewRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateUUID()
		}
		ctx := setRequestID(r.Context(), requestID)
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// NewAuditLog logs every request with timing, status, client IP, and user agent.
func NewAuditLog(auditLogger *audit.Logger, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := newResponseRecorder(w)

			next.ServeHTTP(rec, r)

			duration := time.Since(start)
			requestID := getRequestID(r.Context())
			clientIP := extractClientIP(r)

			entry := audit.Entry{
				Timestamp: start,
				RequestID: requestID,
				Method:    r.Method,
				Path:      r.URL.Path,
				ClientIP:  clientIP,
				StatusCode: rec.statusCode,
				Duration:  duration,
				BytesSent: rec.bytesWritten,
				UserAgent: r.UserAgent(),
			}

			// Enrich with routing info if available
			if geo := getGeoResult(r.Context()); geo != nil {
				entry.CountryCode = geo.CountryCode
			}
			if decision := getDecision(r.Context()); decision != nil {
				entry.Region = decision.Region
				entry.ProviderID = decision.ProviderID
				entry.License = decision.License
				entry.Blocked = !decision.Allowed
				entry.BlockReason = decision.BlockReason
			}

			auditLogger.Log(entry)

			logger.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.statusCode,
				"duration_ms", duration.Milliseconds(),
				"client_ip", clientIP,
				"request_id", requestID,
			)
		})
	}
}
