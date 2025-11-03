package websocket

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"zlay-backend/internal/chat"
	"zlay-backend/internal/db"
	"zlay-backend/internal/messages"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// TODO: Add proper origin checking in production
		return true
	},
	// Enable WebSocket compression
	EnableCompression: true,
}

// Handler manages WebSocket connections
type Handler struct {
	hub              *Hub
	chatService       chat.ChatService
	db               *db.Database
	clientConfigCache *ClientConfigCache
}

// NewHandler creates a new WebSocket handler
func NewHandler(hub *Hub, db *db.Database, clientConfigCache *ClientConfigCache) *Handler {
	return &Handler{
		hub:              hub,
		db:               db,
		clientConfigCache: clientConfigCache,
	}
}

// SetChatService sets the chat service (to avoid circular dependencies)
func (h *Handler) SetChatService(chatService chat.ChatService) {
	h.chatService = chatService
}

// HandleWebSocket handles WebSocket upgrade and connection management
func (h *Handler) HandleWebSocket(c *gin.Context) {
	log.Printf("WebSocket connection attempt from: %s", c.Request.RemoteAddr)
	log.Printf("Request headers: %+v", c.Request.Header)
	
	// Get authentication token from cookie first (preferred)
	token := ""
	
	// Try multiple cookie names
	authCookie, err := c.Cookie("auth_token")
	if err == nil && authCookie != "" {
		token = authCookie
		log.Printf("Found auth_token in cookie")
	}
	
	// Try session_token if auth_token not found
	if token == "" {
		sessionCookie, err := c.Cookie("session_token")
		if err == nil && sessionCookie != "" {
			// URL decode the session token
			decodedToken, decodeErr := url.QueryUnescape(sessionCookie)
			if decodeErr != nil {
				log.Printf("Failed to decode session token: %v", decodeErr)
			} else {
				token = decodedToken
				log.Printf("Found and decoded session_token in cookie")
			}
		}
	}
	
	// Fallback to query parameter
	if token == "" {
		token = c.Query("token")
		log.Printf("Trying auth token from query parameter")
	}
	
	// Fallback to Authorization header
	if token == "" {
		token = c.GetHeader("Authorization")
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}
		log.Printf("Trying auth token from Authorization header")
	}
	
	var tokenStatus string
	if token != "" {
		tokenStatus = "PRESENT"
	} else {
		tokenStatus = "MISSING"
	}
	log.Printf("Final token status: %s", tokenStatus)
	log.Printf("All request cookies: %s", c.Request.Header.Get("Cookie"))

	// Get project ID from query
	projectID := c.Query("project")
	if projectID == "" {
		log.Printf("Missing project ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "project_id is required"})
		return
	}
	
	log.Printf("Project ID: %s", projectID)

	// Authenticate user and get session data
	userID, clientID, err := h.authenticateToken(token)
	if err != nil {
		log.Printf("Authentication failed: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authentication token"})
		return
	}
	
	log.Printf("Authentication successful: userID=%s, clientID=%s", userID, clientID)

	// Upgrade HTTP connection to WebSocket
	log.Printf("Attempting WebSocket upgrade for %s", c.Request.URL.String())
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	log.Printf("WebSocket upgrade successful for %s", c.Request.RemoteAddr)

	// Create new connection
	conn := NewConnection(ws, userID, clientID, h.hub)
	// Attach the handler so the connection can route chat‑related messages
	conn.handler = h

	// Register connection with hub
	h.hub.register <- conn

	// Start connection pumps
	go conn.WritePump()
	go conn.ReadPump()

	// Auto-join the specified project
	conn.JoinProject(projectID)

	log.Printf("WebSocket connection established for user %s, client %s, project %s", userID, clientID, projectID)
	
	// DEBUG: Test LLM config loading on connection
	log.Printf("DEBUG: Testing LLM config loading on connection for client %s", clientID)
	testConfig, err := h.clientConfigCache.GetClientConfig(context.Background(), clientID)
	if err != nil {
		log.Printf("DEBUG: Failed to load LLM config on connection test: %v", err)
	} else {
		// Simple key masking for debug
		maskedKey := "EMPTY"
		if testConfig.APIKey != "" {
			if len(testConfig.APIKey) <= 8 {
				maskedKey = strings.Repeat("*", len(testConfig.APIKey))
			} else if len(testConfig.APIKey) <= 16 {
				maskedKey = testConfig.APIKey[:4] + strings.Repeat("*", len(testConfig.APIKey)-4)
			} else {
				maskedKey = testConfig.APIKey[:8] + strings.Repeat("*", len(testConfig.APIKey)-8)
			}
		}
		log.Printf("DEBUG: Successfully loaded LLM config on connection test - Model: %s, BaseURL: %s, API Key: %s", testConfig.Model, testConfig.BaseURL, maskedKey)
		
		// DEBUG: Log LLM connection details
		log.Printf("DEBUG: LLM client successfully connected and ready for client %s", clientID)
	}
}

// authenticateToken validates the authentication token and returns user and client IDs
func (h *Handler) authenticateToken(token string) (string, string, error) {
	if token == "" {
		return "", "", fmt.Errorf("token is required")
	}

	// Hash token to match database storage format
	tokenHash := sha256.Sum256([]byte(token))
	tokenHashStr := base64.StdEncoding.EncodeToString(tokenHash[:])

	// Query session and user data using ZDB
	query := `
		SELECT u.id, u.client_id, s.expires_at
		FROM sessions s
		JOIN users u ON s.user_id = u.id
		WHERE s.token_hash = $1 AND u.is_active = true
	`
	log.Printf("Authentication query: %s", query)
	log.Printf("Token hash: %s", tokenHashStr)
	
	row, err := h.db.QueryRow(context.Background(), query, tokenHashStr)
	if err != nil {
		log.Printf("Database query error: %v", err)
		return "", "", fmt.Errorf("database error: %w", err)
	}

	if len(row.Values) != 3 {
		return "", "", fmt.Errorf("invalid session data")
	}

	userID, ok := row.Values[0].AsString()
	if !ok {
		return "", "", fmt.Errorf("invalid user ID")
	}
	clientID, ok := row.Values[1].AsString()
	if !ok {
		return "", "", fmt.Errorf("invalid client ID")
	}
	expiresAtStr, ok := row.Values[2].AsString()
	if !ok {
		return "", "", fmt.Errorf("invalid expires at")
	}
	
	// Check if expires_at is already a time.Time from database
	log.Printf("expires_at type: %T", row.Values[2])
	log.Printf("expires_at value: %v", row.Values[2])
	
	// Try direct time conversion first
	if expiresAtTime, ok := row.Values[2].AsTimestamp(); ok {
		// Check if session has expired using the parsed time
		log.Printf("Parsed expires_at time: %v", expiresAtTime.Time)
		if time.Now().After(expiresAtTime.Time) {
			return "", "", fmt.Errorf("token expired")
		}
		return userID, clientID, nil
	}
	// New handling: if the value is a timestamp Value, extract the time directly
	if row.Values[2].Type == db.ValueTypeTimestamp {
		if ts, ok := row.Values[2].Data.(db.Timestamp); ok {
			t := ts.Time
			log.Printf("Extracted time.Time from db.Value Timestamp: %v", t)
			if time.Now().After(t) {
				return "", "", fmt.Errorf("token expired")
			}
			return userID, clientID, nil
		}
	}
	
	// Fallback to string parsing
	expiresAt, err := time.Parse(time.RFC3339, expiresAtStr)
	if err != nil {
		// Try parsing as PostgreSQL timestamp format
		expiresAt, err = time.Parse("2006-01-02 15:04:05 -0700", expiresAtStr)
		if err != nil {
			// Try parsing without timezone
			expiresAt, err = time.Parse("2006-01-02 15:04:05", expiresAtStr)
			if err != nil {
				log.Printf("Failed to parse expires_at '%s': %v", expiresAtStr, err)
				return "", "", fmt.Errorf("invalid expires at format")
			}
		}
	}

	// Check if session has expired
	if time.Now().After(expiresAt) {
		return "", "", fmt.Errorf("token expired")
	}

	return userID, clientID, nil
}

// HandleMessage processes incoming WebSocket messages
func (h *Handler) HandleMessage(conn *Connection, message *WebSocketMessage) {
	log.Printf("Received WebSocket message: type='%s', data=%+v", message.Type, message.Data)
	
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

	log.Printf("DEBUG: User chat: \"%s\"", content)

	// Add connection metadata per AsyncAPI spec
	data["connection_id"] = conn.ID
	data["user_id"] = conn.UserID
	data["project_id"] = conn.ProjectID
	data["client_id"] = conn.ClientID

	// Get client-specific LLM configuration
	clientConfig, err := h.clientConfigCache.GetClientConfig(context.Background(), conn.ClientID)
	if err != nil {
		log.Printf("Failed to get client LLM config: %v", err)
		h.sendErrorResponse(conn, conversationID, "Failed to load LLM configuration", err.Error())
		return
	}

	// Create chat request
	chatReq := &chat.ChatRequest{
		ConversationID: conversationID,
		UserID:         conn.UserID,
		ProjectID:      conn.ProjectID,
		Content:        content,
		ConnectionID:   conn.ID,
		AddTokensFunc:  conn.AddTokens, // Token tracking function
		Connection:     conn,           // Connection reference for token info
	}

	// Process through ChatService with client-specific LLM
	if h.chatService != nil {
		// Temporarily update chat service's LLM client (for now)
		// TODO: Refactor to have client-specific chat services
		chatServiceWithClientLLM := h.chatService.WithLLMClient(clientConfig.LLMClient)
		
		err := chatServiceWithClientLLM.ProcessUserMessage(chatReq)
		if err != nil {
			log.Printf("Error processing user message: %v", err)
			h.sendErrorResponse(conn, conversationID, "Failed to process message", err.Error())
		}
	} else {
		// Fallback for when chat service is not initialized
		response := messages.WebSocketMessage{
			Type: "assistant_response",
			Data: AssistantResponseData{
				ConversationID: conversationID,
				Content:        fmt.Sprintf("I received your message: %s. Chat service not available.", content),
				MessageID:      "msg-" + uuid.New().String(),
				Timestamp:      time.Now().Format(time.RFC3339),
				Done:           true,
			},
			Timestamp: time.Now().UnixMilli(),
		}
		h.hub.SendToConnection(conn, response)
	}
}

// sendErrorResponse sends a formatted error response
func (h *Handler) sendErrorResponse(conn *Connection, conversationID, message, details string) {
	errorResponse := WebSocketMessage{
		Type: "error",
		Data: ErrorData{
			Error:   message,
			Details: map[string]interface{}{"conversation_id": conversationID, "error": details},
		},
		Timestamp: time.Now().UnixMilli(),
	}
	h.hub.SendToConnection(conn, errorResponse)
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

	// Check if an initial message is included
	initialMessage, hasInitialMessage := data["initial_message"].(string)

	if h.chatService != nil {
		// Use actual chat service
		conversation, err := h.chatService.CreateConversation(conn.UserID, conn.ProjectID, title)
		if err != nil {
			log.Printf("Error creating conversation: %v", err)
			errorResponse := WebSocketMessage{
				Type: "error",
				Data: ErrorData{
					Error:   "Failed to create conversation",
					Details: map[string]interface{}{"error": err.Error()},
				},
				Timestamp: time.Now().UnixMilli(),
			}
			h.hub.SendToConnection(conn, errorResponse)
			return
		}

		// Send success response matching AsyncAPI spec
		h.hub.SendToConnection(conn, WebSocketMessage{
			Type: "conversation_created",
			Data: ConversationCreatedData{
				Conversation: convertConversation(conversation),
				Success:      true,
			},
			Timestamp: time.Now().UnixMilli(),
		})

		// If there's an initial message, process it
		if hasInitialMessage && initialMessage != "" {
			// Get client-specific LLM configuration
			clientConfig, err := h.clientConfigCache.GetClientConfig(context.Background(), conn.ClientID)
			if err != nil {
				log.Printf("Failed to get client LLM config: %v", err)
				h.sendErrorResponse(conn, conversation.ID, "Failed to load LLM configuration", err.Error())
				return
			}

			// Create chat request for the initial message
			chatReq := &chat.ChatRequest{
				ConversationID: conversation.ID,
				UserID:         conn.UserID,
				ProjectID:      conn.ProjectID,
				Content:        initialMessage,
				ConnectionID:   conn.ID,
				AddTokensFunc:  conn.AddTokens, // Token tracking function
				Connection:     conn,           // Connection reference for token info
			}

			// Process through ChatService with client-specific LLM
			chatServiceWithClientLLM := h.chatService.WithLLMClient(clientConfig.LLMClient)
			
			err = chatServiceWithClientLLM.ProcessUserMessage(chatReq)
			if err != nil {
				log.Printf("Error processing initial message: %v", err)
				h.sendErrorResponse(conn, conversation.ID, "Failed to process initial message", err.Error())
			}
		}
	} else {
		// Fallback for when chat service is not initialized
		conversation := Conversation{
			ID:        "conv-" + uuid.New().String(),
			Title:     title,
			UserID:    conn.UserID,
			ProjectID: conn.ProjectID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		// Send success response in AsyncAPI format
		h.hub.SendToConnection(conn, WebSocketMessage{
			Type: "conversation_created",
			Data: ConversationCreatedData{
				Conversation: conversation,
				Success:      true,
			},
			Timestamp: time.Now().UnixMilli(),
		})

		// If there's an initial message, send a simple response
		if hasInitialMessage && initialMessage != "" {
			response := messages.WebSocketMessage{
				Type: "assistant_response",
				Data: AssistantResponseData{
					ConversationID: conversation.ID,
					Content:        fmt.Sprintf("I received your initial message: %s. Chat service not available.", initialMessage),
					MessageID:      "msg-" + uuid.New().String(),
					Timestamp:      time.Now().Format(time.RFC3339),
					Done:           true,
				},
				Timestamp: time.Now().UnixMilli(),
			}
			h.hub.SendToConnection(conn, response)
		}
	}
}

// formatAsyncAPIMessage creates a properly formatted AsyncAPI message (deprecated - use typed structs)
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
			errorResponse := WebSocketMessage{
				Type: "error",
				Data: ErrorData{
					Error:   "Failed to get conversations",
					Details: map[string]interface{}{"error": err.Error()},
				},
				Timestamp: time.Now().UnixMilli(),
			}
			h.hub.SendToConnection(conn, errorResponse)
			return
		}

		// Send conversations list matching AsyncAPI spec
		h.hub.SendToConnection(conn, WebSocketMessage{
			Type: "conversations_list",
			Data: ConversationsListData{
				Conversations: convertConversations(conversations),
			},
			Timestamp: time.Now().UnixMilli(),
		})
	} else {
		// Fallback for when chat service is not initialized
		conversations := []Conversation{
			{
				ID:        "conv-1",
				Title:     "Sample Conversation 1",
				UserID:    conn.UserID,
				ProjectID: conn.ProjectID,
				CreatedAt: time.Now().Add(-24 * time.Hour),
				UpdatedAt: time.Now().Add(-24 * time.Hour),
			},
			{
				ID:        "conv-2",
				Title:     "Sample Conversation 2",
				UserID:    conn.UserID,
				ProjectID: conn.ProjectID,
				CreatedAt: time.Now().Add(-2 * 24 * time.Hour),
				UpdatedAt: time.Now().Add(-2 * 24 * time.Hour),
			},
		}

		// Send conversations list in AsyncAPI format
		h.hub.SendToConnection(conn, WebSocketMessage{
			Type: "conversations_list",
			Data: ConversationsListData{
				Conversations: conversations,
			},
			Timestamp: time.Now().UnixMilli(),
		})
	}
}

// handleToolExecutionStarted sends tool execution started notification
func (h *Handler) handleToolExecutionStarted(conn *Connection, toolName, toolCallID, conversationID, messageID string) {
	h.hub.BroadcastToProject(conn.ProjectID, WebSocketMessage{
		Type: "tool_execution_started",
		Data: ToolExecutionStartedData{
			ToolName:       toolName,
			ToolCallID:     toolCallID,
			ConversationID: conversationID,
			MessageID:      messageID,
		},
		Timestamp: time.Now().UnixMilli(),
	})
}

// handleToolExecutionCompleted sends tool execution completed notification
func (h *Handler) handleToolExecutionCompleted(conn *Connection, toolName, toolCallID, conversationID, messageID string, result interface{}, executionTimeMs int) {
	h.hub.BroadcastToProject(conn.ProjectID, WebSocketMessage{
		Type: "tool_execution_completed",
		Data: ToolExecutionCompletedData{
			ToolName:         toolName,
			ToolCallID:       toolCallID,
			ConversationID:   conversationID,
			Success:          true,
			Result:           result,
			ExecutionTimeMs:  executionTimeMs,
		},
		Timestamp: time.Now().UnixMilli(),
	})
}

// handleToolExecutionFailed sends tool execution failed notification
func (h *Handler) handleToolExecutionFailed(conn *Connection, toolName, toolCallID, conversationID, errorMsg, errorCode string) {
	h.hub.BroadcastToProject(conn.ProjectID, WebSocketMessage{
		Type: "tool_execution_failed",
		Data: ToolExecutionFailedData{
			ToolName:       toolName,
			ToolCallID:     toolCallID,
			ConversationID: conversationID,
			Error:          errorMsg,
			ErrorCode:      errorCode,
		},
		Timestamp: time.Now().UnixMilli(),
	})
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
			errorResponse := WebSocketMessage{
				Type: "error",
				Data: ErrorData{
					Error:   "Failed to get conversation",
					Details: map[string]interface{}{"error": err.Error()},
				},
				Timestamp: time.Now().UnixMilli(),
			}
			h.hub.SendToConnection(conn, errorResponse)
			return
		}

		// Send conversation with messages matching AsyncAPI spec
		h.hub.SendToConnection(conn, WebSocketMessage{
			Type: "conversation_details",
			Data: ConversationDetailsData{
				Conversation: convertConversationDetails(conversation),
			},
			Timestamp: time.Now().UnixMilli(),
		})
	} else {
		// Fallback for when chat service is not initialized
		conversation := ConversationWithMessages{
			Conversation: Conversation{
				ID:        conversationID,
				Title:     "Sample Conversation",
				UserID:    conn.UserID,
				ProjectID: conn.ProjectID,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			Messages: []Message{
				{
					ID:        "msg-1",
					Role:      "user",
					Content:   "Hello, how are you?",
					CreatedAt: time.Now().Add(-5 * time.Minute),
				},
				{
					ID:        "msg-2",
					Role:      "assistant",
					Content:   "I'm doing well, thank you for asking!",
					CreatedAt: time.Now().Add(-4 * time.Minute),
				},
			},
		}

		// Send conversation with messages
		h.hub.SendToConnection(conn, WebSocketMessage{
			Type: "conversation_details",
			Data: ConversationDetailsData{
				Conversation: conversation,
			},
			Timestamp: time.Now().UnixMilli(),
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
			errorResponse := WebSocketMessage{
				Type: "error",
				Data: ErrorData{
					Error:   "Failed to delete conversation",
					Details: map[string]interface{}{"error": err.Error()},
				},
				Timestamp: time.Now().UnixMilli(),
			}
			h.hub.SendToConnection(conn, errorResponse)
			return
		}

		// Send success response matching AsyncAPI spec
		h.hub.SendToConnection(conn, WebSocketMessage{
			Type: "conversation_deleted",
			Data: gin.H{
				"conversation_id": conversationID,
				"success":         true,
			},
			Timestamp: time.Now().UnixMilli(),
		})
	} else {
		// Fallback for when chat service is not initialized
		// Send success response in AsyncAPI format
		h.hub.SendToConnection(conn, WebSocketMessage{
			Type: "conversation_deleted",
			Data: gin.H{
				"conversation_id": conversationID,
				"success":         true,
			},
			Timestamp: time.Now().UnixMilli(),
		})
	}
}

// Helper function to get current timestamp
func getCurrentTimestamp() int64 {
	return time.Now().UnixMilli()
}
