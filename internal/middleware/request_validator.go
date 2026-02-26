package middleware

import (
	"net/http"
	"strings"
)

const maxRequestBodySize = 10 * 1024 * 1024 // 10MB

// NewRequestValidator returns a middleware that validates Content-Type and request size.
func NewRequestValidator() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
				ct := r.Header.Get("Content-Type")
				if !strings.HasPrefix(ct, "application/json") {
					http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
					return
				}
			}

			r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)

			next.ServeHTTP(w, r)
		})
	}
}
