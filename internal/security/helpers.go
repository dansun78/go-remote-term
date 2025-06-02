package security

import (
	"net/http"
	"strings"
)

// isHTTPS checks if the request is using HTTPS
func isHTTPS(r *http.Request) bool {
	return r.TLS != nil
}

// isLocalhost checks if the request is coming from localhost
func isLocalhost(r *http.Request) bool {
	host := r.Host
	if strings.Contains(host, ":") {
		host = strings.Split(host, ":")[0]
	}

	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// shouldRedirectToLogin determines if a request should be redirected to login
// based on the request type and headers
func shouldRedirectToLogin(r *http.Request) bool {
	// If it's an AJAX request or API call, don't redirect
	if r.Header.Get("X-Requested-With") == "XMLHttpRequest" {
		return false
	}

	// Check Accept header to see if browser is expecting HTML
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "text/html") {
		return true
	}

	// Browser initiated requests for pages typically need redirects
	return r.Method == "GET" && !strings.HasSuffix(r.URL.Path, ".js") &&
		!strings.HasSuffix(r.URL.Path, ".css") &&
		!strings.HasSuffix(r.URL.Path, ".png") &&
		!strings.HasSuffix(r.URL.Path, ".jpg") &&
		!strings.HasSuffix(r.URL.Path, ".ico")
}

// isOriginAllowed checks if the given origin is in the allowed list
func isOriginAllowed(origin string) bool {
	if origin == "" {
		return true // Same origin or non-browser client
	}

	for _, allowedOrigin := range AllowedOrigins {
		if origin == allowedOrigin {
			return true
		}
	}
	return false
}
