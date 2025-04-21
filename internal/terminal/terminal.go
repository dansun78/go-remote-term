package terminal

import (
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all connections for now
	},
}

// HandleWebSocket handles WebSocket connections for terminal sessions
func HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Failed to upgrade connection:", err)
		return
	}
	defer conn.Close()

	// Create a new shell command with a simplified environment
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	// Use a simplified command to start bash without startup files
	// and with a simple prompt to avoid control sequences
	cmd := exec.Command(shell)

	// Use a simplified environment to reduce control sequences
	cleanEnv := []string{
		"TERM=dumb",                          // Use dumb terminal to disable most control sequences
		"PS1=\\w $ ",                         // Simple prompt without color codes
		"PROMPT_COMMAND=",                    // Disable prompt command
		"BASH_SILENCE_DEPRECATION_WARNING=1", // Silence bash deprecation warnings
		"LANG=C",                             // Use C locale for simpler output
		"PATH=" + os.Getenv("PATH"),          // Preserve PATH
		"HOME=" + os.Getenv("HOME"),          // Preserve HOME
		"USER=" + os.Getenv("USER"),          // Preserve USER
	}
	cmd.Env = cleanEnv

	// Start the command with a pty
	ptmx, err := pty.Start(cmd)
	if err != nil {
		log.Println("Failed to start PTY:", err)
		return
	}
	defer ptmx.Close()

	// Set terminal size to standard dimensions
	pty.Setsize(ptmx, &pty.Winsize{
		Rows: 24,
		Cols: 80,
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

// processTerminalOutput filters problematic control sequences from terminal output
func processTerminalOutput(data []byte) []byte {
	// Convert to string for easier manipulation
	str := string(data)

	// Filter out problematic control sequences

	// Bracket paste mode sequences
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
