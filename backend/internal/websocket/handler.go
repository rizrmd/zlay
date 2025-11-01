package websocket

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/google/uuid"
	"zlay-backend/internal/chat"
	"zlay-backend/internal/db"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// TODO: Add proper origin checking in production
		return true
	},
}

// Handler manages WebSocket connections
type Handler struct {
	hub         *Hub
	chatService chat.ChatService
	db          *db.Database
}

// NewHandler creates a new WebSocket handler
func NewHandler(hub *Hub, db *db.Database) *Handler {
	return &Handler{
		hub: hub,
		db:  db,
	}
}

// SetChatService sets the chat service (to avoid circular dependencies)
func (h *Handler) SetChatService(chatService chat.ChatService) {
	h.chatService = chatService
}

// HandleWebSocket handles WebSocket upgrade and connection management
func (h *Handler) HandleWebSocket(c *gin.Context) {
	// Get authentication token from query or header
	token := c.Query("token")
	if token == "" {
		token = c.GetHeader("Authorization")
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}
	}

	// Get project ID from query
	projectID := c.Query("project")
	if projectID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "project_id is required"})
		return
	}

	// Authenticate user and get session data
	userID, clientID, err := h.authenticateToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authentication token"})
		return
	}

	// Upgrade HTTP connection to WebSocket
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	// Create new connection
	conn := NewConnection(ws, userID, clientID, h.hub)

	// Register connection with hub
	h.hub.register <- conn

	// Start connection pumps
	go conn.WritePump()
	go conn.ReadPump()

	// Auto-join the specified project
	conn.JoinProject(projectID)

	log.Printf("WebSocket connection established for user %s, client %s, project %s", userID, clientID, projectID)
}

// authenticateToken validates the authentication token and returns user and client IDs
func (h *Handler) authenticateToken(token string) (string, string, error) {
	if token == "" {
		return "", "", fmt.Errorf("token is required")
	}

	// Hash token to match database storage format
	tokenHash := sha256.Sum256([]byte(token))
	tokenHashStr := base64.StdEncoding.EncodeToString(tokenHash[:])

	// Query session and user data
	var userID, clientID string
	var expiresAt time.Time
	err := h.db.QueryRow(context.Background(),
		`SELECT u.id, u.client_id, s.expires_at 
		FROM sessions s 
		JOIN users u ON s.user_id = u.id 
		WHERE s.token_hash = $1 AND u.is_active = true`,
		tokenHashStr).Scan(&userID, &clientID, &expiresAt)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", fmt.Errorf("invalid or expired token")
		}
		return "", "", fmt.Errorf("database error: %w", err)
	}

	// Check if session has expired
	if time.Now().After(expiresAt) {
		return "", "", fmt.Errorf("token expired")
	}

	// Validate returned values
	if userID == "" || clientID == "" {
		return "", "", fmt.Errorf("invalid session data")
	}

	return userID, clientID, nil
}

// HandleMessage processes incoming WebSocket messages
func (h *Handler) HandleMessage(conn *Connection, message *WebSocketMessage) {
	switch message.Type {
	case "user_message":
		h.handleUserMessage(conn, message)
	case "create_conversation":
		h.handleCreateConversation(conn, message)
	case "get_conversations":
		h.handleGetConversations(conn, message)
	case "get_conversation":
		h.handleGetConversation(conn, message)
	case "delete_conversation":
		h.handleDeleteConversation(conn, message)
	default:
		log.Printf("Unknown message type: %s", message.Type)
	}
}

// handleUserMessage processes user messages and routes to LLM
func (h *Handler) handleUserMessage(conn *Connection, message *WebSocketMessage) {
	data, ok := message.Data.(map[string]interface{})
	if !ok {
		log.Printf("Invalid user_message data format")
		return
	}

	conversationID, ok := data["conversation_id"].(string)
	if !ok {
		log.Printf("Missing conversation_id in user_message")
		return
	}

	content, ok := data["content"].(string)
	if !ok {
		log.Printf("Missing content in user_message")
		return
	}

	// Create chat request
	chatReq := &chat.ChatRequest{
		ConversationID: conversationID,
		UserID:         conn.UserID,
		ProjectID:      conn.ProjectID,
		Content:        content,
		ConnectionID:   conn.ID,
	}

	// Process through ChatService
	if h.chatService != nil {
		err := h.chatService.ProcessUserMessage(chatReq)
		if err != nil {
			log.Printf("Error processing user message: %v", err)
			
			// Send error response
			errorResponse := formatAsyncAPIMessage("error", gin.H{
				"conversation_id": conversationID,
				"message":        "Failed to process message",
				"error":          err.Error(),
				"timestamp":      time.Now().Format(time.RFC3339),
			})
			h.hub.SendToConnection(conn, errorResponse)
		}
	} else {
		// Fallback for when chat service is not initialized
		response := formatAsyncAPIMessage("assistant_response", gin.H{
			"conversation_id": conversationID,
			"content":         fmt.Sprintf("I received your message: %s. Chat service not available.", content),
			"message_id":      "msg-" + uuid.New().String(),
			"timestamp":       time.Now().Format(time.RFC3339),
			"done":           true,
		})
		h.hub.SendToConnection(conn, response)
	}
}

// handleCreateConversation creates a new conversation
func (h *Handler) handleCreateConversation(conn *Connection, message *WebSocketMessage) {
	data, ok := message.Data.(map[string]interface{})
	if !ok {
		log.Printf("Invalid create_conversation data format")
		return
	}

	title, ok := data["title"].(string)
	if !ok {
		title = "New Conversation" // Default title
	}

	if h.chatService != nil {
		// Use actual chat service
		conversation, err := h.chatService.CreateConversation(conn.UserID, conn.ProjectID, title)
		if err != nil {
			log.Printf("Error creating conversation: %v", err)
			errorResponse := formatAsyncAPIMessage("error", gin.H{
				"message": "Failed to create conversation",
				"error":   err.Error(),
			})
			h.hub.SendToConnection(conn, errorResponse)
			return
		}

		// Send success response
		h.hub.SendToConnection(conn, formatAsyncAPIMessage("conversation_created", gin.H{
			"conversation": conversation,
			"success":     true,
		}))
	} else {
		// Fallback for when chat service is not initialized
		conversation := gin.H{
			"id":         "conv-" + uuid.New().String(),
			"title":      title,
			"user_id":    conn.UserID,
			"project_id": conn.ProjectID,
			"created_at": time.Now().Format(time.RFC3339),
		}

		// Send success response in AsyncAPI format
		h.hub.SendToConnection(conn, formatAsyncAPIMessage("conversation_created", gin.H{
			"conversation": conversation,
			"success":     true,
		}))
	}
}

// formatAsyncAPIMessage creates a properly formatted AsyncAPI message
func formatAsyncAPIMessage(messageType string, data interface{}) WebSocketMessage {
	return WebSocketMessage{
		Type:      messageType,
		Data:      data,
		Timestamp: time.Now().UnixMilli(),
	}
}

// handleGetConversations retrieves all conversations for a project
func (h *Handler) handleGetConversations(conn *Connection, message *WebSocketMessage) {
	if h.chatService != nil {
		// Use actual chat service
		conversations, err := h.chatService.GetConversations(conn.UserID, conn.ProjectID)
		if err != nil {
			log.Printf("Error getting conversations: %v", err)
			errorResponse := formatAsyncAPIMessage("error", gin.H{
				"message": "Failed to get conversations",
				"error":   err.Error(),
			})
			h.hub.SendToConnection(conn, errorResponse)
			return
		}

		// Send conversations list
		h.hub.SendToConnection(conn, formatAsyncAPIMessage("conversations_list", gin.H{
			"conversations": conversations,
		}))
	} else {
		// Fallback for when chat service is not initialized
		conversations := []gin.H{
			{
				"id":         "conv-1",
				"title":      "Sample Conversation 1",
				"user_id":    conn.UserID,
				"project_id": conn.ProjectID,
				"created_at": time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
			},
			{
				"id":         "conv-2",
				"title":      "Sample Conversation 2",
				"user_id":    conn.UserID,
				"project_id": conn.ProjectID,
				"created_at": time.Now().Add(-2 * 24 * time.Hour).Format(time.RFC3339),
			},
		}

		// Send conversations list in AsyncAPI format
		h.hub.SendToConnection(conn, formatAsyncAPIMessage("conversations_list", gin.H{
			"conversations": conversations,
		}))
	}
}

// handleToolExecutionStarted sends tool execution started notification
func (h *Handler) handleToolExecutionStarted(conn *Connection, toolName, toolCallID, conversationID, messageID string) {
	h.hub.BroadcastToProject(conn.ProjectID, formatAsyncAPIMessage("tool_execution_started", gin.H{
		"tool_name":     toolName,
		"tool_call_id":  toolCallID,
		"conversation_id": conversationID,
		"message_id":     messageID,
	}))
}

// handleToolExecutionCompleted sends tool execution completed notification
func (h *Handler) handleToolExecutionCompleted(conn *Connection, toolName, toolCallID, conversationID, messageID string, result interface{}, executionTimeMs int) {
	h.hub.BroadcastToProject(conn.ProjectID, formatAsyncAPIMessage("tool_execution_completed", gin.H{
		"tool_name":       toolName,
		"tool_call_id":    toolCallID,
		"conversation_id":   conversationID,
		"result":          result,
		"execution_time_ms": executionTimeMs,
		"success":         true,
	}))
}

// handleToolExecutionFailed sends tool execution failed notification
func (h *Handler) handleToolExecutionFailed(conn *Connection, toolName, toolCallID, conversationID, errorMsg, errorCode string) {
	h.hub.BroadcastToProject(conn.ProjectID, formatAsyncAPIMessage("tool_execution_failed", gin.H{
		"tool_name":   toolName,
		"tool_call_id": toolCallID,
		"conversation_id": conversationID,
		"error":       errorMsg,
		"error_code":  errorCode,
	}))
}

// handleGetConversation retrieves a specific conversation with messages
func (h *Handler) handleGetConversation(conn *Connection, message *WebSocketMessage) {
	data, ok := message.Data.(map[string]interface{})
	if !ok {
		log.Printf("Invalid get_conversation data format")
		return
	}

	conversationID, ok := data["conversation_id"].(string)
	if !ok {
		log.Printf("Missing conversation_id in get_conversation")
		return
	}

	if h.chatService != nil {
		// Use actual chat service
		conversation, err := h.chatService.GetConversation(conversationID, conn.UserID)
		if err != nil {
			log.Printf("Error getting conversation: %v", err)
			errorResponse := formatAsyncAPIMessage("error", gin.H{
				"message": "Failed to get conversation",
				"error":   err.Error(),
			})
			h.hub.SendToConnection(conn, errorResponse)
			return
		}

		// Send conversation with messages
		h.hub.SendToConnection(conn, WebSocketMessage{
			Type: "conversation_details",
			Data: gin.H{
				"conversation": conversation,
			},
		})
	} else {
		// Fallback for when chat service is not initialized
		conversation := gin.H{
			"id":         conversationID,
			"title":      "Sample Conversation",
			"user_id":    conn.UserID,
			"project_id": conn.ProjectID,
			"created_at": time.Now().Format(time.RFC3339),
			"messages": []gin.H{
				{
					"id":         "msg-1",
					"role":       "user",
					"content":     "Hello, how are you?",
					"created_at": time.Now().Add(-5 * time.Minute).Format(time.RFC3339),
				},
				{
					"id":         "msg-2",
					"role":       "assistant",
					"content":     "I'm doing well, thank you for asking!",
					"created_at": time.Now().Add(-4 * time.Minute).Format(time.RFC3339),
				},
			},
		}

		// Send conversation with messages
		h.hub.SendToConnection(conn, WebSocketMessage{
			Type: "conversation_details",
			Data: gin.H{
				"conversation": conversation,
			},
		})
	}
}

// handleDeleteConversation deletes a conversation
func (h *Handler) handleDeleteConversation(conn *Connection, message *WebSocketMessage) {
	data, ok := message.Data.(map[string]interface{})
	if !ok {
		log.Printf("Invalid delete_conversation data format")
		return
	}

	conversationID, ok := data["conversation_id"].(string)
	if !ok {
		log.Printf("Missing conversation_id in delete_conversation")
		return
	}

	if h.chatService != nil {
		// Use actual chat service
		err := h.chatService.DeleteConversation(conversationID, conn.UserID)
		if err != nil {
			log.Printf("Error deleting conversation: %v", err)
			errorResponse := formatAsyncAPIMessage("error", gin.H{
				"message": "Failed to delete conversation",
				"error":   err.Error(),
			})
			h.hub.SendToConnection(conn, errorResponse)
			return
		}

		// Send success response
		h.hub.SendToConnection(conn, formatAsyncAPIMessage("conversation_deleted", gin.H{
			"conversation_id": conversationID,
			"success":        true,
		}))
	} else {
		// Fallback for when chat service is not initialized
		// Send success response in AsyncAPI format
		h.hub.SendToConnection(conn, formatAsyncAPIMessage("conversation_deleted", gin.H{
			"conversation_id": conversationID,
			"success":        true,
		}))
	}
}

// Helper function to get current timestamp
func getCurrentTimestamp() int64 {
	return time.Now().UnixMilli()
}
