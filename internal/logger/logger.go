package logger

import (
	"log"
	"net/http"
	"time"
)

// RequestLoggerMiddleware is a simple middleware that logs request information
func RequestLoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Call the next handler in the chain
		next.ServeHTTP(w, r)

		// Log request details after handler completes
		duration := time.Since(start)
		log.Printf("[%s] %s %s (took %v)", r.Method, r.URL.Path, r.RemoteAddr, duration)
	})
}
