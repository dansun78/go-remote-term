package security

import (
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
)

// Config holds configuration options for security features
type Config struct {
	InsecureMode bool // Allow connections from any host, not just localhost
}

// Current security configuration, set by main.go
var config Config

// SetConfig updates the security configuration
func SetConfig(cfg Config) {
	config = cfg
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
func checkSecurity(w http.ResponseWriter, r *http.Request) bool {
	// If insecure flag is not set and we're using HTTP, check if request is from localhost
	if !config.InsecureMode && !IsHTTPS(r) && !IsLocalhost(r) {
		http.Error(w, "HTTP access restricted to localhost only", http.StatusForbidden)
		return false
	}
	return true
}

// Middleware adds security checks to http handlers
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if checkSecurity(w, r) {
			next.ServeHTTP(w, r)
		}
	})
}

// Handler adds security checks to http.HandlerFunc handlers
func Handler(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if checkSecurity(w, r) {
			next(w, r)
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
