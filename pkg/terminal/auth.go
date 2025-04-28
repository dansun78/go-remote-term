package terminal

import (
	"encoding/json"
	"log"

	"github.com/gorilla/websocket"
)

// DefaultAuthProvider implements the AuthProvider interface with a simple token check
type DefaultAuthProvider struct {
	Token string
}

// ValidataAuthToken implements AuthProvider.ValidataAuthToken
func (p *DefaultAuthProvider) ValidataAuthToken(token string) bool {
	// If no token is configured, allow all (not recommended for production)
	if p.Token == "" {
		return true
	}
	return token == p.Token
}

// SetAuthToken sets the authentication token for the default auth provider
func SetAuthToken(options *TerminalOptions, token string) {
	// If the current provider is the default one, update its token
	if defaultProvider, ok := options.AuthProvider.(*DefaultAuthProvider); ok {
		defaultProvider.Token = token
	} else {
		// If a custom provider is used, replace it with a default one with the given token
		options.AuthProvider = &DefaultAuthProvider{Token: token}
	}
}

// validateClientAuth validates a client's authentication message
// Returns whether authentication was successful and any error message
func validateClientAuth(conn *websocket.Conn, options *TerminalOptions) (bool, string, *Message) {
	// Wait for authentication message
	_, rawMessage, err := conn.ReadMessage()
	if err != nil {
		log.Println("Failed to read authentication message:", err)
		return false, "Failed to read authentication message", nil
	}

	// Parse the authentication message
	var msg Message
	if err := json.Unmarshal(rawMessage, &msg); err != nil {
		log.Println("Failed to parse authentication message:", err)
		return false, "Invalid authentication format", nil
	}

	// Handle authentication
	if msg.Type != "auth" {
		log.Println("Expected auth message type but got:", msg.Type)
		return false, "Invalid message type", nil
	}

	// Check token validity - client-provided token must not be empty
	if msg.Token == "" {
		log.Println("Authentication failed: Missing token")
		return false, "Missing authentication token", nil
	}

	// Validate token using the AuthProvider interface
	if options.AuthProvider != nil && !options.AuthProvider.ValidataAuthToken(msg.Token) {
		log.Println("Authentication failed: Invalid token")
		return false, "Invalid authentication token", nil
	}

	// Authentication successful
	return true, "", &msg
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

// sendAuthSuccess sends a successful authentication response
func sendAuthSuccess(conn *websocket.Conn, sessionID string) error {
	authResp := Response{
		Type:      "auth_response",
		Success:   true,
		SessionID: sessionID,
	}
	respBytes, _ := json.Marshal(authResp)
	return conn.WriteMessage(websocket.TextMessage, respBytes)
}
