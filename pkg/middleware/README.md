# Go Middleware Chain

A lightweight, flexible middleware chaining library for Go HTTP services, inspired by the Fiber framework's approach to middleware management.

## Features

- Simple, intuitive API for creating and applying middleware chains
- Support for both `http.Handler` and `http.HandlerFunc` middleware types
- Correct execution order that matches the order middleware is added
- Zero dependencies beyond the Go standard library

## Installation

```bash
go get github.com/dansun78/go-remote-term/pkg/middleware
```

## Usage

### Basic Usage

```go
package main

import (
	"log"
	"net/http"

	"github.com/dansun78/go-remote-term/pkg/middleware"
)

// LoggerMiddleware is a simple logging middleware
func LoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Request: %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

// AuthMiddleware is a simple authentication middleware
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	// Create a middleware chain
	chain := middleware.New(
		LoggerMiddleware,   // Your custom logger middleware
		AuthMiddleware,     // Your custom auth middleware
	)
	
	// Apply middleware chain to a handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World!"))
	})
	
	http.Handle("/", middleware.Chain(handler, chain...))
	http.ListenAndServe(":8080", nil)
}
```

### Working with http.HandlerFunc

```go
func MyHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Hello from my handler"))
}

// Define function middleware
func LoggingFuncMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Request: %s %s", r.Method, r.URL.Path)
		next(w, r)
	}
}

// Convert standard middleware to func middleware
funcMiddlewares := []middleware.FuncMiddleware{
	middleware.ConvertToFuncMiddleware(LoggerMiddleware), // Convert from handler middleware
	LoggingFuncMiddleware, // Direct func middleware
}

// Apply to a HandlerFunc
http.HandleFunc("/api", middleware.ChainFunc(MyHandler, funcMiddlewares...))
```

### Creating Parameterized Middleware

```go
// Create a middleware that accepts parameters
func RateLimiter(requestsPerMinute int) middleware.HandlerMiddleware {
	// Set up rate limiting logic...
	
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Apply rate limiting logic based on parameters
			// ...
			
			// If request is allowed:
			next.ServeHTTP(w, r)
		})
	}
}

// Use in a chain
chain := middleware.New(
	LoggerMiddleware,
	RateLimiter(60), // 60 requests per minute
)
```

## API Reference

- `Chain(handler, ...middleware)`: Chains middleware with an http.Handler
- `ChainFunc(handlerFunc, ...funcMiddleware)`: Chains middleware with an http.HandlerFunc
- `New(...middleware)`: Creates a reusable middleware stack
- `ConvertToFuncMiddleware(middleware)`: Converts handler middleware to func middleware 

## License

This package is released under the same license as go-remote-term.
