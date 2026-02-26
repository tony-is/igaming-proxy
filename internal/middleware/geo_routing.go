package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"igaming-proxy/internal/compliance"
	"igaming-proxy/internal/geoip"
)

type contextKey string

const (
	contextKeyRequestID contextKey = "request_id"
	contextKeyGeo       contextKey = "geo"
	contextKeyDecision  contextKey = "decision"
)

// generateUUID generates a new UUID string.
func generateUUID() string {
	return uuid.New().String()
}

// setRequestID stores the request ID in the context.
func setRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, contextKeyRequestID, id)
}

// getRequestID retrieves the request ID from the context.
func getRequestID(ctx context.Context) string {
	if v, ok := ctx.Value(contextKeyRequestID).(string); ok {
		return v
	}
	return ""
}

// getGeoResult retrieves the GeoResult from the context.
func getGeoResult(ctx context.Context) *geoip.GeoResult {
	if v, ok := ctx.Value(contextKeyGeo).(*geoip.GeoResult); ok {
		return v
	}
	return nil
}

// getDecision retrieves the RoutingDecision from the context.
func getDecision(ctx context.Context) *compliance.RoutingDecision {
	if v, ok := ctx.Value(contextKeyDecision).(*compliance.RoutingDecision); ok {
		return v
	}
	return nil
}

// extractClientIP extracts the real client IP from the request.
func extractClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// NewGeoRouting returns a middleware that detects the client's geo location.
func NewGeoRouting(detector *geoip.Detector) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := extractClientIP(r)
			geo, err := detector.Detect(ip)
			if err != nil {
				// Use a placeholder for unknown IPs (e.g., localhost)
				geo = &geoip.GeoResult{
					CountryCode: "XX",
					CountryName: "Unknown",
				}
			}
			ctx := context.WithValue(r.Context(), contextKeyGeo, geo)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// NewComplianceCheck returns a middleware that evaluates routing and compliance decisions.
func NewComplianceCheck(engine *compliance.Engine) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			geo := getGeoResult(r.Context())
			if geo == nil {
				geo = &geoip.GeoResult{CountryCode: "XX"}
			}

			productType := r.Header.Get("X-Product-Type")
			preferredProvider := r.Header.Get("X-Preferred-Provider")

			decision := engine.Evaluate(geo, productType, preferredProvider)
			ctx := context.WithValue(r.Context(), contextKeyDecision, decision)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
