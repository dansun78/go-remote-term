package terminal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/google/uuid"
)

// init starts a background routine to cleanup expired sessions
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

// ResizeTerminal resizes the terminal window
func ResizeTerminal(ptmx *os.File, rows, cols uint16) error {
	return pty.Setsize(ptmx, &pty.Winsize{
		Rows: rows,
		Cols: cols,
		X:    0,
		Y:    0,
	})
}
