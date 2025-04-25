# Go Remote Terminal Package

This package provides a WebSocket-based terminal implementation that can be integrated into web applications to provide terminal access.

## Features

- WebSocket-based communication for low-latency interaction
- PTY (pseudoterminal) support for proper terminal emulation
- Terminal output processing to handle control sequences
- Configurable terminal settings (shell, dimensions, environment)
- Clean termination of processes

## Installation

```bash
go get github.com/dansun78/go-remote-term/pkg/terminal
```

## Usage

### Basic Example

```go
package main

import (
	"log"
	"net/http"

	"github.com/dansun78/go-remote-term/pkg/terminal"
)

func main() {
	// Handle the WebSocket endpoint with default terminal settings
	http.HandleFunc("/terminal", terminal.HandleWebSocket)

	log.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
```

### Custom Terminal Options

```go
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/dansun78/go-remote-term/pkg/terminal"
)

func main() {
	// Create custom terminal options
	options := terminal.DefaultOptions()
	options.Shell = "/bin/zsh"
	options.InitialRows = 30
	options.InitialCols = 100
	options.Environment = append(options.Environment, "COLOR_PROMPT=1")

	// Handle the WebSocket endpoint with custom options
	http.HandleFunc("/terminal", func(w http.ResponseWriter, r *http.Request) {
		terminal.HandleWebSocketWithOptions(w, r, options)
	})

	log.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
```

### Custom WebSocket Settings

```go
package main

import (
	"log"
	"net/http"

	"github.com/dansun78/go-remote-term/pkg/terminal"
	"github.com/gorilla/websocket"
)

func main() {
	// Configure custom WebSocket upgrader with more restrictive CORS
	terminal.Upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			return origin == "https://myapp.example.com"
		},
	}

	// Handle the WebSocket endpoint
	http.HandleFunc("/terminal", terminal.HandleWebSocket)

	log.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
```

## Frontend Integration

To integrate with your frontend application, you need to implement a WebSocket client that connects to your terminal endpoint. Here's a basic example using JavaScript:

```javascript
const ws = new WebSocket('ws://localhost:8080/terminal');

// Handle incoming terminal output
ws.onmessage = function(event) {
  const terminalOutput = document.getElementById('terminal-output');
  terminalOutput.textContent += event.data;
  // Scroll to bottom
  terminalOutput.scrollTop = terminalOutput.scrollHeight;
};

// Handle user input
function sendCommand(command) {
  ws.send(command);
}

// Example: Send user input when they press Enter
document.getElementById('terminal-input').addEventListener('keydown', function(e) {
  if (e.key === 'Enter') {
    const input = this.value;
    sendCommand(input + '\n');
    this.value = '';
    e.preventDefault();
  }
});
```

## Dependencies

This package depends on:
- github.com/creack/pty - For PTY support
- github.com/gorilla/websocket - For WebSocket communication

## License

This package is released under the same license as go-remote-term.