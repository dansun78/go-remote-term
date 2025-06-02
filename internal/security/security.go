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
	InsecureMode bool   // Disable localhost-only restriction for HTTP mode (allows remote connections)
	AuthToken    string // Authentication token for session access
}

// Current security configuration, set by main.go
var config Config

// SetConfig updates the security configuration
func SetConfig(cfg Config) {
	config = cfg
}

// AllowedOrigins stores the list of origins that are allowed to connect and is used for CORS. They are initialized to allow localhost URLs by default.
// This can be updated by the SetAllowedOrigins function.
var AllowedOrigins = []string{
	"http://localhost:8080",
	"https://localhost:8080", // Match the default address regardless of protocol
}

// SetAllowedOrigins updates the list of origins allowed to connect
func SetAllowedOrigins(origins []string) {
	AllowedOrigins = origins
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
