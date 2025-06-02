package security

import (
	"context"
	"net/http"
	"strings"
)

// AuthenticateMiddleware authenticates incoming HTTP requests
func AuthenticateMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If insecure flag is not set and we're using HTTP, check if request is from localhost
		if !config.InsecureMode && !isHTTPS(r) && !isLocalhost(r) {
			http.Error(w, "HTTP access restricted to localhost only", http.StatusForbidden)
			return
		}

		// Store current token in request context for other handlers to access
		ctx := context.WithValue(r.Context(), TokenContextKey, config.AuthToken)
		newRequest := r.WithContext(ctx)

		// If auth token is set, validate it
		if config.AuthToken != "" {
			// For WebSocket endpoints, don't check credentials here
			// We'll validate them after the WebSocket connection is established
			if strings.HasPrefix(newRequest.URL.Path, "/ws") {
				// For WebSocket, we'll validate in the WebSocket handler
				next.ServeHTTP(w, newRequest)
				return
			}

			// For API endpoints, check Authorization header
			authHeader := newRequest.Header.Get("Authorization")
			if strings.HasPrefix(newRequest.URL.Path, "/api") {
				if authHeader != "Bearer "+config.AuthToken {
					http.Error(w, "Unauthorized: Invalid or missing token", http.StatusUnauthorized)
					return
				}
				next.ServeHTTP(w, newRequest)
				return
			}

			// For web UI, check token parameter in URL or cookie
			tokenParam := newRequest.URL.Query().Get("token")
			tokenCookie, err := newRequest.Cookie("auth_token")

			// For specific login page, allow access without token
			if newRequest.URL.Path == "/login.html" {
				next.ServeHTTP(w, newRequest)
				return
			}

			// For static resources needed by login page, allow access
			if newRequest.URL.Path == "/style.css" {
				next.ServeHTTP(w, newRequest)
				return
			}

			// Check if token is valid
			isValidToken := (tokenParam == config.AuthToken || (err == nil && tokenCookie.Value == config.AuthToken))

			// If token is invalid, redirect to login page with error message
			if !isValidToken {
				// If it's a user-facing HTML request, redirect to login page with error
				if shouldRedirectToLogin(newRequest) {
					// If an invalid token was explicitly provided (not just missing), show an error
					if tokenParam != "" || (err == nil && tokenCookie.Value != "") {
						http.Redirect(w, newRequest, "/login.html?error=unauthorized", http.StatusFound)
					} else {
						// If token is just missing, redirect without error message
						http.Redirect(w, newRequest, "/login.html", http.StatusFound)
					}
				} else {
					// For API requests or non-HTML resources, return standard 401 Unauthorized
					http.Error(w, "Unauthorized: Invalid or missing token", http.StatusUnauthorized)
				}
				return
			}

			// If token is valid, set it as a cookie for future requests
			if tokenParam == config.AuthToken {
				http.SetCookie(w, &http.Cookie{
					Name:     "auth_token",
					Value:    config.AuthToken,
					HttpOnly: true,
					Secure:   isHTTPS(newRequest),
					Path:     "/",
					MaxAge:   3600 * 24, // 1 day
				})
			}
		}

		next.ServeHTTP(w, newRequest)
	})
}

// CORSMiddleware adds CORS headers to responses
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the Origin header
		origin := r.Header.Get("Origin")

		// Only add CORS headers if the origin is present (cross-origin request)
		if origin != "" {
			// Check if the origin is allowed
			if isOriginAllowed(origin) {
				// Set CORS headers for allowed origins
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

				// Handle preflight requests
				if r.Method == "OPTIONS" {
					w.WriteHeader(http.StatusOK)
					return
				}
			} else {
				// For disallowed origins, don't add CORS headers
				// This will cause browsers to block the request
				if r.Method == "OPTIONS" {
					http.Error(w, "CORS origin not allowed", http.StatusForbidden)
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}
