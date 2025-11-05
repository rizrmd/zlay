package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"zlay-backend/internal/llm"
	msglib "zlay-backend/internal/messages"
	"zlay-backend/internal/tools"

	"github.com/gin-gonic/gin"
	"github.com/openai/openai-go"
)

// StreamState tracks active streaming conversations
type StreamState struct {
	ConversationID     string    `json:"conversation_id"`
	UserID           string    `json:"user_id"`
	ProjectID        string    `json:"project_id"`
	MessageID        string    `json:"message_id"`
	CurrentContent   string    `json:"current_content"`
	StartTime       time.Time `json:"start_time"`
	LastChunk       time.Time `json:"last_chunk"`
	IsActive        bool      `json:"is_active"`
	
	// 🔄 NEW: Track active connections for this stream
	ActiveConnectionIDs map[string]bool `json:"active_connection_ids"`
	Mutex              sync.RWMutex   `json:"-"`
	
	// 🔄 NEW: Track all connections that ever joined this stream (for persistence)
	AllConnectionIDs    map[string]bool `json:"all_connection_ids"`
}

// ChatService interface defines chat operations
type ChatService interface {
	ProcessUserMessage(req *ChatRequest) error
	CreateConversation(userID, projectID, title string) (*Conversation, error)
	GetConversations(userID, projectID string) ([]*Conversation, error)
	GetConversation(conversationID, userID string) (*ConversationDetails, error)
	DeleteConversation(conversationID, userID string) error
	WithLLMClient(llmClient llm.LLMClient) ChatService
	
	// 🔄 NEW: Streaming state management
	GetStreamState(conversationID string) (*StreamState, error)
	GetAllActiveStreams() map[string]*StreamState
	ClearStreamState(conversationID string) error
	GetConversationStatus(conversationID, userID string) (gin.H, error)
	
	// 🔄 NEW: Connection management for streaming
	AttachConnectionToStream(conversationID, connectionID string) error
	DetachConnectionFromStream(conversationID, connectionID string) error
	SendStreamToActiveConnections(conversationID string, message interface{}) error
	
	// 🔄 NEW: Load streaming conversation (including partial messages)
	LoadStreamingConversation(conversationID, userID string) (*ConversationDetails, error)
	
	// 🔄 NEW: Get only the active streaming message from memory
	GetActiveStreamingMessage(conversationID, userID string) (*StreamState, error)
	
	// 🔄 NEW: Update conversation status
	UpdateConversationStatus(conversationID, userID, status string) error
}

// chatService implements ChatService interface
type chatService struct {
	db           tools.DBConnection
	hub          msglib.Hub
	llmClient    llm.LLMClient
	toolRegistry tools.ToolRegistry
	
	// 🔄 NEW: Streaming state tracking
	activeStreams map[string]*StreamState
	streamingMutex sync.RWMutex
}

	// 🔄 NEW: Initialize streaming state tracking when creating chat service
func NewChatService(db tools.DBConnection, hub msglib.Hub, llmClient llm.LLMClient, toolRegistry tools.ToolRegistry) *chatService {
	return &chatService{
		db:           db,
		hub:          hub,
		llmClient:    llmClient,
		toolRegistry: toolRegistry,
		
		// 🔄 NEW: Initialize streaming tracking
		activeStreams: make(map[string]*StreamState),
	}
}

// WithLLMClient returns a new chat service instance with the specified LLM client
func (s *chatService) WithLLMClient(llmClient llm.LLMClient) ChatService {
	// Create a copy of the service with the new LLM client
	newService := &chatService{
		db:           s.db,
		hub:          s.hub,
		llmClient:    llmClient,
		toolRegistry: s.toolRegistry,
		
		// 🔄 NEW: Copy streaming state
		activeStreams: make(map[string]*StreamState),
	}
	
	// Copy existing streaming state
	s.streamingMutex.RLock()
	for convID, streamState := range s.activeStreams {
		// Create deep copy with connection tracking
		streamCopy := &StreamState{
			ConversationID:     streamState.ConversationID,
			UserID:            streamState.UserID,
			ProjectID:         streamState.ProjectID,
			MessageID:         streamState.MessageID,
			CurrentContent:     streamState.CurrentContent,
			StartTime:         streamState.StartTime,
			LastChunk:         streamState.LastChunk,
			IsActive:          streamState.IsActive,
			ActiveConnectionIDs: make(map[string]bool),
			AllConnectionIDs:    make(map[string]bool),  // 🔄 NEW: Copy all connections
			Mutex:             sync.RWMutex{},
		}
		
		// Copy existing connection IDs
		streamState.Mutex.RLock()
		for connID := range streamState.AllConnectionIDs {  // 🔄 NEW: Copy all connections
			streamCopy.AllConnectionIDs[connID] = true
		}
		streamState.Mutex.RUnlock()
		
		newService.activeStreams[convID] = streamCopy
	}
	s.streamingMutex.RUnlock()
	
	// Cast to interface type to satisfy return signature
	return ChatService(newService)
}

// ProcessUserMessage handles an incoming user message
func (s *chatService) ProcessUserMessage(req *ChatRequest) error {
	log.Printf("🚀 ProcessUserMessage CALLED:")
	log.Printf("   • Conversation ID: %s", req.ConversationID)
	log.Printf("   • User ID: %s", req.UserID)
	log.Printf("   • Project ID: %s", req.ProjectID)
	log.Printf("   • Content: \"%s\"", req.Content)
	log.Printf("   • Connection ID: %s", req.ConnectionID)
	log.Printf("   • Content Length: %d chars", len(req.Content))

	ctx := context.Background()

	// Create and save user message
	log.Printf("💾 CREATING AND SAVING USER MESSAGE...")
	userMsg := NewMessage(req.ConversationID, "user", req.Content, req.UserID, req.ProjectID)
	log.Printf("   • Message ID: %s", userMsg.ID)
	log.Printf("   • Role: %s", userMsg.Role)
	log.Printf("   • Created At: %s", userMsg.CreatedAt.Format(time.RFC3339))

	if err := s.saveMessage(ctx, userMsg); err != nil {
		log.Printf("❌ FAILED TO SAVE USER MESSAGE: %v", err)
		return fmt.Errorf("failed to save user message: %w", err)
	}
	log.Printf("✅ USER MESSAGE SAVED SUCCESSFULLY")

	// Broadcast user message to project room
	log.Printf("📡 BROADCASTING USER MESSAGE TO PROJECT %s", req.ProjectID)
	broadcastMsg := tools.WebSocketMessage{
		Type: "user_message_sent",
		Data: gin.H{
			"message":       userMsg,
			"connection_id": req.ConnectionID,
		},
		Timestamp: time.Now().UnixMilli(),
	}
	s.hub.BroadcastToProject(req.ProjectID, broadcastMsg)
	log.Printf("✅ USER MESSAGE BROADCASTED")

	// Get conversation history for context
	log.Printf("📚 FETCHING CONVERSATION HISTORY FOR CONTEXT...")
	history, err := s.getConversationHistory(ctx, req.ConversationID, req.UserID)
	if err != nil {
		log.Printf("❌ FAILED TO GET CONVERSATION HISTORY: %v", err)
		return fmt.Errorf("failed to get conversation history: %w", err)
	}
	log.Printf("✅ CONVERSATION HISTORY LOADED: %d messages", len(history))

	// Get available tools for this project
	log.Printf("🔧 FETCHING AVAILABLE TOOLS FOR PROJECT %s", req.ProjectID)
	availableTools := s.toolRegistry.GetAvailableTools(req.ProjectID)
	log.Printf("✅ TOOLS LOADED: %d tools available", len(availableTools))
	for i, tool := range availableTools {
		log.Printf("   • Tool %d: %s - %s", i+1, tool.Name, tool.Description)
	}

	// Convert messages to OpenAI format
	log.Printf("🔄 CONVERTING %d MESSAGES TO OPENAI FORMAT", len(history))
	openaiMessages := s.convertToOpenAIMessages(history)
	log.Printf("✅ MESSAGES CONVERTED TO OPENAI FORMAT")

	// Start streaming response
	log.Printf("🌊 STARTING LLM STREAMING RESPONSE...")
	return s.streamLLMResponse(ctx, req, openaiMessages, s.convertTools(availableTools))
}

// CreateConversation creates a new conversation
func (s *chatService) CreateConversation(userID, projectID, title string) (*Conversation, error) {
	ctx := context.Background()
	conversation := NewConversation(projectID, userID, title, "completed")

	// Save to database
	query := `
		INSERT INTO conversations (id, project_id, user_id, title, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, project_id, user_id, title, status, created_at, updated_at
	`

	var conv Conversation
	err := s.db.QueryRow(ctx, query,
		conversation.ID, conversation.ProjectID, conversation.UserID,
		conversation.Title, conversation.Status, conversation.CreatedAt, conversation.UpdatedAt,
	).Scan(
		&conv.ID, &conv.ProjectID, &conv.UserID,
		&conv.Title, &conv.Status, &conv.CreatedAt, &conv.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	return &conv, nil
}

// GetConversations retrieves all conversations for a user and project
func (s *chatService) GetConversations(userID, projectID string) ([]*Conversation, error) {
	ctx := context.Background()

	query := `
		SELECT id, project_id, user_id, title, status, created_at, updated_at
		FROM conversations
		WHERE user_id = $1 AND project_id = $2
		ORDER BY updated_at DESC
	`

	rows, err := s.db.Query(ctx, query, userID, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get conversations: %w", err)
	}
	defer rows.Close()

	var conversations []*Conversation
	for rows.Next() {
		var conv Conversation
		if err := rows.Scan(
			&conv.ID, &conv.ProjectID, &conv.UserID,
			&conv.Title, &conv.Status, &conv.CreatedAt, &conv.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan conversation: %w", err)
		}
		conversations = append(conversations, &conv)
	}

	return conversations, nil
}

// GetConversation retrieves a specific conversation with messages
func (s *chatService) GetConversation(conversationID, userID string) (*ConversationDetails, error) {
	ctx := context.Background()

	// Get conversation details
	convQuery := `
		SELECT id, project_id, user_id, title, status, created_at, updated_at
		FROM conversations
		WHERE id = $1 AND user_id = $2
	`

	var conversation Conversation
	err := s.db.QueryRow(ctx, convQuery, conversationID, userID).Scan(
		&conversation.ID, &conversation.ProjectID, &conversation.UserID,
		&conversation.Title, &conversation.Status, &conversation.CreatedAt, &conversation.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}

	// Get messages for conversation
	msgQuery := `
		SELECT id, conversation_id, role, content, metadata, tool_calls, created_at
		FROM messages
		WHERE conversation_id = $1
		ORDER BY created_at ASC
	`

	rows, err := s.db.Query(ctx, msgQuery, conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}
	defer rows.Close()

	var messages []*Message
	for rows.Next() {
		var msg Message
		var toolCallsJSON []byte
		var metadataJSON []byte

		if err := rows.Scan(
			&msg.ID, &msg.ConversationID, &msg.Role, &msg.Content,
			&metadataJSON, &toolCallsJSON, &msg.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}

		// Parse JSON fields
		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &msg.Metadata)
		}
		if len(toolCallsJSON) > 0 {
			json.Unmarshal(toolCallsJSON, &msg.ToolCalls)
		}

		messages = append(messages, &msg)
	}

	return &ConversationDetails{
		Conversation: &conversation,
		Messages:     messages,
		ToolStatus:   make(map[string]string),
	}, nil
}

// DeleteConversation deletes a conversation and its messages
func (s *chatService) DeleteConversation(conversationID, userID string) error {
	ctx := context.Background()

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete messages first (foreign key constraint)
	_, err = tx.Exec("DELETE FROM messages WHERE conversation_id = $1", conversationID)
	if err != nil {
		return fmt.Errorf("failed to delete messages: %w", err)
	}

	// Delete conversation
	_, err = tx.Exec("DELETE FROM conversations WHERE id = $1 AND user_id = $2", conversationID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete conversation: %w", err)
	}

	return tx.Commit()
}

// UpdateConversationStatus updates the status of a conversation
func (s *chatService) UpdateConversationStatus(conversationID, userID, status string) error {
	ctx := context.Background()
	
	query := `
		UPDATE conversations 
		SET status = $1, updated_at = $2
		WHERE id = $3 AND user_id = $4
	`
	
	_, err := s.db.Exec(ctx, query, status, time.Now(), conversationID, userID)
	if err != nil {
		return fmt.Errorf("failed to update conversation status: %w", err)
	}
	
	return nil
}

// streamLLMResponse streams LLM response to WebSocket
func (s *chatService) streamLLMResponse(ctx context.Context, req *ChatRequest, messages []openai.ChatCompletionMessageParamUnion, tools []Tool) error {
	log.Printf("🌊 streamLLMResponse CALLED:")
	log.Printf("   • Conversation ID: %s", req.ConversationID)
	log.Printf("   • User ID: %s", req.UserID)
	log.Printf("   • Project ID: %s", req.ProjectID)
	log.Printf("   • Messages Count: %d", len(messages))
	log.Printf("   • Tools Count: %d", len(tools))
	log.Printf("   • Connection ID: %s", req.ConnectionID)

	// Set conversation status to processing when streaming starts
	log.Printf("📊 SETTING CONVERSATION STATUS TO 'processing'...")
	if err := s.UpdateConversationStatus(req.ConversationID, req.UserID, "processing"); err != nil {
		log.Printf("❌ FAILED TO UPDATE CONVERSATION STATUS TO processing: %v", err)
	} else {
		log.Printf("✅ CONVERSATION STATUS UPDATED TO processing")
	}
	
	// Convert tools to OpenAI format
	var openaiTools []openai.ChatCompletionToolParam
	for _, tool := range tools {
		openaiTools = append(openaiTools, openai.ChatCompletionToolParam{
			Type: "function",
			Function: openai.FunctionDefinitionParam{
				Name:        tool.Name,
				Description: openai.String(tool.Description),
				Parameters:  tool.Parameters,
			},
		})
	}

	// Create LLM request
	llmReq := &llm.LLMRequest{
		Messages:    messages,
		Tools:       openaiTools,
		MaxTokens:   4000,
		Temperature: 0.7,
	}

	// Create assistant message placeholder
	assistantMsg := NewMessage(req.ConversationID, "assistant", "", req.UserID, req.ProjectID)

	// 🔄 NEW: Initialize streaming state tracking
	streamState := &StreamState{
		ConversationID:     req.ConversationID,
		UserID:            req.UserID,
		ProjectID:         req.ProjectID,
		MessageID:         assistantMsg.ID,
		CurrentContent:     "",
		StartTime:         time.Now(),
		IsActive:          true,
		ActiveConnectionIDs: make(map[string]bool),
		AllConnectionIDs:    make(map[string]bool),  // 🔄 NEW: Track all connections
		Mutex:             sync.RWMutex{},
	}

	// 🔄 NEW: Add streaming state to tracking BEFORE creating callback
	s.streamingMutex.Lock()
	s.activeStreams[req.ConversationID] = streamState
	s.streamingMutex.Unlock()
	
	// 🔄 NEW: Add the originating connection to active connections for this stream
	if req.ConnectionID != "" {
		streamState.Mutex.Lock()
		streamState.ActiveConnectionIDs[req.ConnectionID] = true
		streamState.AllConnectionIDs[req.ConnectionID] = true  // 🔄 NEW: Track all connections
		streamState.Mutex.Unlock()
		log.Printf("🔄 Added connection %s to stream %s", req.ConnectionID, req.ConversationID)
	}
	
	log.Printf("🔄 Started tracking streaming state for conversation: %s", req.ConversationID)

	// Start streaming response
	streamStarted := false
	tokenCount := 0
	lastSentLength := 0

	callback := func(chunk *llm.StreamingChunk) error {
		// 🔥 DETAILED LOGGING: Log every chunk received from LLM
		log.Printf("📦 LLM CHUNK RECEIVED:")
		log.Printf("   • Content: \"%s\"", chunk.Content)
		log.Printf("   • Content Length: %d", len(chunk.Content))
		log.Printf("   • Done: %t", chunk.Done)
		log.Printf("   • Tokens Used: %d", chunk.TokensUsed)
		log.Printf("   • Tool Calls: %v", chunk.ToolCalls)
		log.Printf("   • Stream Started: %t", streamStarted)

		// Track token usage
		var chunkTokens int64 = 0
		if chunk.TokensUsed > 0 {
			chunkTokens = int64(chunk.TokensUsed)
		}

		// Log first chunk and completion
		if !streamStarted && chunk.Content != "" {
			log.Printf("🎯 Chat service: Starting to stream chunk to WebSocket for conversation %s", req.ConversationID)
			log.Printf("🎯 FIRST CHUNK CONTENT: \"%s\"", chunk.Content)
			streamStarted = true
		}
		if chunk.Done {
			log.Printf("🎯 Chat service: Final chunk processed, broadcasting to WebSocket for conversation %s", req.ConversationID)
		}

		// 🔄 CRITICAL: Update the streaming state stored in activeStreams map
		if chunk.Content != "" {
			s.streamingMutex.RLock()
			if activeStream, exists := s.activeStreams[req.ConversationID]; exists {
				activeStream.CurrentContent += chunk.Content
				activeStream.LastChunk = time.Now()
				// Also update local reference for consistency
				streamState.CurrentContent = activeStream.CurrentContent
				streamState.LastChunk = activeStream.LastChunk
				
				// Count tokens (rough estimation: 1 token ≈ 4 characters)
				tokenCount += len(chunk.Content) / 4
				if len(chunk.Content) % 4 != 0 {
					tokenCount += 1
				}
				
				// 🔥 DEBUG: Log content updates
				log.Printf("🔥 DEBUG: Updated streaming content for %s: '%s' (total length: %d, token count: %d)", 
					req.ConversationID, activeStream.CurrentContent, len(activeStream.CurrentContent), tokenCount)
			} else {
				// Fallback: update local state if not in map
				streamState.CurrentContent += chunk.Content
				streamState.LastChunk = time.Now()
				// Count tokens for fallback
				tokenCount += len(chunk.Content) / 4
				if len(chunk.Content) % 4 != 0 {
					tokenCount += 1
				}
				log.Printf("🔥 DEBUG: Stream state not found in map, updated local state")
			}
			s.streamingMutex.RUnlock()  // 🔥 FIX: Use RUnlock() for RLock()
		}

		// Check token limit using connection reference
		var tokensUsed, tokensLimit, tokensRemaining int64
		if req.Connection != nil {
			tokensUsed, tokensLimit, tokensRemaining = req.Connection.GetTokenUsage()
			
			// Apply new tokens to get updated state
			if chunkTokens > 0 {
				if !req.AddTokensFunc(chunkTokens) {
					// Send token limit exceeded message
					errorResponse := msglib.NewWebSocketMessage(
						"error",
						gin.H{
							"error": "Token limit exceeded",
							"code": "TOKEN_LIMIT_EXCEEDED",
							"conversation_id": req.ConversationID,
						},
						tokensUsed, tokensLimit, tokensRemaining,
					)
					errorResponse.Timestamp = time.Now().UnixMilli()
					s.hub.BroadcastToProject(req.ProjectID, errorResponse)
					return fmt.Errorf("token limit exceeded for connection %s", req.ConnectionID)
				}
				// Get updated token usage after adding tokens
				tokensUsed, tokensLimit, tokensRemaining = req.Connection.GetTokenUsage()
			}
		} else {
			// Fallback for when connection is not available
			tokensUsed, tokensLimit, tokensRemaining = chunkTokens, 1000000, 1000000-chunkTokens
		}

		// Accumulate content
		if chunk.Content != "" {
			assistantMsg.Content += chunk.Content
			assistantMsg.CreatedAt = time.Now()
		}

		// 🔄 NEW: Determine if we should send accumulated content (every 30 tokens or on completion)
		// Send when: first chunk, every 30 tokens, OR when remaining tokens would never trigger another batch
	shouldSend := chunk.Done || (tokenCount > 0 && tokenCount % 30 == 0) || (!streamStarted && chunk.Content != "")
		
		if shouldSend {
			// Get accumulated content from stream state
			s.streamingMutex.RLock()
			var accumulatedContent string
			if activeStream, exists := s.activeStreams[req.ConversationID]; exists {
				accumulatedContent = activeStream.CurrentContent
			} else {
				accumulatedContent = streamState.CurrentContent
			}
			s.streamingMutex.RUnlock()

			// Calculate how much new content we're sending
			newContent := ""
			if len(accumulatedContent) > lastSentLength {
				newContent = accumulatedContent[lastSentLength:]
				lastSentLength = len(accumulatedContent)
			}

			// 🔥 DETAILED LOGGING: Create WebSocket response message with accumulated content
			log.Printf("📨 CREATING WEBSOCKET RESPONSE MESSAGE:")
			log.Printf("   • Conversation ID: %s", req.ConversationID)
			log.Printf("   • Should Send: %t", shouldSend)
			log.Printf("   • Token Count: %d", tokenCount)
			log.Printf("   • Accumulated Content: \"%s\"", accumulatedContent)
			log.Printf("   • New Content Since Last Send: \"%s\"", newContent)
			log.Printf("   • New Content Length: %d", len(newContent))
			log.Printf("   • Message ID: %s", assistantMsg.ID)
			log.Printf("   • Done: %t", chunk.Done)
			log.Printf("   • Tokens Used: %d", tokensUsed)
			log.Printf("   • Tokens Remaining: %d", tokensRemaining)

			response := &msglib.WebSocketMessage{
				Type: "assistant_response",
				Data: gin.H{
					"conversation_id": req.ConversationID,
					"content":         accumulatedContent, // 🔄 Send accumulated content from stream state
					"message_id":      assistantMsg.ID,
					"timestamp":       time.Now().UnixMilli(),
					"done":            chunk.Done,
					"tool_calls":      chunk.ToolCalls,
				},
				Timestamp:       time.Now().UnixMilli(),
				TokensUsed:     tokensUsed,
				TokensLimit:    tokensLimit,
				TokensRemaining: tokensRemaining,
			}

			log.Printf("📨 WEBSOCKET RESPONSE CREATED:")
			log.Printf("   • Type: %s", response.Type)
			log.Printf("   • Timestamp: %d", response.Timestamp)
			log.Printf("   • Data Keys: %v", getMapKeys(response.Data.(gin.H)))

			// For first chunk, include the message structure
			if !streamStarted && chunk.Content != "" {
				response.Data.(gin.H)["message"] = assistantMsg
				response.Data.(gin.H)["done"] = false
				streamStarted = true
				log.Printf("📡 BROADCASTING FIRST ACCUMULATED CHUNK TO WEBSOCKET:")
				log.Printf("   • Accumulated Content: '%s'", accumulatedContent)
				log.Printf("   • Tokens Used: %d", tokensUsed)
				log.Printf("   • Tokens Remaining: %d", tokensRemaining)
			}

			if chunk.Done {
				log.Printf("📡 BROADCASTING FINAL ACCUMULATED CHUNK TO WEBSOCKET:")
				log.Printf("   • Final Accumulated Content: '%s'", accumulatedContent)
				log.Printf("   • Content Length: %d", len(accumulatedContent))
				log.Printf("   • Done: %t", chunk.Done)
				log.Printf("   • Tokens Used: %d", tokensUsed)
			} else if tokenCount % 30 == 0 {
				log.Printf("📡 BROADCASTING 30-TOKEN ACCUMULATED CHUNK TO WEBSOCKET:")
				log.Printf("   • Accumulated Content: '%s'", accumulatedContent)
				log.Printf("   • Content Length: %d", len(accumulatedContent))
				log.Printf("   • Token Count: %d", tokenCount)
				log.Printf("   • Tokens Used: %d", tokensUsed)
			}
				
			// 🔄 NEW: Send only to active connections for this stream
			log.Printf("🎯 SENDING ACCUMULATED CONTENT TO ACTIVE CONNECTIONS FOR STREAM %s", req.ConversationID)
			if err := s.SendStreamToActiveConnections(req.ConversationID, &response); err != nil {
				log.Printf("❌ ERROR SENDING STREAM TO ACTIVE CONNECTIONS: %v", err)
				log.Printf("🔄 FALLING BACK TO PROJECT BROADCAST...")
				// Fallback to project broadcast if targeted send fails
				s.hub.BroadcastToProject(req.ProjectID, &response)
				log.Printf("✅ FALLBACK PROJECT BROADCAST COMPLETED")
			} else {
				log.Printf("✅ ACCUMULATED STREAM SENT TO ACTIVE CONNECTIONS SUCCESSFULLY")
			}
		} else {
			log.Printf("⏸️ NOT SENDING - Token count: %d (next send at %d)", tokenCount, ((tokenCount/30)+1)*30)
		}
		
		return nil
	}

	// Execute LLM call with streaming
	log.Printf("🤖 EXECUTING LLM STREAMING CALL...")
	log.Printf("   • LLM Request:")
	log.Printf("     - Messages Count: %d", len(llmReq.Messages))
	log.Printf("     - Max Tokens: %d", llmReq.MaxTokens)
	log.Printf("     - Temperature: %f", llmReq.Temperature)
	log.Printf("     - Tools Count: %d", len(llmReq.Tools))
	
	err := s.llmClient.StreamChat(ctx, llmReq, callback)

	if err != nil {
		// 🔄 NEW: Clear streaming state on error
		log.Printf("❌ LLM STREAMING FAILED: %v", err)
		s.streamingMutex.Lock()
		delete(s.activeStreams, req.ConversationID)
		s.streamingMutex.Unlock()
		log.Printf("🔄 CLEARED STREAMING STATE DUE TO ERROR: %s", req.ConversationID)
		
		// Update conversation status to interrupted when streaming fails
		if updateErr := s.UpdateConversationStatus(req.ConversationID, req.UserID, "interrupted"); updateErr != nil {
			log.Printf("Failed to update conversation status to interrupted: %v", updateErr)
		}
		
		// Send error to client
		errorResponse := WebSocketMessage{
			Type: "error",
			Data: gin.H{
				"conversation_id": req.ConversationID,
				"error":           "Failed to get AI response: " + err.Error(),
				"code":            "AI_RESPONSE_ERROR",
				"details": gin.H{
					"original_error": err.Error(),
				},
			},
			Timestamp: time.Now().UnixMilli(),
		}
		s.hub.BroadcastToProject(req.ProjectID, errorResponse)
		return err
	}

	log.Printf("✅ LLM STREAMING COMPLETED SUCCESSFULLY")
	log.Printf("   • Final Message Content Length: %d", len(assistantMsg.Content))
	log.Printf("   • Tool Calls Count: %d", len(assistantMsg.ToolCalls))

	// Process tool calls if any
	if len(assistantMsg.ToolCalls) > 0 {
		log.Printf("🔧 PROCESSING %d TOOL CALLS", len(assistantMsg.ToolCalls))
		if err := s.processToolCalls(ctx, req, assistantMsg); err != nil {
			log.Printf("❌ ERROR PROCESSING TOOL CALLS: %v", err)
		} else {
			log.Printf("✅ TOOL CALLS PROCESSED SUCCESSFULLY")
		}
	}

	// Save complete assistant message
	log.Printf("💾 SAVING COMPLETE ASSISTANT MESSAGE...")
	if err := s.saveMessage(ctx, assistantMsg); err != nil {
		log.Printf("❌ FAILED TO SAVE ASSISTANT MESSAGE: %v", err)
	} else {
		log.Printf("✅ ASSISTANT MESSAGE SAVED SUCCESSFULLY")
	}

	// 🔄 NEW: Mark streaming as completed but keep it available for frontend
	s.streamingMutex.Lock()
	if streamState, exists := s.activeStreams[req.ConversationID]; exists {
		streamState.IsActive = false
		log.Printf("🔄 MARKED STREAM AS COMPLETED BUT KEEPING IN MEMORY: %s", req.ConversationID)
		
		// Schedule cleanup after 30 seconds
		go func(conversationID string) {
			time.Sleep(30 * time.Second)
			s.streamingMutex.Lock()
			delete(s.activeStreams, conversationID)
			s.streamingMutex.Unlock()
			log.Printf("🧹 CLEANED UP COMPLETED STREAM AFTER 30s: %s", conversationID)
		}(req.ConversationID)
	}
	s.streamingMutex.Unlock()
	
	// Update conversation status to completed when streaming finishes
	log.Printf("📊 UPDATING CONVERSATION STATUS TO 'completed'...")
	if err := s.UpdateConversationStatus(req.ConversationID, req.UserID, "completed"); err != nil {
		log.Printf("❌ FAILED TO UPDATE CONVERSATION STATUS TO completed: %v", err)
	} else {
		log.Printf("✅ CONVERSATION STATUS UPDATED TO completed")
	}

	// Send completion message
	log.Printf("📡 SENDING COMPLETION MESSAGE...")
	completionResponse := WebSocketMessage{
		Type:      "assistant_response",
		Timestamp: time.Now().UnixMilli(),
		Data: gin.H{
			"conversation_id": req.ConversationID,
			"message_id":      assistantMsg.ID,
			"timestamp":       time.Now().Format(time.RFC3339),
			"done":            true,
		},
	}
	log.Printf("📡 BROADCASTING COMPLETION MESSAGE TO PROJECT %s", req.ProjectID)
	s.hub.BroadcastToProject(req.ProjectID, completionResponse)
	log.Printf("✅ COMPLETION MESSAGE BROADCASTED")

	log.Printf("🎉 STREAMLLMRESPONSE COMPLETED SUCCESSFULLY FOR CONVERSATION: %s", req.ConversationID)
	return nil
}

// processToolCalls executes pending tool calls
func (s *chatService) processToolCalls(ctx context.Context, req *ChatRequest, assistantMsg *Message) error {
	for _, toolCall := range assistantMsg.ToolCalls {
		if toolCall.Status != "pending" {
			continue
		}

		// Update status to executing
		assistantMsg.UpdateToolCallStatus(toolCall.ID, "executing", "", "")
		s.hub.BroadcastToProject(req.ProjectID, WebSocketMessage{
			Type: "tool_execution_started",
			Data: gin.H{
				"tool_name":       toolCall.Function.Name,
				"tool_call_id":    toolCall.ID,
				"conversation_id": req.ConversationID,
				"message_id":      assistantMsg.ID,
			},
			Timestamp: time.Now().UnixMilli(),
		})

		// Execute tool
		args, ok := toolCall.Function.Arguments.(map[string]interface{})
		if !ok {
			args = make(map[string]interface{})
		}
		result, err := s.toolRegistry.ExecuteTool(ctx, req.UserID, req.ProjectID, toolCall.Function.Name, args)

		var status string
		var resultJSON string
		if err != nil {
			status = "failed"
			resultJSON = fmt.Sprintf(`{"error": "%s"}`, err.Error())
		} else {
			status = "completed"
			if resultBytes, err := json.Marshal(result); err == nil {
				resultJSON = string(resultBytes)
			} else {
				resultJSON = `{"error": "Failed to marshal result"}`
			}
		}

		// Update tool call status
		assistantMsg.UpdateToolCallStatus(toolCall.ID, status, resultJSON, "")

		// Broadcast tool execution result
		if status == "completed" {
			s.hub.BroadcastToProject(req.ProjectID, WebSocketMessage{
				Type:      "tool_execution_completed",
				Timestamp: time.Now().UnixMilli(),
				Data: gin.H{
					"tool_name":       toolCall.Function.Name,
					"tool_call_id":    toolCall.ID,
					"conversation_id": req.ConversationID,
					"result":          json.RawMessage(resultJSON),
					"success":         true,
				},
			})
		} else if status == "failed" {
			s.hub.BroadcastToProject(req.ProjectID, WebSocketMessage{
				Type:      "tool_execution_failed",
				Timestamp: time.Now().UnixMilli(),
				Data: gin.H{
					"tool_name":       toolCall.Function.Name,
					"tool_call_id":    toolCall.ID,
					"conversation_id": req.ConversationID,
					"error":           resultJSON,
					"error_code":      "EXECUTION_ERROR",
				},
			})
		}
	}

	return nil
}

// broadcastToolStatus sends tool execution status to clients
func (s *chatService) broadcastToolStatus(projectID, conversationID, messageID string, index int, toolCall ToolCall) {
	toolStatus := WebSocketMessage{
		Type: "tool_execution",
		Data: gin.H{
			"conversation_id": conversationID,
			"message_id":      messageID,
			"tool_index":      index,
			"tool_call":       toolCall,
		},
	}
	s.hub.BroadcastToProject(projectID, toolStatus)
}

// 🔄 NEW: LoadStreamingConversation loads conversation including streaming state
func (s *chatService) LoadStreamingConversation(conversationID, userID string) (*ConversationDetails, error) {
	log.Printf("🔥 DEBUG: LoadStreamingConversation called for conv: %s, user: %s", conversationID, userID)
	
	// First, get the complete conversation from database (this gets all saved history)
	dbDetails, err := s.GetConversation(conversationID, userID)
	if err != nil {
		log.Printf("🔥 ERROR: Failed to get conversation from database: %v", err)
		return nil, fmt.Errorf("failed to get conversation from database: %w", err)
	}
	
	log.Printf("🔥 DEBUG: Loaded %d messages from database", len(dbDetails.Messages))
	for i, msg := range dbDetails.Messages {
		log.Printf("🔥 Database message %d: %s - '%s' (length: %d)", 
			i, msg.Role, msg.Content, len(msg.Content))
	}
	
	// Then, check if there's an active streaming state
	s.streamingMutex.RLock()
	streamState, hasStream := s.activeStreams[conversationID]
	s.streamingMutex.RUnlock()
	
	var contentLength int
	if hasStream {
		contentLength = len(streamState.CurrentContent)
	}
	log.Printf("🔥 DEBUG: Checking streaming state for %s: hasStream=%v, content_length=%d", 
		conversationID, hasStream, contentLength)
	
	if hasStream && streamState.IsActive {
		log.Printf("Loading streaming conversation %s with partial content: %s", conversationID, streamState.CurrentContent)
		
		// 🔥 FIX: Check if the streaming message already exists in database
		// The streaming message ID should match what would be in database
		streamingAssistantMsg := &Message{
			ID:            streamState.MessageID,
			ConversationID: streamState.ConversationID,
			Role:          "assistant",
			Content:        streamState.CurrentContent,
			CreatedAt:      streamState.StartTime,
			UserID:        streamState.UserID,
			ProjectID:     streamState.ProjectID,
		}
		
		// Check if partial message already exists in the loaded messages
		assistantExists := false
		messageIndex := -1
		for i, msg := range dbDetails.Messages {
			if msg.ID == streamState.MessageID {
				assistantExists = true
				messageIndex = i
				// Update existing message with current streaming content
				if len(msg.Content) < len(streamState.CurrentContent) {
					// Only update if streaming content is longer (has new data)
					msg.Content = streamState.CurrentContent
				log.Printf("Updated existing assistant message with streaming content, old: %d, new: %d", 
					len(msg.Content), len(streamState.CurrentContent))
				}
				break
			}
		}
		
		// Add streaming assistant message if not already present in database
		if !assistantExists {
			// This is a brand new streaming message not yet saved to DB
			log.Printf("🔥 DEBUG: Adding new streaming assistant message (not in database yet): %s", streamState.MessageID)
			dbDetails.Messages = append(dbDetails.Messages, streamingAssistantMsg)
		} else {
			log.Printf("Found existing assistant message in database: %s at index %d", streamState.MessageID, messageIndex)
		}
		
		log.Printf("Final message count after streaming integration: %d", len(dbDetails.Messages))
	} else {
		log.Printf("No active stream for conversation: %s, returning database-only data", conversationID)
	}
	
	return dbDetails, nil
}

// Helper methods

func (s *chatService) saveMessage(ctx context.Context, msg *Message) error {
	toolCallsJSON, _ := json.Marshal(msg.ToolCalls)
	metadataJSON, _ := json.Marshal(msg.Metadata)

	query := `
		INSERT INTO messages (id, conversation_id, role, content, metadata, tool_calls, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := s.db.Exec(ctx, query,
		msg.ID, msg.ConversationID, msg.Role, msg.Content,
		metadataJSON, toolCallsJSON, msg.CreatedAt,
	)

	return err
}

func (s *chatService) getConversationHistory(ctx context.Context, conversationID, userID string) ([]*Message, error) {
	query := `
		SELECT id, conversation_id, role, content, metadata, tool_calls, created_at
		FROM messages
		WHERE conversation_id = $1
		ORDER BY created_at ASC
		LIMIT 50 -- Limit context window
	`

	rows, err := s.db.Query(ctx, query, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*Message
	for rows.Next() {
		var msg Message
		var toolCallsJSON []byte
		var metadataJSON []byte

		if err := rows.Scan(
			&msg.ID, &msg.ConversationID, &msg.Role, &msg.Content,
			&metadataJSON, &toolCallsJSON, &msg.CreatedAt,
		); err != nil {
			return nil, err
		}

		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &msg.Metadata)
		}
		if len(toolCallsJSON) > 0 {
			json.Unmarshal(toolCallsJSON, &msg.ToolCalls)
		}

		messages = append(messages, &msg)
	}

	return messages, nil
}

func (s *chatService) convertToOpenAIMessages(messages []*Message) []openai.ChatCompletionMessageParamUnion {
	var openaiMessages []openai.ChatCompletionMessageParamUnion

	// Add system message if needed (project context, rules, etc.)
	// TODO: Implement system message generation based on project

	for _, msg := range messages {
		if msg.Role == "user" || msg.Role == "assistant" || msg.Role == "system" {
			if msg.Role == "user" {
				openaiMessages = append(openaiMessages, openai.UserMessage(msg.Content))
			} else if msg.Role == "assistant" {
				// Handle assistant messages with tool calls
				if len(msg.ToolCalls) > 0 {
					// For now, skip tool calls in assistant messages to avoid API issues
					// TODO: Fix tool call format conversion when OpenAI API stabilizes
					openaiMessages = append(openaiMessages, openai.AssistantMessage(msg.Content))
				} else {
					// Basic assistant message without tool calls
					openaiMessages = append(openaiMessages, openai.AssistantMessage(msg.Content))
				}
			} else if msg.Role == "system" {
				openaiMessages = append(openaiMessages, openai.SystemMessage(msg.Content))
			}
		}
	}

	return openaiMessages
}

func (s *chatService) convertTools(availableTools []tools.Tool) []Tool {
	var convertedTools []Tool

	for _, tool := range availableTools {
		// Convert tool parameters map
		parameters := make(map[string]interface{})
		for k, v := range tool.Parameters() {
			parameters[k] = map[string]interface{}{
				"type":        "string",
				"description": v.Description,
			}
		}

		convertedTool := Tool{
			Name:        tool.Name(),
			Description: tool.Description(),
			Parameters:  parameters,
			Type:        "function",
		}
		convertedTools = append(convertedTools, convertedTool)
	}

	return convertedTools
}

// 🔄 NEW: Streaming state management methods

// GetConversationStatus returns detailed conversation status including streaming state
func (s *chatService) GetConversationStatus(conversationID, userID string) (gin.H, error) {
	// Check database for conversation existence
	// First verify conversation exists and belongs to user
	conversationQuery := `
		SELECT id, title, created_at, updated_at 
		FROM conversations 
		WHERE id = $1 AND user_id = $2`
	
	rows, err := s.db.Query(context.Background(), conversationQuery, conversationID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check conversation: %w", err)
	}
	defer rows.Close()
	
	conversationExists := rows.Next()
	if !conversationExists {
		return gin.H{
			"conversation_id": conversationID,
			"exists": false,
			"is_processing": false,
			"error": "Conversation not found",
		}, nil
	}
	
	// Check streaming state
	var isProcessing bool = false
	var currentContent string = ""
	var startTime time.Time
	
	s.streamingMutex.RLock()
	streamState, hasStream := s.activeStreams[conversationID]
	s.streamingMutex.RUnlock()
	
	if hasStream && streamState.IsActive {
		isProcessing = true
		currentContent = streamState.CurrentContent
		startTime = streamState.StartTime
		log.Printf("Found active stream for conversation %s: content_length=%d", 
			conversationID, len(streamState.CurrentContent))
	}
	
	return gin.H{
		"conversation_id": conversationID,
		"exists": true,
		"is_processing": isProcessing,
		"current_content": currentContent,
		"streaming_since": startTime.UnixMilli(),
	}, nil
}

// GetStreamState returns the current streaming state for a conversation
func (s *chatService) GetStreamState(conversationID string) (*StreamState, error) {
	s.streamingMutex.RLock()
	defer s.streamingMutex.RUnlock()
	
	streamState, exists := s.activeStreams[conversationID]
	if !exists {
		return nil, fmt.Errorf("no active stream for conversation: %s", conversationID)
	}
	
	return streamState, nil
}

// GetAllActiveStreams returns all currently active streaming conversations
func (s *chatService) GetAllActiveStreams() map[string]*StreamState {
	s.streamingMutex.RLock()
	defer s.streamingMutex.RUnlock()
	
	result := make(map[string]*StreamState)
	for convID, streamState := range s.activeStreams {
		result[convID] = streamState
	}
	
	return result
}

// ClearStreamState removes the streaming state for a conversation
func (s *chatService) ClearStreamState(conversationID string) error {
	s.streamingMutex.Lock()
	defer s.streamingMutex.Unlock()
	
	delete(s.activeStreams, conversationID)
	log.Printf("Cleared streaming state for conversation: %s", conversationID)
	return nil
}

// 🔄 NEW: AttachConnectionToStream adds a connection to active stream
func (s *chatService) AttachConnectionToStream(conversationID, connectionID string) error {
	s.streamingMutex.Lock()
	defer s.streamingMutex.Unlock()
	
	streamState, exists := s.activeStreams[conversationID]
	if !exists {
		return fmt.Errorf("no active stream for conversation: %s", conversationID)
	}
	
	streamState.Mutex.Lock()
	streamState.ActiveConnectionIDs[connectionID] = true
	streamState.AllConnectionIDs[connectionID] = true  // 🔄 NEW: Track all connections
	streamState.Mutex.Unlock()
	
	log.Printf("Attached connection %s to stream %s", connectionID, conversationID)
	return nil
}

// 🔄 NEW: DetachConnectionFromStream removes a connection from active stream
func (s *chatService) DetachConnectionFromStream(conversationID, connectionID string) error {
	s.streamingMutex.Lock()
	defer s.streamingMutex.Unlock()
	
	streamState, exists := s.activeStreams[conversationID]
	if !exists {
		return fmt.Errorf("no active stream for conversation: %s", conversationID)
	}
	
	streamState.Mutex.Lock()
	delete(streamState.ActiveConnectionIDs, connectionID)
	remainingConnections := len(streamState.ActiveConnectionIDs)
	streamState.Mutex.Unlock()
	
	log.Printf("Detached connection %s from stream %s, remaining connections: %d", connectionID, conversationID, remainingConnections)
	
	// 🔥 CRITICAL: If no more active connections, mark conversation as interrupted
	if remainingConnections == 0 {
		log.Printf("🔌 No more active connections for conversation %s, marking as interrupted", conversationID)
		
		// Update conversation status to interrupted in database
		err := s.UpdateConversationStatus(conversationID, "", "interrupted")
		if err != nil {
			log.Printf("❌ Failed to update conversation status to interrupted: %v", err)
		} else {
			log.Printf("✅ Updated conversation %s status to interrupted", conversationID)
		}
	}
	
	return nil
}

// 🔄 NEW: GetActiveConnectionsForStream returns current active connection count for a stream
func (s *chatService) GetActiveConnectionsForStream(conversationID string) int {
	s.streamingMutex.RLock()
	defer s.streamingMutex.RUnlock()
	
	streamState, exists := s.activeStreams[conversationID]
	if !exists {
		return 0
	}
	
	streamState.Mutex.RLock()
	count := len(streamState.ActiveConnectionIDs)
	streamState.Mutex.RUnlock()
	
	return count
}

// 🔄 NEW: SendStreamToActiveConnections sends a message only to connections tracking this stream
// Helper function to get map keys for logging
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Helper function to get active stream keys for logging
func getActiveStreamKeys(streams map[string]*StreamState) []string {
	keys := make([]string, 0, len(streams))
	for k := range streams {
		keys = append(keys, k)
	}
	return keys
}

func (s *chatService) SendStreamToActiveConnections(conversationID string, message interface{}) error {
	log.Printf("🎯 SendStreamToActiveConnections CALLED:")
	log.Printf("   • Conversation ID: %s", conversationID)
	log.Printf("   • Message Type: %T", message)

	s.streamingMutex.RLock()
	streamState, exists := s.activeStreams[conversationID]
	s.streamingMutex.RUnlock()
	
	if !exists {
		log.Printf("❌ NO ACTIVE STREAM FOUND FOR CONVERSATION: %s", conversationID)
		log.Printf("   • Available Streams: %v", getActiveStreamKeys(s.activeStreams))
		return fmt.Errorf("no active stream for conversation: %s", conversationID)
	}

	log.Printf("✅ ACTIVE STREAM FOUND:")
	log.Printf("   • Stream Exists: %t", exists)
	log.Printf("   • Active Connections Count: %d", len(streamState.ActiveConnectionIDs))
	log.Printf("   • All Connections Count: %d", len(streamState.AllConnectionIDs))
	
	streamState.Mutex.RLock()
	activeConnectionIDs := make([]string, 0, len(streamState.ActiveConnectionIDs))
	for connID := range streamState.ActiveConnectionIDs {
		activeConnectionIDs = append(activeConnectionIDs, connID)
	}
	streamState.Mutex.RUnlock()
	
	// 🔄 CRITICAL FIX: If no active connections but stream is still alive, use project broadcast fallback
	if len(activeConnectionIDs) == 0 {
		streamState.Mutex.RLock()
		allConnectionIDs := make([]string, 0, len(streamState.AllConnectionIDs))
		for connID := range streamState.AllConnectionIDs {
			allConnectionIDs = append(allConnectionIDs, connID)
		}
		streamState.Mutex.RUnlock()
		
		log.Printf("⚠️ NO ACTIVE CONNECTIONS FOR STREAM %s", conversationID)
		log.Printf("   • Active Connection IDs: %v", activeConnectionIDs)
		log.Printf("   • All Connection IDs: %v", allConnectionIDs)
		log.Printf("🔄 USING PROJECT BROADCAST FALLBACK...")
		
		// Fallback to project broadcast if we have any connections that ever joined this stream
		if len(streamState.AllConnectionIDs) > 0 {
			log.Printf("📡 BROADCASTING TO PROJECT %s", streamState.ProjectID)
			s.hub.BroadcastToProject(streamState.ProjectID, message)
			log.Printf("✅ PROJECT BROADCAST COMPLETED")
			return nil
		}
		
		// No connections at all
		log.Printf("❌ NO CONNECTIONS EVER JOINED STREAM %s - CANNOT SEND MESSAGE", conversationID)
		return fmt.Errorf("no connections available for stream: %s", conversationID)
	}
	
	log.Printf("🎯 SENDING TO %d ACTIVE CONNECTIONS: %v", len(activeConnectionIDs), activeConnectionIDs)
	
	// Try to get concrete hub for targeted sending, fallback to broadcast
	if websocketHub, ok := s.hub.(interface {
		GetConnectionByID(string) interface{}
		SendToConnection(interface{}, interface{}) error
	}); ok {
		log.Printf("🔗 USING TARGETED CONNECTION SENDING")
		// Send to each specific connection
		for _, connID := range activeConnectionIDs {
			log.Printf("📤 SENDING TO CONNECTION: %s", connID)
			if conn := websocketHub.GetConnectionByID(connID); conn != nil {
				if err := websocketHub.SendToConnection(conn, message); err != nil {
					log.Printf("❌ ERROR SENDING TO CONNECTION %s: %v", connID, err)
				} else {
					log.Printf("✅ SENT TO CONNECTION %s SUCCESSFULLY", connID)
				}
			} else {
				log.Printf("⚠️ CONNECTION %s NOT FOUND", connID)
			}
		}
	} else {
		log.Printf("🔄 FALLING BACK TO PROJECT BROADCAST")
		// Fallback to project broadcast
		s.hub.BroadcastToProject(streamState.ProjectID, message)
		log.Printf("✅ PROJECT BROADCAST COMPLETED")
	}
	
	log.Printf("✅ SendStreamToActiveConnections COMPLETED")
	return nil
}

// GetActiveStreamingMessage returns only the active streaming message from memory
func (s *chatService) GetActiveStreamingMessage(conversationID, userID string) (*StreamState, error) {
	s.streamingMutex.RLock()
	streamState, exists := s.activeStreams[conversationID]
	
	// Log all active streams in memory (no filtering)
	log.Printf("🔍 DEBUG: All active streams in memory during lookup for %s:", conversationID)
	if len(s.activeStreams) == 0 {
		log.Printf("   • No active streams found in memory")
	} else {
		for convID, state := range s.activeStreams {
			log.Printf("   • Stream %s:", convID)
			log.Printf("     - ConversationID: %s", state.ConversationID)
			log.Printf("     - UserID: %s", state.UserID)
			log.Printf("     - ProjectID: %s", state.ProjectID)
			log.Printf("     - MessageID: %s", state.MessageID)
			log.Printf("     - CurrentContent Length: %d", len(state.CurrentContent))
			log.Printf("     - StartTime: %s", state.StartTime.Format(time.RFC3339))
			log.Printf("     - LastChunk: %s", state.LastChunk.Format(time.RFC3339))
			log.Printf("     - IsActive: %t", state.IsActive)
			log.Printf("     - ActiveConnectionIDs: %d", len(state.ActiveConnectionIDs))
			log.Printf("     - AllConnectionIDs: %d", len(state.AllConnectionIDs))
		}
	}
	
	s.streamingMutex.RUnlock()
	
	log.Printf("🔍 DEBUG: StreamState lookup for conversation %s:", conversationID)
	log.Printf("   • Exists: %t", exists)
	if exists {
		log.Printf("   • Requested StreamState Data:")
		log.Printf("     - ConversationID: %s", streamState.ConversationID)
		log.Printf("     - UserID: %s", streamState.UserID)
		log.Printf("     - ProjectID: %s", streamState.ProjectID)
		log.Printf("     - MessageID: %s", streamState.MessageID)
		log.Printf("     - CurrentContent Length: %d", len(streamState.CurrentContent))
		log.Printf("     - StartTime: %s", streamState.StartTime.Format(time.RFC3339))
		log.Printf("     - LastChunk: %s", streamState.LastChunk.Format(time.RFC3339))
		log.Printf("     - IsActive: %t", streamState.IsActive)
		log.Printf("     - ActiveConnectionIDs: %v", streamState.ActiveConnectionIDs)
		log.Printf("     - AllConnectionIDs: %v", streamState.AllConnectionIDs)
	}
	
	if !exists {
		return nil, fmt.Errorf("no active stream for conversation: %s", conversationID)
	}
	
	// Verify the stream belongs to the requesting user
	if streamState.UserID != userID {
		return nil, fmt.Errorf("stream does not belong to user: %s", userID)
	}
	
	// Return stream whether active or completed (but don't return if it was explicitly cleared)
	status := "processing"
	if !streamState.IsActive {
		status = "completed"
	}
	
	log.Printf("🔄 Returning streaming message for conversation %s (status: %s, content length: %d)", 
		conversationID, status, len(streamState.CurrentContent))
	
	return streamState, nil
}
