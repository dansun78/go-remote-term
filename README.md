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

### Running with HTTP (default)

```bash
./go-remote-term
```

By default, the server will start on port 8080. You can access the terminal by opening a browser and navigating to:

```
http://localhost:8080
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

### Command Line Options

- `-addr`: HTTP/HTTPS service address (default: ":8080")
- `-cert`: TLS certificate file path (for HTTPS)
- `-key`: TLS key file path (for HTTPS)

## Security Considerations

This application is designed for development and controlled environments. For production use, consider implementing:

- User authentication
- Connection encryption (HTTPS)
- Access control
- IP filtering
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