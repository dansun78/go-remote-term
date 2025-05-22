package security

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Key for token in request context
type contextKey string

const TokenContextKey contextKey = "auth_token"

// Config holds configuration options for security features
type Config struct {
	InsecureMode bool   // Allow connections from any host, not just localhost
	AuthToken    string // Authentication token for session access
}

// Current security configuration, set by main.go
var config Config

// SetConfig updates the security configuration
func SetConfig(cfg Config) {
	config = cfg
}

// GetAuthToken returns the configured authentication token
func GetAuthToken() string {
	return config.AuthToken
}

// GenerateRandomToken creates a UUIDv4 token for authentication
func GenerateRandomToken() (string, error) {
	// Generate a UUIDv4 (random UUID)
	tokenUUID, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("failed to generate UUIDv4 token: %v", err)
	}

	// Return the UUID as a string
	return tokenUUID.String(), nil
}

// GenerateSelfSignedCert creates a temporary self-signed certificate and key
// and returns their file paths
func GenerateSelfSignedCert() (string, string, error) {
	// Create a temporary directory for certificates
	tmpDir := filepath.Join(os.TempDir(), "go-remote-term-certs")
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		return "", "", fmt.Errorf("failed to create temp directory: %v", err)
	}

	// Generate a private key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %v", err)
	}

	// Generate a unique serial number
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate serial number: %v", err)
	}

	// Generate a certificate template
	now := time.Now()
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Go Remote Terminal Self-Signed"},
			CommonName:   "localhost",
		},
		NotBefore:             now,
		NotAfter:              now.Add(365 * 24 * time.Hour), // Valid for 1 year
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	// Add localhost and common IPs as Subject Alternative Names
	template.DNSNames = []string{"localhost"}
	template.IPAddresses = []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}

	// Create the certificate
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to create certificate: %v", err)
	}

	// Create certificate file path
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	// Save certificate to file
	certOut, err := os.Create(certFile)
	if err != nil {
		return "", "", fmt.Errorf("failed to create cert.pem: %v", err)
	}
	defer certOut.Close()

	// Write the certificate in PEM format
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return "", "", fmt.Errorf("failed to write certificate: %v", err)
	}

	// Save private key to file
	keyOut, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return "", "", fmt.Errorf("failed to create key.pem: %v", err)
	}
	defer keyOut.Close()

	// Convert the private key to PKCS8 format
	privBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal private key: %v", err)
	}

	// Write the private key in PEM format
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		return "", "", fmt.Errorf("failed to write private key: %v", err)
	}

	return certFile, keyFile, nil
}

// checkSecurity verifies if the request should be allowed based on security settings
func checkSecurity(w http.ResponseWriter, r *http.Request) (*http.Request, bool) {
	// If insecure flag is not set and we're using HTTP, check if request is from localhost
	if !config.InsecureMode && !IsHTTPS(r) && !IsLocalhost(r) {
		http.Error(w, "HTTP access restricted to localhost only", http.StatusForbidden)
		return r, false
	}

	// Store current token in request context for other handlers to access
	ctx := context.WithValue(r.Context(), TokenContextKey, config.AuthToken)
	r = r.WithContext(ctx)

	// If auth token is set, validate it
	if config.AuthToken != "" {
		// For WebSocket endpoints, don't check credentials here
		// We'll validate them after the WebSocket connection is established
		if strings.HasPrefix(r.URL.Path, "/ws") {
			// For WebSocket, we'll validate in the WebSocket handler
			return r, true
		}

		// For API endpoints, check Authorization header
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(r.URL.Path, "/api") {
			if authHeader != "Bearer "+config.AuthToken {
				http.Error(w, "Unauthorized: Invalid or missing token", http.StatusUnauthorized)
				return r, false
			}
			return r, true
		}

		// For web UI, check token parameter in URL or cookie
		tokenParam := r.URL.Query().Get("token")
		tokenCookie, err := r.Cookie("auth_token")

		// For specific login page, allow access without token
		if r.URL.Path == "/login.html" {
			return r, true
		}

		// For static resources needed by login page, allow access
		if r.URL.Path == "/style.css" {
			return r, true
		}

		// Check if token is valid
		isValidToken := (tokenParam == config.AuthToken || (err == nil && tokenCookie.Value == config.AuthToken))

		// If token is invalid, redirect to login page with error message
		if !isValidToken {
			// If it's a user-facing HTML request, redirect to login page with error
			if shouldRedirectToLogin(r) {
				// If an invalid token was explicitly provided (not just missing), show an error
				if tokenParam != "" || (err == nil && tokenCookie.Value != "") {
					http.Redirect(w, r, "/login.html?error=unauthorized", http.StatusFound)
				} else {
					// If token is just missing, redirect without error message
					http.Redirect(w, r, "/login.html", http.StatusFound)
				}
			} else {
				// For API requests or non-HTML resources, return standard 401 Unauthorized
				http.Error(w, "Unauthorized: Invalid or missing token", http.StatusUnauthorized)
			}
			return r, false
		}

		// If token is valid, set it as a cookie for future requests
		if tokenParam == config.AuthToken {
			http.SetCookie(w, &http.Cookie{
				Name:     "auth_token",
				Value:    config.AuthToken,
				HttpOnly: true,
				Secure:   IsHTTPS(r),
				Path:     "/",
				MaxAge:   3600 * 24, // 1 day
			})
		}
	}

	return r, true
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

// Middleware adds security checks to http handlers
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if newRequest, ok := checkSecurity(w, r); ok {
			next.ServeHTTP(w, newRequest)
		}
	})
}

// Handler adds security checks to http.HandlerFunc handlers
func Handler(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if newRequest, ok := checkSecurity(w, r); ok {
			next(w, newRequest)
		}
	}
}

// IsHTTPS checks if the request is using HTTPS
func IsHTTPS(r *http.Request) bool {
	return r.TLS != nil
}

// IsLocalhost checks if the request is coming from localhost
func IsLocalhost(r *http.Request) bool {
	host := r.Host
	if strings.Contains(host, ":") {
		host = strings.Split(host, ":")[0]
	}

	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// EnsureLocalhostBinding makes sure the address is bound to localhost if insecure mode is not enabled
// Returns the potentially modified address
func EnsureLocalhostBinding(addr string) string {
	// If insecure mode is enabled, return the original address
	if config.InsecureMode {
		return addr
	}

	// If address already specifies localhost, return as is
	if strings.HasPrefix(addr, "127.0.0.1:") || strings.HasPrefix(addr, "localhost:") {
		return addr
	}

	// Extract port from addr
	parts := strings.Split(addr, ":")
	port := "8080"
	if len(parts) > 1 {
		port = parts[len(parts)-1]
	}

	// Override with localhost
	localAddr := "127.0.0.1:" + port
	fmt.Printf("Restricting HTTP to localhost only, binding to %s\n", localAddr)
	return localAddr
}

// AllowedOrigins stores the list of origins that are allowed to connect
var AllowedOrigins = []string{
	"http://localhost:8080",
	"https://localhost:8080", // Match the default address regardless of protocol
}

// SetAllowedOrigins updates the list of origins allowed to connect
func SetAllowedOrigins(origins []string) {
	AllowedOrigins = origins
}

// IsOriginAllowed checks if the given origin is in the allowed list
func IsOriginAllowed(origin string) bool {
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

// CORSMiddleware adds CORS headers to responses
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the Origin header
		origin := r.Header.Get("Origin")

		// Only add CORS headers if the origin is present (cross-origin request)
		if origin != "" {
			// Check if the origin is allowed
			if IsOriginAllowed(origin) {
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

// SecureMiddleware combines security and CORS middlewares
func SecureMiddleware(next http.Handler) http.Handler {
	return CORSMiddleware(Middleware(next))
}

// SecureHandler combines security and CORS middlewares for http.HandlerFunc
func SecureHandler(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if newRequest, ok := checkSecurity(w, r); ok {
				next(w, newRequest)
			}
		})).ServeHTTP(w, r)
	}
}
