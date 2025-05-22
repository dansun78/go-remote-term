# Go Remote Terminal Package

This package provides a WebSocket-based terminal implementation that can be integrated into web applications to provide terminal access.

## Features

- WebSocket-based communication for low-latency interaction
- PTY (pseudoterminal) support for proper terminal emulation
- Terminal output processing to handle control sequences
- Configurable terminal settings (shell, dimensions, environment)
- Authentication with token-based access control
- Session persistence with reconnection support
- Clean termination of processes
- Flexible CORS configuration for multi-device access

## Package Structure

The terminal package is split into multiple files for better organization:

- `models.go` - Type definitions, interfaces, and data structures
- `auth.go` - Authentication functionality and token validation
- `session.go` - Session management and terminal process handling
- `websocket.go` - WebSocket connection management and CORS configuration
- `terminal.go` - Core public API functions
- `utils.go` - Helper functions for terminal output processing

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

### Authentication Example

```go
package main

import (
	"log"
	"net/http"

	"github.com/dansun78/go-remote-term/pkg/terminal"
)

func main() {
	// Create terminal options with authentication
	options := terminal.DefaultOptions()
	
	// Set an authentication token
	terminal.SetAuthToken(options, "your-secure-token")
	
	// Or implement your own auth provider
	// options.AuthProvider = &MyCustomAuthProvider{}
	
	// Handle the WebSocket endpoint with authenticated options
	http.HandleFunc("/terminal", func(w http.ResponseWriter, r *http.Request) {
		terminal.HandleWebSocketWithOptions(w, r, options)
	})

	log.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
```

### Custom CORS Configuration

```go
package main

import (
	"log"
	"net/http"

	"github.com/dansun78/go-remote-term/pkg/terminal"
	"github.com/gorilla/websocket"
)

func main() {
	// Set allowed origins for CORS
	terminal.SetAllowedOrigins([]string{
		"https://myapp.example.com",
		"http://localhost:3000",
	})

	// Or configure the WebSocket upgrader directly for more control
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

To integrate with your frontend application, you need to implement a WebSocket client that connects to your terminal endpoint. Here's an example using a modern JavaScript approach with authentication:

```javascript
// Initialize WebSocket connection with authentication
function connectTerminal(token, sessionId = null) {
  const ws = new WebSocket('ws://localhost:8080/terminal');
  
  ws.onopen = function() {
    // Send authentication message
    ws.send(JSON.stringify({
      type: 'auth',
      token: token,
      session_id: sessionId // Include previous session ID for reconnection, or null for new session
    }));
  };
  
  ws.onmessage = function(event) {
    // Try to parse as JSON first (for control messages)
    try {
      const jsonMsg = JSON.parse(event.data);
      
      if (jsonMsg.type === 'auth_response') {
        if (jsonMsg.success) {
          console.log('Authentication successful');
          // Store session ID for reconnection
          localStorage.setItem('terminal_session_id', jsonMsg.session_id);
        } else {
          console.error('Authentication failed:', jsonMsg.message);
        }
        return;
      }
      
      if (jsonMsg.type === 'session_ended') {
        console.log('Terminal session ended:', jsonMsg.message);
        return;
      }
      
      // Other JSON messages...
    } catch (e) {
      // Not JSON, treat as terminal output
      const terminalOutput = document.getElementById('terminal-output');
      terminalOutput.textContent += event.data;
      // Scroll to bottom
      terminalOutput.scrollTop = terminalOutput.scrollHeight;
    }
  };
  
  // Handle terminal resize
  function resizeTerminal(rows, cols) {
    ws.send(JSON.stringify({
      type: 'resize',
      rows: rows,
      cols: cols
    }));
  }
  
  // Handle regular input
  function sendCommand(command) {
    ws.send(command);
  }
  
  return { ws, sendCommand, resizeTerminal };
}

// Connect with authentication token
const terminal = connectTerminal('your-secure-token', localStorage.getItem('terminal_session_id'));

// Example: Handle user input
document.getElementById('terminal-input').addEventListener('keydown', function(e) {
  if (e.key === 'Enter') {
    const input = this.value;
    terminal.sendCommand(input + '\n');
    this.value = '';
    e.preventDefault();
  }
});
```

## Custom Authentication Provider

You can implement your own authentication provider by implementing the `AuthProvider` interface:

```go
package main

import (
	"database/sql"
	"log"
	"net/http"

	_ "github.com/mattn/go-sqlite3"
	"github.com/dansun78/go-remote-term/pkg/terminal"
)

// DBAuthProvider implements the terminal.AuthProvider interface
// to validate tokens against a database
type DBAuthProvider struct {
	DB *sql.DB
}

// ValidataAuthToken checks if the token is valid in the database
func (p *DBAuthProvider) ValidataAuthToken(token string) bool {
	var exists bool
	err := p.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM auth_tokens WHERE token = ? AND expired = 0)", token).Scan(&exists)
	if err != nil {
		log.Printf("Error validating token: %v", err)
		return false
	}
	return exists
}

func main() {
	// Open SQLite database
	db, err := sql.Open("sqlite3", "./auth.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	
	// Set up custom auth provider with database connection
	authProvider := &DBAuthProvider{DB: db}
	
	// Create terminal options with custom auth provider
	options := terminal.DefaultOptions()
	options.AuthProvider = authProvider
	
	// Handle the WebSocket endpoint with authenticated options
	http.HandleFunc("/terminal", func(w http.ResponseWriter, r *http.Request) {
		terminal.HandleWebSocketWithOptions(w, r, options)
	})
	
	log.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
```

## CORS Origin Settings

The terminal package provides two ways to handle CORS for WebSocket connections:

1. Using the `SetAllowedOrigins` function:
```go
terminal.SetAllowedOrigins([]string{
    "http://localhost:3000",
    "https://app.example.com",
})
```

2. By directly setting the WebSocket upgrader:
```go
terminal.Upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        // Your custom CORS logic here
        return true // Allow all origins
    },
}
```

## Dependencies

This package depends on:
- github.com/creack/pty - For PTY support
- github.com/gorilla/websocket - For WebSocket communication
- github.com/google/uuid - For session ID generation

## License

This package is released under the same license as go-remote-term.