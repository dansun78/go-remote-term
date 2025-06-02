package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/dansun78/go-remote-term/internal/logger"
	"github.com/dansun78/go-remote-term/internal/network"
	"github.com/dansun78/go-remote-term/internal/security"
	"github.com/dansun78/go-remote-term/pkg/middleware"
	"github.com/dansun78/go-remote-term/pkg/terminal"
)

// Version information that can be overridden during build
var (
	// These values will be overridden during build with ldflags
	AppName    = "go-remote-term"
	AppVersion = "dev"     // Default to "dev" if not specified at build time
	BuildDate  = "unknown" // Default to "unknown" if not specified at build time
	GoVersion  = "unknown" // Go compiler version used for building
)

var (
	addr           = flag.String("addr", ":8080", "HTTP service address")
	certFile       = flag.String("cert", "", "TLS cert file path")
	keyFile        = flag.String("key", "", "TLS key file path")
	secure         = flag.Bool("secure", false, "Force HTTPS usage (generates self-signed cert if not provided)")
	insecure       = flag.Bool("insecure", false, "Disable localhost-only restriction for HTTP mode (allows remote connections)")
	token          = flag.String("token", "", "Authentication token for accessing the terminal (if empty, a random token will be generated)")
	versionFlag    = flag.Bool("version", false, "Display version information")
	allowedOrigins = flag.String("allowed-origins", "", "Comma-separated list of allowed origins for CORS (default: localhost URLs only)")
)

// SecurityAuthProvider adapts our security package to the terminal.AuthProvider interface
type SecurityAuthProvider struct {
	authToken string
}

// ValidataAuthToken implements the terminal.AuthProvider interface
func (p *SecurityAuthProvider) ValidataAuthToken(token string) bool {
	return token == p.authToken
}

// TerminalHandler creates a handler for terminal WebSocket connections
func TerminalHandler(authToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Create terminal options with our auth provider
		opts := terminal.DefaultOptions()
		opts.AuthProvider = &SecurityAuthProvider{authToken: authToken}

		// Store the token in request context for compatibility with existing code
		ctx := context.WithValue(r.Context(), "auth_token", authToken)
		r = r.WithContext(ctx)

		// Handle the WebSocket connection with our configured options
		terminal.HandleWebSocketWithOptions(w, r, opts)
	}
}

func main() {
	flag.Parse()

	// Handle version flag
	if *versionFlag {
		fmt.Printf("%s v%s (built on %s with %s)\n", AppName, AppVersion, BuildDate, GoVersion)
		os.Exit(0)
	}

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
		fmt.Println("=====================================================")
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

	// Configure CORS allowed origins
	if *allowedOrigins != "" {
		// User provided custom origins, use them directly
		originsSlice := strings.Split(*allowedOrigins, ",")
		for i, origin := range originsSlice {
			originsSlice[i] = strings.TrimSpace(origin)
		}

		// We need to set allowed origins in both packages:
		// - security package handles CORS for regular HTTP requests
		// - terminal package handles CORS for WebSocket connections
		// This separation maintains proper package boundaries without creating circular dependencies
		terminal.SetAllowedOrigins(originsSlice)
		security.SetAllowedOrigins(originsSlice)

		fmt.Println("CORS allowed origins:", strings.Join(originsSlice, ", "))
	} else {
		// No custom origins provided, generate default based on configuration

		defaultOrigins := []string{}

		// Parse the address to determine hostname and port
		hostname := "localhost" // Default hostname
		port := "8080"          // Default port

		if strings.Contains(*addr, ":") {
			parts := strings.Split(*addr, ":")
			if len(parts) > 1 {
				port = parts[len(parts)-1]

				// If a specific hostname is provided (not empty or 0.0.0.0), use it
				if len(parts) > 1 && parts[0] != "" && parts[0] != "0.0.0.0" {
					hostname = parts[0]
				}
			}
		}

		// Add origin for the configured hostname first
		if *secure || *certFile != "" || *keyFile != "" {
			// HTTPS mode
			defaultOrigins = append(defaultOrigins, "https://"+hostname+":"+port)
		} else {
			// HTTP mode, potentially add both protocols
			defaultOrigins = append(defaultOrigins, "http://"+hostname+":"+port)
			// Also include HTTPS for compatibility with proxies
			defaultOrigins = append(defaultOrigins, "https://"+hostname+":"+port)
		}

		// Handle special case for 0.0.0.0 (all interfaces)
		// In this case, we need to provide more flexible CORS settings since
		// users might access the application via various hostnames or IPs
		if strings.HasPrefix(*addr, "0.0.0.0:") {
			// When binding to all interfaces, we should inform the user that they may need
			// to explicitly set allowed origins for proper security
			log.Println("WARNING: Binding to all interfaces (0.0.0.0). For production use,")
			log.Println("         consider explicitly setting allowed origins with --allowed-origins")

			// Get all local IP addresses and add them to allowed origins
			// This makes it possible to access the server from other devices on the network
			localIPs, err := network.GetLocalIPAddresses()
			if err != nil {
				log.Printf("Error getting local IP addresses: %v", err)
				log.Println("Only localhost origins will be allowed. Use --allowed-origins to add more.")
			} else if len(localIPs) > 0 {
				log.Printf("Found %d local IP addresses that can be used to access the server", len(localIPs))

				// Create additional default origins for each local IP
				for _, ip := range localIPs {
					// Add both HTTP and HTTPS origins for each IP
					if *secure || *certFile != "" || *keyFile != "" {
						// Only add HTTPS for secure mode
						defaultOrigins = append(defaultOrigins, "https://"+ip+":"+port)
					} else {
						// Add both for non-secure mode
						defaultOrigins = append(defaultOrigins, "http://"+ip+":"+port)
						defaultOrigins = append(defaultOrigins, "https://"+ip+":"+port)
					}
				}

				// Log the additional origins so users know what's available
				log.Println("The following local IPs can be used to access the server:")
				for _, ip := range localIPs {
					log.Printf("  - %s", ip)
				}
			}
		}

		// Set allowed origins for both WebSocket and HTTP endpoints
		terminal.SetAllowedOrigins(defaultOrigins)
		security.SetAllowedOrigins(defaultOrigins)

		fmt.Println("CORS allowed origins:", strings.Join(defaultOrigins, ", "))
	}

	// Serve embedded static files with middleware for security
	// Using the new middleware chaining approach with explicit definitions
	middlewareChain := []middleware.HandlerMiddleware{
		logger.RequestLoggerMiddleware,
		security.CORSMiddleware,
		security.AuthenticateMiddleware,
	}
	http.Handle("/", middleware.Chain(http.FileServer(http.FS(staticFS)), middlewareChain...))

	// Terminal WebSocket handler with middleware for security
	// The security middleware will handle authentication, but we also pass the token
	// to our TerminalHandler which will create the appropriate auth provider
	handlerMiddlewares := []middleware.FuncMiddleware{
		middleware.ConvertToFuncMiddleware(logger.RequestLoggerMiddleware),
		middleware.ConvertToFuncMiddleware(security.CORSMiddleware),
		middleware.ConvertToFuncMiddleware(security.AuthenticateMiddleware),
	}
	http.HandleFunc("/ws", middleware.ChainFunc(TerminalHandler(authToken), handlerMiddlewares...))

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
