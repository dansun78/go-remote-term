// Package middleware provides utilities for creating and managing HTTP middleware chains.
// Inspired by the Fiber framework's approach to middleware composition, this package
// makes it easy to define, organize, and apply middleware in a clear and concise way.
package middleware

import (
	"net/http"
)

// HandlerMiddleware defines the signature of middleware functions for http.Handler
type HandlerMiddleware func(http.Handler) http.Handler

// FuncMiddleware defines the signature of middleware functions for http.HandlerFunc
type FuncMiddleware func(http.HandlerFunc) http.HandlerFunc

// Chain creates a new middleware chain from the given middlewares
// and applies them to the provided handler in the order they were passed
func Chain(handler http.Handler, middlewares ...HandlerMiddleware) http.Handler {
	// Apply middlewares in reverse order so they execute in the order they were passed
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

// ChainFunc creates a new middleware chain from the given middlewares
// and applies them to the provided handler function in the order they were passed
func ChainFunc(handler http.HandlerFunc, middlewares ...FuncMiddleware) http.HandlerFunc {
	// Apply middlewares in reverse order so they execute in the order they were passed
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

// ConvertToFuncMiddleware converts a HandlerMiddleware to a FuncMiddleware
func ConvertToFuncMiddleware(middleware HandlerMiddleware) FuncMiddleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			middleware(next).ServeHTTP(w, r)
		}
	}
}

// New creates and returns a slice of middlewares that can be applied together
// This function is similar to the Fiber framework's approach where you define all middlewares together
func New(middlewares ...HandlerMiddleware) []HandlerMiddleware {
	return middlewares
}
