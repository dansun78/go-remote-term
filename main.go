package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/dansun78/go-remote-term/internal/security"
	"github.com/dansun78/go-remote-term/pkg/terminal"
)

var (
	addr     = flag.String("addr", ":8080", "HTTP service address")
	certFile = flag.String("cert", "", "TLS cert file path")
	keyFile  = flag.String("key", "", "TLS key file path")
	secure   = flag.Bool("secure", false, "Force HTTPS usage (generates self-signed cert if not provided)")
	insecure = flag.Bool("insecure", false, "Allow connections from any host, not just localhost")
	token    = flag.String("token", "", "Authentication token for accessing the terminal (if empty, a random token will be generated)")
)

func main() {
	flag.Parse()

	// Set security configuration
	var authToken string
	if *token == "" {
		// Generate a random token if not provided
		randomToken, err := security.GenerateRandomToken()
		if err != nil {
			log.Fatalf("Failed to generate random token: %v", err)
		}
		authToken = randomToken
		// Log to stderr via standard logger
		log.Printf("Generated authentication token: %s", authToken)

		// Also print directly to stdout with clear formatting to make sure users see it
		fmt.Println("\n=====================================================")
		fmt.Println("AUTHENTICATION TOKEN (required to access terminal):")
		fmt.Printf("  %s\n", authToken)
		fmt.Println("=====================================================\n")
	} else {
		authToken = *token
		// Print confirmation that we're using the provided token
		fmt.Printf("Using provided authentication token: %s\n", authToken)
	}

	security.SetConfig(security.Config{
		InsecureMode: *insecure,
		AuthToken:    authToken,
	})

	// If secure mode is enabled but no cert/key provided, generate them
	if *secure && (*certFile == "" || *keyFile == "") {
		tempCert, tempKey, err := security.GenerateSelfSignedCert()
		if err != nil {
			log.Fatalf("Failed to generate self-signed certificate: %v", err)
		}
		*certFile = tempCert
		*keyFile = tempKey

		// Print certificate info to stdout
		fmt.Printf("Generated self-signed certificate: %s\n", *certFile)
		fmt.Printf("Generated private key: %s\n", *keyFile)
		fmt.Println("WARNING: Self-signed certificates are not secure for production use.")
		fmt.Println("         Please use proper certificates for production environments.")
	}

	// Get embedded static files
	staticFS, err := GetStaticFS()
	if err != nil {
		log.Fatal("Failed to access static files: ", err)
	}

	// Serve embedded static files with middleware for security
	http.Handle("/", security.Middleware(http.FileServer(http.FS(staticFS))))

	// Terminal WebSocket handler with middleware for security
	http.HandleFunc("/ws", security.Handler(terminal.HandleWebSocket))

	// Start the server
	fmt.Printf("Starting remote terminal server on %s\n", *addr)

	if *certFile != "" && *keyFile != "" {
		fmt.Println("Using HTTPS")
		err = http.ListenAndServeTLS(*addr, *certFile, *keyFile, nil)
	} else {
		if *secure {
			// This shouldn't be reached due to the earlier handling
			log.Fatal("HTTPS is required but certificate generation failed")
		}

		// Ensure localhost binding if needed
		*addr = security.EnsureLocalhostBinding(*addr)

		fmt.Println("WARNING: Using HTTP (insecure)")
		err = http.ListenAndServe(*addr, nil)
	}

	if err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
