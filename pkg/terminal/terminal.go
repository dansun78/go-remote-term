// Package terminal provides WebSocket-based terminal functionality that can be
// integrated into web applications to provide terminal access.
package terminal

import (
	"net/http"
	"os"
	"time"
)

// HandleWebSocket handles WebSocket connections for terminal sessions with default options
func HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	opts := DefaultOptions()
	// This assumes token is stored in context by middleware elsewhere
	// You would need to implement middleware to set this token from your security package
	if token, ok := r.Context().Value("auth_token").(string); ok {
		SetAuthToken(opts, token)
	}
	HandleWebSocketWithOptions(w, r, opts)
}

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
		AuthProvider: &DefaultAuthProvider{}, // Default provider with no token (should be overridden)
	}
}
