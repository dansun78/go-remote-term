// Package terminal provides WebSocket-based terminal functionality that can be
// integrated into web applications to provide terminal access.
package terminal

import (
	"bytes"
	"os"
	"os/exec"
	"sync"
	"time"
)

// AuthProvider defines the interface for authentication providers
// This allows the terminal package to verify authentication without
// depending on specific implementation details
type AuthProvider interface {
	// ValidataAuthToken checks if the provided token is valid
	ValidataAuthToken(token string) bool
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

	// AuthProvider is used to validate authentication tokens
	AuthProvider AuthProvider
}

// Message represents the messages sent between client and server
type Message struct {
	Type      string `json:"type"`
	Token     string `json:"token,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Data      string `json:"data,omitempty"`
	Rows      uint16 `json:"rows,omitempty"`
	Cols      uint16 `json:"cols,omitempty"`
}

// Response represents server responses sent to clients
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
