package middleware

import (
	"net/http"
	"strings"
)

// NewAuth returns a middleware that validates API key or Bearer token.
func NewAuth(apiKey, jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check API key header
			if apiKey != "" {
				if r.Header.Get("X-API-Key") == apiKey {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Check Bearer token
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				token := strings.TrimPrefix(authHeader, "Bearer ")
				if validateJWT(token, jwtSecret) {
					next.ServeHTTP(w, r)
					return
				}
			}

			http.Error(w, "unauthorized", http.StatusUnauthorized)
		})
	}
}

// validateJWT is a placeholder for JWT validation logic.
func validateJWT(token, secret string) bool {
	// TODO: implement real JWT validation using a JWT library
	return token != "" && secret != ""
}
