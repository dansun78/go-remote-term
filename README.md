# Go Remote Terminal

A lightweight web-based terminal application that provides remote shell access via HTTP/HTTPS.

## Overview

Go Remote Terminal is a Go-based application that launches a shell and exposes it to remote users through a web interface. It uses WebSockets for real-time communication between the browser and the shell process, providing an interactive terminal experience in the browser.

## Features

- Real PTY (Pseudo Terminal) support for proper terminal emulation
- WebSocket-based communication for real-time interaction
- Support for both HTTP and HTTPS connections
- Interactive web terminal interface
- Single binary deployment with embedded web assets
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

By default, the server will start on port 8080 and only accept connections from localhost for security. You can access the terminal by opening a browser and navigating to:

```
http://localhost:8080
```

### Running with HTTP (allow remote connections)

To allow connections from any host (not just localhost):

```bash
./go-remote-term -insecure
```

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
- `-insecure`: Allow connections from any host, not just localhost (default: false)

## Security Considerations

The application includes built-in security measures:
- HTTP access is restricted to localhost by default
- Option to force HTTPS for all connections
- WebSocket connections follow the same security rules

For production use, consider implementing additional security:
- User authentication
- Access control
- IP filtering beyond localhost restriction
- Session timeouts

## Project Structure

```
go-remote-term/
├── assets.go            # Embeds static files into the binary
├── main.go              # Application entry point
├── internal/
│   └── terminal/
│       └── terminal.go  # Terminal handling and WebSocket logic
├── static/
│   ├── index.html       # Web interface HTML
│   ├── style.css        # Terminal styling
│   └── terminal.js      # Terminal frontend JavaScript
├── go.mod               # Go module definition
└── go.sum               # Go module checksums
```

## Dependencies

- [github.com/gorilla/websocket](https://github.com/gorilla/websocket) - WebSocket implementation
- [github.com/creack/pty](https://github.com/creack/pty) - Pseudo-terminal handling

## License

This project is open source and available under the [MIT License](LICENSE).

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.