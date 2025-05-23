package middleware_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dansun78/go-remote-term/pkg/middleware"
)

// Example middleware that adds a header
func headerMiddleware(headerName, headerValue string) middleware.HandlerMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(headerName, headerValue)
			next.ServeHTTP(w, r)
		})
	}
}

// Simple middleware that records it was called
func trackingMiddleware(callCounter *int) middleware.HandlerMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			*callCounter++
			next.ServeHTTP(w, r)
		})
	}
}

func TestMiddlewareChain(t *testing.T) {
	// Setup test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Hello, world!")
	})

	// Create counters to track middleware execution
	counter1 := 0
	counter2 := 0

	// Create middleware chain
	chain := middleware.New(
		headerMiddleware("X-Test", "test-value"),
		trackingMiddleware(&counter1),
		trackingMiddleware(&counter2),
	)

	// Apply middleware to handler
	wrappedHandler := middleware.Chain(handler, chain...)

	// Create test request
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	w := httptest.NewRecorder()

	// Serve the request
	wrappedHandler.ServeHTTP(w, req)

	// Check that both middleware were called in the right order
	if counter1 != 1 || counter2 != 1 {
		t.Errorf("Expected both middleware to be called once, got counter1=%d, counter2=%d", counter1, counter2)
	}

	// Check response
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Check the header was set
	if resp.Header.Get("X-Test") != "test-value" {
		t.Errorf("Expected X-Test header to be set to test-value, got %s", resp.Header.Get("X-Test"))
	}
}

func TestFuncMiddlewareChain(t *testing.T) {
	// Setup test handler function
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Hello from handler function!")
	}

	counter := 0

	// Convert regular middleware to func middleware
	funcMiddleware := middleware.ConvertToFuncMiddleware(trackingMiddleware(&counter))

	// Apply middleware to handler function
	wrappedHandlerFunc := middleware.ChainFunc(handler, funcMiddleware)

	// Create test request
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	w := httptest.NewRecorder()

	// Serve the request
	wrappedHandlerFunc(w, req)

	// Check that middleware was called
	if counter != 1 {
		t.Errorf("Expected middleware to be called once, got %d", counter)
	}

	// Check response
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}
}
