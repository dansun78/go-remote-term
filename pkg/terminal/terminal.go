// Package terminal provides WebSocket-based terminal functionality that can be
// integrated into web applications to provide terminal access.
package terminal

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/dansun78/go-remote-term/internal/security"
	"github.com/gorilla/websocket"
)

// Default WebSocket upgrader with permissive CORS settings
// Applications can use their own upgrader by setting terminal.Upgrader
var Upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all connections by default
	},
}

// TerminalOptions configures the behavior of the terminal session
type TerminalOptions struct {
	// Shell is the path to the shell executable (defaults to $SHELL or /bin/bash)
	Shell string

	// InitialRows sets the initial number of rows (default: 24)
	InitialRows uint16

	// InitialCols sets the initial number of columns (default: 80)
	InitialCols uint16

	// Environment variables to pass to the shell
	Environment []string
}

// Message represents WebSocket message structure
type Message struct {
	Type  string `json:"type"`
	Token string `json:"token,omitempty"`
}

// AuthResponse represents an authentication response message
type AuthResponse struct {
	Type    string `json:"type"`
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// DefaultOptions returns the default terminal options
func DefaultOptions() *TerminalOptions {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	return &TerminalOptions{
		Shell:       shell,
		InitialRows: 24,
		InitialCols: 80,
		Environment: []string{
			"TERM=dumb",                          // Use dumb terminal to disable most control sequences
			"PS1=\\w $ ",                         // Simple prompt without color codes
			"PROMPT_COMMAND=",                    // Disable prompt command
			"BASH_SILENCE_DEPRECATION_WARNING=1", // Silence bash deprecation warnings
			"LANG=C",                             // Use C locale for simpler output
			"PATH=" + os.Getenv("PATH"),          // Preserve PATH
			"HOME=" + os.Getenv("HOME"),          // Preserve HOME
			"USER=" + os.Getenv("USER"),          // Preserve USER
		},
	}
}

// HandleWebSocket handles WebSocket connections for terminal sessions with default options
func HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	HandleWebSocketWithOptions(w, r, DefaultOptions())
}

// HandleWebSocketWithOptions handles WebSocket connections for terminal sessions with custom options
func HandleWebSocketWithOptions(w http.ResponseWriter, r *http.Request, options *TerminalOptions) {
	conn, err := Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Failed to upgrade connection:", err)
		return
	}
	defer conn.Close()

	// Wait for authentication message
	_, rawMessage, err := conn.ReadMessage()
	if err != nil {
		log.Println("Failed to read authentication message:", err)
		return
	}

	// Parse the authentication message
	var msg Message
	if err := json.Unmarshal(rawMessage, &msg); err != nil {
		log.Println("Failed to parse authentication message:", err)
		authResp := AuthResponse{
			Type:    "auth_response",
			Success: false,
			Message: "Invalid authentication format",
		}
		respBytes, _ := json.Marshal(authResp)
		conn.WriteMessage(websocket.TextMessage, respBytes)
		return
	}

	if msg.Type != "auth" {
		log.Println("Expected auth message type but got:", msg.Type)
		authResp := AuthResponse{
			Type:    "auth_response",
			Success: false,
			Message: "Invalid message type",
		}
		respBytes, _ := json.Marshal(authResp)
		conn.WriteMessage(websocket.TextMessage, respBytes)
		return
	}

	// Get the configured auth token
	configuredToken := security.GetAuthToken()

	// Check token validity - client-provided token must not be empty
	if msg.Token == "" {
		authResp := AuthResponse{
			Type:    "auth_response",
			Success: false,
			Message: "Missing authentication token",
		}
		respBytes, _ := json.Marshal(authResp)
		conn.WriteMessage(websocket.TextMessage, respBytes)
		log.Println("Authentication failed: Missing token")
		return
	}

	// If we have a configured server token and it doesn't match the provided one, reject
	if configuredToken != "" && msg.Token != configuredToken {
		authResp := AuthResponse{
			Type:    "auth_response",
			Success: false,
			Message: "Invalid authentication token",
		}
		respBytes, _ := json.Marshal(authResp)
		conn.WriteMessage(websocket.TextMessage, respBytes)
		log.Println("Authentication failed: Invalid token")
		return
	}

	// Send successful authentication response
	authResp := AuthResponse{
		Type:    "auth_response",
		Success: true,
	}
	respBytes, _ := json.Marshal(authResp)
	if err := conn.WriteMessage(websocket.TextMessage, respBytes); err != nil {
		log.Println("Failed to send auth response:", err)
		return
	}

	// Create a new shell command with the specified options
	cmd := exec.Command(options.Shell)
	cmd.Env = options.Environment

	// Start the command with a pty
	ptmx, err := pty.Start(cmd)
	if err != nil {
		log.Println("Failed to start PTY:", err)
		return
	}
	defer ptmx.Close()

	// Set terminal size to specified dimensions
	pty.Setsize(ptmx, &pty.Winsize{
		Rows: options.InitialRows,
		Cols: options.InitialCols,
		X:    0,
		Y:    0,
	})

	// Small delay to allow terminal to initialize
	time.Sleep(100 * time.Millisecond)

	// Let's run a setup script to make sure the shell is properly configured
	setupCommands := []string{
		"stty echo\n",           // Enable local echo so user can see what they type
		"stty onlcr\n",          // Map NL to CR-NL on output
		"stty icrnl\n",          // Map CR to NL on input
		"stty opost\n",          // Enable output processing
		"export PS1='\\w $ '\n", // Set prompt explicitly again
		"clear\n",               // Clear the screen
	}

	// Clear any pending input
	discardBuf := make([]byte, 1024)
	ptmx.Read(discardBuf)

	// Send setup commands safely with delay between them
	for _, cmd := range setupCommands {
		_, err = ptmx.Write([]byte(cmd))
		if err != nil {
			log.Println("Error writing setup command:", err)
		}
		time.Sleep(50 * time.Millisecond)

		// Discard output from the commands
		ptmx.Read(discardBuf)
	}

	// Explicitly send a newline to force prompt display
	_, err = ptmx.Write([]byte("\n"))
	if err != nil {
		log.Println("Error triggering prompt:", err)
	}

	// Small delay before starting the I/O loops to ensure prompt shows up
	time.Sleep(100 * time.Millisecond)

	// Wait group to ensure all goroutines complete
	var wg sync.WaitGroup
	wg.Add(2)

	// Terminal output to WebSocket
	go func() {
		defer wg.Done()

		buf := make([]byte, 1024)
		for {
			n, err := ptmx.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Println("Error reading from PTY:", err)
				}
				break
			}

			// Process the output to remove problematic control sequences
			output := processTerminalOutput(buf[:n])

			// Only send if there's actual content
			if len(output) > 0 {
				if err := conn.WriteMessage(websocket.TextMessage, output); err != nil {
					log.Println("Error writing to WebSocket:", err)
					break
				}
			}
		}
	}()

	// WebSocket input to terminal
	go func() {
		defer wg.Done()

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Println("Error reading from WebSocket:", err)
				break
			}

			// Check if message is JSON (might be a control message)
			var jsonMsg map[string]interface{}
			if err := json.Unmarshal(message, &jsonMsg); err == nil {
				// This is a JSON message, not terminal input
				log.Println("Received JSON control message:", string(message))
				continue
			}

			if _, err := ptmx.Write(message); err != nil {
				log.Println("Error writing to PTY:", err)
				break
			}
		}

		// Signal the process to terminate
		cmd.Process.Signal(syscall.SIGTERM)
	}()

	// Wait for the command to finish
	wg.Wait()
	if err := cmd.Wait(); err != nil {
		// Don't log if process was terminated normally
		if err.Error() != "signal: terminated" {
			log.Println("Command exited with error:", err)
		}
	}
}

// ResizeTerminal resizes the terminal window
func ResizeTerminal(ptmx *os.File, rows, cols uint16) error {
	return pty.Setsize(ptmx, &pty.Winsize{
		Rows: rows,
		Cols: cols,
		X:    0,
		Y:    0,
	})
}

// processTerminalOutput filters problematic control sequences from terminal output
func processTerminalOutput(data []byte) []byte {
	// Convert to string for easier manipulation
	str := string(data)

	// Filter out problematic control sequences
	replacePatterns := []string{
		"\x1b[?2004h", "\x1b[?2004l", // Bracketed paste mode
		"\x1b[?1049h", "\x1b[?1049l", // Alternate screen buffer
		"\x1b[?1h", "\x1b=", // Application cursor keys
		"\x1b[?12h", "\x1b[?12l", // Cursor blinking
	}

	for _, pattern := range replacePatterns {
		str = replaceAllStringLiteral(str, pattern, "")
	}

	// Return as bytes
	return []byte(str)
}

// replaceAllStringLiteral is a helper function to replace all occurrences
// of a literal string without regex interpretation
func replaceAllStringLiteral(s, old, new string) string {
	if old == "" {
		return s // Avoid infinite loop for empty old string
	}

	result := ""
	for {
		i := indexOf(s, old)
		if i == -1 {
			return result + s
		}
		result += s[:i] + new
		s = s[i+len(old):]
	}
}

// indexOf is a helper function to find the index of a substring
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
