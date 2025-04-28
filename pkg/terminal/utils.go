package terminal

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
