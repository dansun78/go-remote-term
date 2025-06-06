# Go Remote Terminal

A lightweight web-based terminal application that provides secure remote shell access via HTTP/HTTPS.

## Overview

Go Remote Terminal is a Go-based application that launches a shell and exposes it to remote users through a web interface. It uses WebSockets for real-time communication between the browser and the shell process, providing an interactive terminal experience in the browser.

## Features

- Real PTY (Pseudo Terminal) support for proper terminal emulation
- WebSocket-based communication for real-time interaction
- Token-based authentication system
- Persistent terminal sessions with reconnection capability
- Support for both HTTP and HTTPS connections (with automatic self-signed certificate generation)
- Interactive web terminal interface
- Single binary deployment with embedded web assets
- Automatic detection of network interfaces when binding to 0.0.0.0
- Smart CORS configuration for multi-device access
- Fiber-like middleware chaining for easy and understandable HTTP handler composition
- Support for common terminal features:
  - Command history (arrow keys)
  - Tab completion
  - Special key combinations (Ctrl+C, Ctrl+D, etc.)
  - Standard terminal output

## Installation

### Prerequisites

- Go 1.16 or higher
- Git

### Building from source

```bash
# Clone the repository
git clone https://github.com/dansun78/go-remote-term.git
cd go-remote-term

# Build the application
go build -o go-remote-term
```

## Usage

### Running with HTTP (default, localhost only)

```bash
./go-remote-term
```

By default, the server will start on port 8080 and only accept connections from localhost for security. A random authentication token will be generated and displayed in the console. You can access the terminal by opening a browser and navigating to:

```
http://localhost:8080
```

You will be prompted to enter the authentication token.

### Using a custom authentication token

To specify your own authentication token:

```bash
./go-remote-term -token="your-secure-token"
```

### Running with HTTP (allow remote connections)

To allow connections from any host (not just localhost):

```bash
./go-remote-term -insecure
```

### Binding to all network interfaces

To bind the server to all network interfaces:

```bash
./go-remote-term -addr=0.0.0.0:8080 -insecure
```

When binding to 0.0.0.0, the server will automatically detect all local network IP addresses and add them to the allowed CORS origins, making it easier to access the terminal from other devices on your network.

### Running with HTTPS

For secure connections, you need to provide TLS certificates:

```bash
./go-remote-term -addr=":8443" -cert=/path/to/cert.pem -key=/path/to/key.pem
```

Then access the terminal via:

```
https://localhost:8443
```

### Enforcing HTTPS

To enforce HTTPS usage (recommended for production):

```bash
./go-remote-term -secure -cert=/path/to/cert.pem -key=/path/to/key.pem
```

If you don't provide certificate and key files with the -secure flag, the application will automatically generate a self-signed certificate:

```bash
./go-remote-term -secure
```

**Note**: Browsers will display a security warning when using self-signed certificates. This is normal and you can proceed by accepting the risk. For production environments, use proper certificates from a trusted certificate authority.

### Command Line Options

- `-addr`: HTTP/HTTPS service address (default: ":8080")
- `-cert`: TLS certificate file path (for HTTPS)
- `-key`: TLS key file path (for HTTPS)
- `-secure`: Force HTTPS usage, generates self-signed cert if not provided (default: false)
- `-insecure`: Disable localhost-only restriction for HTTP mode (allows remote connections) (default: false)
- `-token`: Authentication token for accessing the terminal (if empty, a random token will be generated)
- `-allowed-origins`: Comma-separated list of allowed origins for CORS (default: auto-detected based on address)
- `-version`: Display version information

## Security Features

The application includes built-in security measures:
- HTTP access is restricted to localhost by default
- Token-based authentication system
- Login page for web access with token validation
- Option to force HTTPS for all connections (with automatic self-signed certificate generation)
- WebSocket connections follow the same security rules
- Terminal sessions with timeout for inactive connections
- Advanced CORS configuration with automatic detection of local network addresses

For production use, consider implementing additional security:
- Two-factor authentication 
- Access control based on user roles
- IP filtering beyond localhost restriction
- Audit logging

## CORS Configuration

When binding the server to all network interfaces (0.0.0.0), the application automatically detects all local IP addresses and adds them to the allowed CORS origins list. This makes it possible to access the terminal from any device on your local network.

For production environments, you should explicitly set the allowed origins for better security:

```bash
./go-remote-term -addr=0.0.0.0:8080 -insecure -allowed-origins="https://example.com,https://admin.example.com"
```

## Project Structure

```
go-remote-term/
├── .gitignore            # Git ignore rules
├── assets.go             # Embeds static files into the binary
├── build.sh              # Build script for different platforms
├── LICENSE               # MIT License
├── main.go               # Application entry point
├── Makefile              # Build automation
├── README.md             # Project documentation
├── version.conf          # Version configuration
├── internal/
│   ├── logger/
│   │   └── logger.go     # Logging utilities and middleware
│   ├── network/
│   │   └── network.go    # Network utilities for IP detection
│   └── security/
│       └── security.go   # Security implementation (auth, HTTPS)
├── pkg/
│   ├── middleware/
│   │   ├── chain.go      # Middleware chaining implementation
│   │   ├── chain_test.go # Unit tests for middleware chaining
│   │   └── README.md     # Middleware documentation
│   └── terminal/
│       ├── auth.go       # Authentication handling
│       ├── models.go     # Data models and structures
│       ├── session.go    # Terminal session management
│       ├── terminal.go   # Core terminal handling and PTY
│       ├── utils.go      # Utility functions
│       ├── websocket.go  # WebSocket communication logic
│       └── README.md     # Terminal package documentation
├── static/
│   ├── index.html        # Terminal interface HTML
│   ├── login.html        # Authentication page
│   ├── style.css         # Terminal and login styling
│   └── terminal.js       # Terminal frontend JavaScript
├── go.mod                # Go module definition
└── go.sum                # Go module checksums
```

## Third-Party Libraries

This project uses the following third-party libraries:

### Backend (Go)
- [github.com/gorilla/websocket](https://github.com/gorilla/websocket) - WebSocket implementation for Go (BSD 3-Clause License)
- [github.com/creack/pty](https://github.com/creack/pty) - Pseudo-terminal handling for Go (MIT License)
- [github.com/google/uuid](https://github.com/google/uuid) - UUID generation library (BSD 3-Clause License)

### Frontend (JavaScript)
- [xterm.js](https://github.com/xtermjs/xterm.js/) (v5.3.0) - A terminal emulator for the web (MIT License)
- [xterm-addon-fit](https://github.com/xtermjs/xterm.js/) (v0.8.0) - An add-on for xterm.js that enables resizing the terminal (MIT License)
- [Font Awesome](https://fontawesome.com/) (v6.5.1) - Icon library and toolkit (Free version uses MIT License for code, CC BY 4.0 License for icons, and SIL OFL 1.1 License for fonts)

## License

This project is open source and available under the [MIT License](LICENSE).

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Developer Guide

### Middleware Chaining

Go Remote Term uses a middleware chaining approach similar to the Fiber framework, making it easy to compose HTTP handlers with middleware:

```go
// Creating a middleware chain
middlewareChain := []security.MiddlewareFunc{
    logger.RequestLoggerMiddleware,
    security.CORSMiddleware,
    security.SecurityCheckMiddleware,
}

// Apply middleware chain to a handler
http.Handle("/", security.Chain(http.FileServer(http.FS(staticFS)), middlewareChain...))

// Working with handler functions
handlerMiddlewares := []security.HandlerMiddlewareFunc{
    security.ConvertToHandlerFunc(security.CORSMiddleware),
    security.ConvertToHandlerFunc(security.Middleware),
}
http.HandleFunc("/path", security.ChainHandlerFunc(myHandlerFunc, handlerMiddlewares...))

// Creating custom middleware chains
customMiddleware := security.ChainMiddleware(
    security.CORSMiddleware,
    security.Middleware,
    yourCustomMiddleware
)
http.Handle("/custom", security.Chain(yourHandler, customMiddleware...))
```

#### Creating Custom Middleware

You can easily create and add your own custom middleware:

```go
// Creating a request logging middleware
func RequestLoggerMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        
        // Call the next handler
        next.ServeHTTP(w, r)
        
        // Log after handling the request
        log.Printf("[%s] %s (took %v)", r.Method, r.URL.Path, time.Since(start))
    })
}

// Middleware that accepts parameters - a rate limiter example
func RateLimiterMiddleware(requestsPerMinute int) security.MiddlewareFunc {
    clients := make(map[string]*client)
    
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Rate limiting logic here...
            next.ServeHTTP(w, r)
        })
    }
}

// Combining built-in and custom middleware
myChain := security.ChainMiddleware(
    RequestLoggerMiddleware,
    RateLimiterMiddleware(60), // 60 requests per minute
    security.CORSMiddleware
)

http.Handle("/api", security.Chain(apiHandler, myChain...))
```

This approach makes it easier to understand and manage middleware composition compared to nested function calls, especially when working with multiple middleware components.