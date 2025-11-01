package messages

// WebSocketMessage represents a message sent over WebSocket
type WebSocketMessage struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp int64       `json:"timestamp"`
	ID        string      `json:"id,omitempty"`
	
	// Real-time token information (optional fields)
	TokensUsed     int64 `json:"tokens_used,omitempty"`
	TokensLimit    int64 `json:"tokens_limit,omitempty"`
	TokensRemaining int64 `json:"tokens_remaining,omitempty"`
}

// Hub interface for broadcasting messages
type Hub interface {
	BroadcastToProject(projectID string, message interface{})
}

// NewWebSocketMessage creates a new WebSocketMessage with token info
func NewWebSocketMessage(messageType string, data interface{}, tokensUsed, tokensLimit, tokensRemaining int64) *WebSocketMessage {
	return &WebSocketMessage{
		Type:           messageType,
		Data:           data,
		Timestamp:      0, // Set by caller
		TokensUsed:     tokensUsed,
		TokensLimit:    tokensLimit,
		TokensRemaining: tokensRemaining,
	}
}