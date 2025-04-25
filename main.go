package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/dansun78/go-remote-term/internal/security"
	"github.com/dansun78/go-remote-term/pkg/terminal"
)

var (
	addr     = flag.String("addr", ":8080", "HTTP service address")
	certFile = flag.String("cert", "", "TLS cert file path")
	keyFile  = flag.String("key", "", "TLS key file path")
	secure   = flag.Bool("secure", false, "Force HTTPS usage (generates self-signed cert if not provided)")
	insecure = flag.Bool("insecure", false, "Allow connections from any host, not just localhost")
)

func main() {
	flag.Parse()

	// Set security configuration
	security.SetConfig(security.Config{
		InsecureMode: *insecure,
	})

	// If secure mode is enabled but no cert/key provided, generate them
	if *secure && (*certFile == "" || *keyFile == "") {
		tempCert, tempKey, err := security.GenerateSelfSignedCert()
		if err != nil {
			log.Fatalf("Failed to generate self-signed certificate: %v", err)
		}
		*certFile = tempCert
		*keyFile = tempKey
		log.Printf("Generated self-signed certificate: %s and key: %s", *certFile, *keyFile)
		log.Println("WARNING: Self-signed certificates are not secure for production use.")
		log.Println("         Please use proper certificates for production environments.")
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

	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
