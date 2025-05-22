package terminal

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// AllowedOrigins stores the list of origins that are allowed to connect
var AllowedOrigins = []string{
	"http://localhost:8080",
	"https://localhost:8080", // Match the default address regardless of protocol
}

// Default WebSocket upgrader with improved CORS settings
// Applications can use their own upgrader by setting terminal.Upgrader
var Upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")

		// If origin is empty, this might be a non-browser client or same-origin request
		if origin == "" {
			return true
		}

		// Check if the origin is in our allowed list
		for _, allowedOrigin := range AllowedOrigins {
			if origin == allowedOrigin {
				return true
			}
		}

		log.Printf("Rejected WebSocket connection from origin: %s", origin)
		return false
	},
}

// SetAllowedOrigins updates the list of origins allowed to connect
func SetAllowedOrigins(origins []string) {
	AllowedOrigins = origins
}

// HandleWebSocketWithOptions handles WebSocket connections for terminal sessions with custom options
func HandleWebSocketWithOptions(w http.ResponseWriter, r *http.Request, options *TerminalOptions) {
	conn, err := Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Failed to upgrade connection:", err)
		return
	}
	defer conn.Close()

	// Validate authentication
	authenticated, errMsg, authMsg := validateClientAuth(conn, options)
	if !authenticated {
		sendErrorResponse(conn, errMsg)
		return
	}

	// At this point user is authenticated
	msg := authMsg // From validateClientAuth

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
	if err := sendAuthSuccess(conn, session.ID); err != nil {
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
	handleTerminalConnection(conn, session, isNewSession)
}

// handleTerminalConnection manages a WebSocket connection for an existing terminal session
func handleTerminalConnection(conn *websocket.Conn, session *TerminalSession, isNewSession bool) {
	// Wait group for connection handling goroutines
	var wg sync.WaitGroup
	wg.Add(2)

	// Channel to signal when this connection is closed
	connClosed := make(chan struct{})

	// Forward terminal output to the WebSocket
	go func() {
		defer wg.Done()

		// Initialize lastSize based on session type
		// For new sessions, we want to show all output from the beginning (lastSize = 0)
		// For reconnections, we've already sent the buffer, so we start from current size
		session.Lock.Lock()
		var lastSize int
		if isNewSession {
			lastSize = 0
		} else {
			lastSize = session.OutputBuffer.Len()
		}
		session.Lock.Unlock()

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
