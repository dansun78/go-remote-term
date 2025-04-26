// Package terminal provides WebSocket-based terminal functionality that can be
// integrated into web applications to provide terminal access.
package terminal

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	"github.com/google/uuid"
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

	// SessionTimeout defines how long to keep a disconnected session alive (default: 10 minutes)
	SessionTimeout time.Duration
}

// Terminal session messages
type Message struct {
	Type      string `json:"type"`
	Token     string `json:"token,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Data      string `json:"data,omitempty"`
	Rows      uint16 `json:"rows,omitempty"`
	Cols      uint16 `json:"cols,omitempty"`
}

// Response messages
type Response struct {
	Type      string `json:"type"`
	Success   bool   `json:"success"`
	Message   string `json:"message,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

// TerminalSession represents an active terminal session
type TerminalSession struct {
	ID           string
	PTY          *os.File
	Command      *exec.Cmd
	Options      *TerminalOptions
	OutputBuffer *bytes.Buffer
	LastActive   time.Time
	Connections  int
	Lock         sync.Mutex
	Done         chan struct{}
}

// Global session manager
var (
	sessions     = make(map[string]*TerminalSession)
	sessionsLock sync.Mutex
)

// DefaultOptions returns the default terminal options
func DefaultOptions() *TerminalOptions {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	return &TerminalOptions{
		Shell:          shell,
		InitialRows:    24,
		InitialCols:    80,
		SessionTimeout: 10 * time.Minute, // Keep sessions alive for 10 minutes by default
		Environment: []string{
			"TERM=xterm-256color",                // Use xterm-256color instead of dumb for better control sequence support
			"PS1=\\w $ ",                         // Simple prompt without color codes
			"PROMPT_COMMAND=",                    // Disable prompt command
			"BASH_SILENCE_DEPRECATION_WARNING=1", // Silence bash deprecation warnings
			"LANG=en_US.UTF-8",                   // Use UTF-8 locale for better character support
			"PATH=" + os.Getenv("PATH"),          // Preserve PATH
			"HOME=" + os.Getenv("HOME"),          // Preserve HOME
			"USER=" + os.Getenv("USER"),          // Preserve USER
			"COLORTERM=truecolor",                // Enable full color support
		},
	}
}

// HandleWebSocket handles WebSocket connections for terminal sessions with default options
func HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	HandleWebSocketWithOptions(w, r, DefaultOptions())
}

// startSessionCleanupRoutine starts a background routine to cleanup expired sessions
func init() {
	go func() {
		for {
			time.Sleep(1 * time.Minute)
			cleanupExpiredSessions()
		}
	}()
}

// cleanupExpiredSessions removes sessions that have been inactive longer than their timeout
func cleanupExpiredSessions() {
	sessionsLock.Lock()
	defer sessionsLock.Unlock()

	now := time.Now()
	for id, session := range sessions {
		session.Lock.Lock()
		lastActive := session.LastActive
		connections := session.Connections
		session.Lock.Unlock()

		// If session has no active connections and has exceeded timeout
		if connections == 0 && now.Sub(lastActive) > session.Options.SessionTimeout {
			log.Printf("Cleaning up expired session %s (inactive for %v)", id, now.Sub(lastActive))
			closeSession(id)
		}
	}
}

// closeSession terminates and removes a session
func closeSession(sessionID string) {
	sessionsLock.Lock()
	defer sessionsLock.Unlock()

	session, exists := sessions[sessionID]
	if !exists {
		return
	}

	// Signal the terminal process to terminate
	if session.Command != nil && session.Command.Process != nil {
		session.Command.Process.Signal(syscall.SIGTERM)
	}

	// Close the PTY if it exists
	if session.PTY != nil {
		session.PTY.Close()
	}

	// Signal done channel
	close(session.Done)

	// Remove from sessions map
	delete(sessions, sessionID)
}

// terminateSession explicitly terminates a session by ID
func terminateSession(sessionID string) bool {
	if sessionID == "" {
		return false
	}

	sessionsLock.Lock()
	_, exists := sessions[sessionID]
	sessionsLock.Unlock()

	if !exists {
		return false
	}

	log.Printf("Explicitly terminating session %s at user request", sessionID)
	closeSession(sessionID)
	return true
}

// createNewSession initializes a new terminal session
func createNewSession(options *TerminalOptions) (*TerminalSession, error) {
	// Generate a unique session ID
	sessionID := uuid.New().String()

	// Create a new shell command with the specified options
	cmd := exec.Command(options.Shell)
	cmd.Env = options.Environment

	// Start the command with a pty
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to start PTY: %v", err)
	}

	// Set terminal size to specified dimensions
	pty.Setsize(ptmx, &pty.Winsize{
		Rows: options.InitialRows,
		Cols: options.InitialCols,
		X:    0,
		Y:    0,
	})

	// Initialize the terminal session
	session := &TerminalSession{
		ID:           sessionID,
		PTY:          ptmx,
		Command:      cmd,
		Options:      options,
		OutputBuffer: new(bytes.Buffer),
		LastActive:   time.Now(),
		Connections:  0,
		Done:         make(chan struct{}),
	}

	// Configure the terminal
	configureTerminal(session)

	// Store in the global sessions map
	sessionsLock.Lock()
	sessions[sessionID] = session
	sessionsLock.Unlock()

	// Start output buffer routine
	go bufferTerminalOutput(session)

	return session, nil
}

// configureTerminal sets up the terminal with proper settings
func configureTerminal(session *TerminalSession) {
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
	session.PTY.Read(discardBuf)

	// Send setup commands safely with delay between them
	for _, cmd := range setupCommands {
		_, err := session.PTY.Write([]byte(cmd))
		if err != nil {
			log.Println("Error writing setup command:", err)
		}
		time.Sleep(50 * time.Millisecond)

		// Discard output from the commands
		session.PTY.Read(discardBuf)
	}

	// Explicitly send a newline to force prompt display
	_, err := session.PTY.Write([]byte("\n"))
	if err != nil {
		log.Println("Error triggering prompt:", err)
	}

	// Small delay before starting the I/O loops to ensure prompt shows up
	time.Sleep(100 * time.Millisecond)
}

// bufferTerminalOutput continuously reads from the PTY and adds it to the buffer
func bufferTerminalOutput(session *TerminalSession) {
	buf := make([]byte, 1024)
	for {
		select {
		case <-session.Done:
			return
		default:
			n, err := session.PTY.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Printf("Error reading from PTY (session %s): %v", session.ID, err)
				} else {
					log.Printf("Shell exited for session %s (EOF detected)", session.ID)
				}

				// When we get EOF or any other error, the shell has likely exited
				// Send notification first, then terminate the session
				go func(sessionID string, s *TerminalSession) {
					log.Printf("Broadcasting shell exit notification for session %s", sessionID)

					// Create termination notification message
					notification := Response{
						Type:      "session_ended",
						Success:   false,
						Message:   "Shell process has exited",
						SessionID: sessionID,
					}

					notificationBytes, _ := json.Marshal(notification)

					// Send the notification through the session's output buffer
					// while it still exists
					s.Lock.Lock()
					connections := s.Connections

					if connections > 0 && s.OutputBuffer != nil {
						log.Printf("Broadcasting to %d connections", connections)
						// We'll wrap our JSON in a special marker so it's recognized as JSON
						// This is needed because we're adding it to the output buffer
						wrappedMessage := append([]byte("\n<JSON>"), notificationBytes...)
						wrappedMessage = append(wrappedMessage, []byte("</JSON>\n")...)
						s.OutputBuffer.Write(wrappedMessage)
					}
					s.Lock.Unlock()

					// Give some time for notification to be sent before terminating session
					time.Sleep(500 * time.Millisecond)

					// Now terminate the session
					log.Printf("Automatically terminating session %s due to shell exit", sessionID)
					terminateSession(sessionID)
				}(session.ID, session)

				return
			}

			// Process the output to remove problematic control sequences
			output := processTerminalOutput(buf[:n])

			// Add to the buffer and update last active time
			if len(output) > 0 {
				session.Lock.Lock()
				session.OutputBuffer.Write(output)
				session.LastActive = time.Now()
				session.Lock.Unlock()
			}
		}
	}
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
		sendErrorResponse(conn, "Invalid authentication format")
		return
	}

	// Handle authentication
	if msg.Type != "auth" {
		log.Println("Expected auth message type but got:", msg.Type)
		sendErrorResponse(conn, "Invalid message type")
		return
	}

	// Get the configured auth token
	configuredToken := security.GetAuthToken()

	// Check token validity - client-provided token must not be empty
	if msg.Token == "" {
		sendErrorResponse(conn, "Missing authentication token")
		log.Println("Authentication failed: Missing token")
		return
	}

	// If we have a configured server token and it doesn't match the provided one, reject
	if configuredToken != "" && msg.Token != configuredToken {
		sendErrorResponse(conn, "Invalid authentication token")
		log.Println("Authentication failed: Invalid token")
		return
	}

	// At this point user is authenticated

	var session *TerminalSession
	var isNewSession bool

	// Check if client is requesting reconnection to existing session
	if msg.SessionID != "" {
		sessionsLock.Lock()
		existingSession, exists := sessions[msg.SessionID]
		sessionsLock.Unlock()

		if exists {
			session = existingSession
			isNewSession = false
			log.Printf("Reconnecting to existing session: %s", session.ID)
		} else {
			log.Printf("Requested session %s not found, creating new session", msg.SessionID)
			isNewSession = true
		}
	} else {
		isNewSession = true
	}

	// Create new session if needed
	if isNewSession {
		newSession, err := createNewSession(options)
		if err != nil {
			sendErrorResponse(conn, fmt.Sprintf("Failed to create terminal: %v", err))
			return
		}
		session = newSession
		log.Printf("Created new terminal session: %s", session.ID)
	}

	// Send successful authentication response with session ID
	authResp := Response{
		Type:      "auth_response",
		Success:   true,
		SessionID: session.ID,
	}
	respBytes, _ := json.Marshal(authResp)
	if err := conn.WriteMessage(websocket.TextMessage, respBytes); err != nil {
		log.Println("Failed to send auth response:", err)
		return
	}

	// Increment connection count
	session.Lock.Lock()
	session.Connections++

	// If reconnecting, send buffer contents
	if !isNewSession && session.OutputBuffer.Len() > 0 {
		bufferContents := session.OutputBuffer.Bytes()
		session.Lock.Unlock()

		// Send current buffer contents to client for session continuity
		err := conn.WriteMessage(websocket.TextMessage, bufferContents)
		if err != nil {
			log.Printf("Error sending buffer to client: %v", err)
		}
	} else {
		session.Lock.Unlock()
	}

	// Handle WebSocket connection for this session
	handleTerminalConnection(conn, session)
}

// handleTerminalConnection manages a WebSocket connection for an existing terminal session
func handleTerminalConnection(conn *websocket.Conn, session *TerminalSession) {
	// Wait group for connection handling goroutines
	var wg sync.WaitGroup
	wg.Add(2)

	// Channel to signal when this connection is closed
	connClosed := make(chan struct{})

	// Forward terminal output to the WebSocket
	go func() {
		defer wg.Done()

		lastSize := 0
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-connClosed:
				return
			case <-session.Done:
				return
			case <-ticker.C:
				// Check if there's new output to send
				session.Lock.Lock()
				currentSize := session.OutputBuffer.Len()
				if currentSize > lastSize {
					// Only send the new part since last check
					bufferContents := session.OutputBuffer.Bytes()[lastSize:currentSize]
					session.Lock.Unlock()

					if err := conn.WriteMessage(websocket.TextMessage, bufferContents); err != nil {
						log.Println("Error writing to WebSocket:", err)
						return
					}

					lastSize = currentSize
				} else {
					session.Lock.Unlock()
				}
			}
		}
	}()

	// WebSocket input to terminal
	go func() {
		defer wg.Done()
		defer close(connClosed)

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Printf("WebSocket connection closed: %v", err)
				break
			}

			// Check if message is JSON (might be a control message)
			var jsonMsg Message
			if err := json.Unmarshal(message, &jsonMsg); err == nil {
				// Handle control messages
				if jsonMsg.Type == "resize" && jsonMsg.Rows > 0 && jsonMsg.Cols > 0 {
					// Resize the terminal
					ResizeTerminal(session.PTY, jsonMsg.Rows, jsonMsg.Cols)
					continue
				}

				// Handle terminate session request
				if jsonMsg.Type == "terminate" && jsonMsg.SessionID == session.ID {
					// Send acknowledgment before terminating
					resp := Response{
						Type:      "terminate_response",
						Success:   true,
						Message:   "Session terminated",
						SessionID: session.ID,
					}
					respBytes, _ := json.Marshal(resp)
					conn.WriteMessage(websocket.TextMessage, respBytes)

					// Schedule termination (do it after response is sent)
					go func() {
						time.Sleep(100 * time.Millisecond) // Brief delay to allow response to be sent
						terminateSession(session.ID)
					}()
					continue
				}

				// Handle other control messages
				log.Println("Received JSON control message:", string(message))
				continue
			}

			// For normal input, write to PTY
			session.Lock.Lock()
			session.LastActive = time.Now()
			_, err = session.PTY.Write(message)
			session.Lock.Unlock()

			if err != nil {
				log.Println("Error writing to PTY:", err)
				break
			}
		}
	}()

	// Wait for connection handling to complete
	wg.Wait()

	// Decrement connection count when this connection ends
	session.Lock.Lock()
	session.Connections--
	session.LastActive = time.Now()
	session.Lock.Unlock()

	log.Printf("WebSocket connection closed for session %s, remaining connections: %d",
		session.ID, session.Connections)

	// Note: We don't automatically close the session here to allow reconnection
}

// sendErrorResponse sends an error response to the client
func sendErrorResponse(conn *websocket.Conn, message string) {
	resp := Response{
		Type:    "auth_response",
		Success: false,
		Message: message,
	}
	respBytes, _ := json.Marshal(resp)
	conn.WriteMessage(websocket.TextMessage, respBytes)
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

// broadcastSessionTerminated notifies all connected clients that a session has ended
func broadcastSessionTerminated(sessionID string) {
	sessionsLock.Lock()
	session, exists := sessions[sessionID]
	if !exists {
		sessionsLock.Unlock()
		return
	}

	// Create termination notification message
	notification := Response{
		Type:      "session_ended",
		Success:   false,
		Message:   "Shell process has exited",
		SessionID: sessionID,
	}

	notificationBytes, _ := json.Marshal(notification)

	// Keep track of the session's connection count - we'll access it with the lock released
	connections := session.Connections
	sessionsLock.Unlock()

	// If there are active connections, we need to notify them
	if connections > 0 {
		log.Printf("Broadcasting shell exit notification for session %s to %d connections",
			sessionID, connections)

		// We need to send the termination notification through the PTY output buffer
		// This is a bit of a hack, but it ensures that all connected clients receive it
		// through their normal WebSocket output channel
		session.Lock.Lock()
		if session.OutputBuffer != nil {
			// We'll wrap our JSON in a special marker so it's recognized as JSON
			// This is needed because we're adding it to the output buffer
			wrappedMessage := append([]byte("\n<JSON>"), notificationBytes...)
			wrappedMessage = append(wrappedMessage, []byte("</JSON>\n")...)
			session.OutputBuffer.Write(wrappedMessage)
		}
		session.LastActive = time.Now()
		session.Lock.Unlock()
	} else {
		log.Printf("No active connections for session %s, skipping termination broadcast", sessionID)
	}
}
