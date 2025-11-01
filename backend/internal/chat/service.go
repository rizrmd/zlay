package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openai/openai-go"
	"zlay-backend/internal/llm"
	"zlay-backend/internal/messages"
	"zlay-backend/internal/tools"
)

// ChatService interface defines chat operations
type ChatService interface {
	ProcessUserMessage(req *ChatRequest) error
	CreateConversation(userID, projectID, title string) (*Conversation, error)
	GetConversations(userID, projectID string) ([]*Conversation, error)
	GetConversation(conversationID, userID string) (*ConversationDetails, error)
	DeleteConversation(conversationID, userID string) error
	WithLLMClient(llmClient llm.LLMClient) ChatService
}

// chatService implements ChatService interface
type chatService struct {
	db           tools.DBConnection
	hub          messages.Hub
	llmClient    llm.LLMClient
	toolRegistry tools.ToolRegistry
}

// NewChatService creates a new chat service
func NewChatService(db tools.DBConnection, hub messages.Hub, llmClient llm.LLMClient, toolRegistry tools.ToolRegistry) *chatService {
	return &chatService{
		db:           db,
		hub:          hub,
		llmClient:    llmClient,
		toolRegistry: toolRegistry,
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
	}
	return newService
}

// ProcessUserMessage handles an incoming user message
func (s *chatService) ProcessUserMessage(req *ChatRequest) error {
	ctx := context.Background()

	// Create and save user message
	userMsg := NewMessage(req.ConversationID, "user", req.Content, req.UserID, req.ProjectID)
	if err := s.saveMessage(ctx, userMsg); err != nil {
		return fmt.Errorf("failed to save user message: %w", err)
	}

	// Broadcast user message to project room
	s.hub.BroadcastToProject(req.ProjectID, tools.WebSocketMessage{
		Type: "user_message_sent",
		Data: gin.H{
			"message":       userMsg,
			"connection_id": req.ConnectionID,
		},
		Timestamp: time.Now().UnixMilli(),
	})

	// Get conversation history for context
	history, err := s.getConversationHistory(ctx, req.ConversationID, req.UserID)
	if err != nil {
		return fmt.Errorf("failed to get conversation history: %w", err)
	}

	// Get available tools for this project
	availableTools := s.toolRegistry.GetAvailableTools(req.ProjectID)

	// Convert messages to OpenAI format
	openaiMessages := s.convertToOpenAIMessages(history)

	// Start streaming response
	return s.streamLLMResponse(ctx, req, openaiMessages, s.convertTools(availableTools))
}

// CreateConversation creates a new conversation
func (s *chatService) CreateConversation(userID, projectID, title string) (*Conversation, error) {
	ctx := context.Background()
	conversation := NewConversation(projectID, userID, title)

	// Save to database
	query := `
		INSERT INTO conversations (id, project_id, user_id, title, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, project_id, user_id, title, created_at, updated_at
	`

	var conv Conversation
	err := s.db.QueryRow(ctx, query,
		conversation.ID, conversation.ProjectID, conversation.UserID,
		conversation.Title, conversation.CreatedAt, conversation.UpdatedAt,
	).Scan(
		&conv.ID, &conv.ProjectID, &conv.UserID,
		&conv.Title, &conv.CreatedAt, &conv.UpdatedAt,
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
		SELECT id, project_id, user_id, title, created_at, updated_at
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
			&conv.Title, &conv.CreatedAt, &conv.UpdatedAt,
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
		SELECT id, project_id, user_id, title, created_at, updated_at
		FROM conversations
		WHERE id = $1 AND user_id = $2
	`

	var conversation Conversation
	err := s.db.QueryRow(ctx, convQuery, conversationID, userID).Scan(
		&conversation.ID, &conversation.ProjectID, &conversation.UserID,
		&conversation.Title, &conversation.CreatedAt, &conversation.UpdatedAt,
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

// streamLLMResponse streams LLM response to WebSocket
func (s *chatService) streamLLMResponse(ctx context.Context, req *ChatRequest, messages []openai.ChatCompletionMessageParamUnion, tools []Tool) error {
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

	// Start streaming response
	streamStarted := false

	callback := func(chunk *llm.StreamingChunk) error {
		// Track token usage
		var chunkTokens int64 = 0
		if chunk.TokensUsed > 0 {
			chunkTokens = int64(chunk.TokensUsed)
		}

		// Check token limit using connection reference
		var tokensUsed, tokensLimit, tokensRemaining int64
		if req.Connection != nil {
			tokensUsed, tokensLimit, tokensRemaining = req.Connection.GetTokenUsage()
			
			// Apply new tokens to get updated state
			if chunkTokens > 0 {
				if !req.AddTokensFunc(chunkTokens) {
					// Send token limit exceeded message
					errorResponse := messages.NewWebSocketMessage(
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

		// Send streaming chunk to client with token info
		response := messages.WebSocketMessage{
			Type: "assistant_response",
			Data: gin.H{
				"conversation_id": req.ConversationID,
				"content":         chunk.Content,
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

		if !streamStarted && chunk.Content != "" {
			// First chunk, include message metadata
			response.Data = gin.H{
				"conversation_id": req.ConversationID,
				"message":         assistantMsg,
				"timestamp":       time.Now().UnixMilli(),
				"done":            false,
			}
			streamStarted = true
		}

		s.hub.BroadcastToProject(req.ProjectID, &response)
		return nil
	}

	// Execute LLM call with streaming
	err := s.llmClient.StreamChat(ctx, llmReq, callback)

	if err != nil {
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

	// Process tool calls if any
	if len(assistantMsg.ToolCalls) > 0 {
		if err := s.processToolCalls(ctx, req, assistantMsg); err != nil {
			log.Printf("Error processing tool calls: %v", err)
		}
	}

	// Save complete assistant message
	if err := s.saveMessage(ctx, assistantMsg); err != nil {
		log.Printf("Failed to save assistant message: %v", err)
	}

	// Send completion message
	completionResponse := WebSocketMessage{
		Type:      "assistant_response",
		Timestamp: time.Now().UnixMilli(),
		Data: gin.H{
			"conversation_id": req.ConversationID,
			"content":         assistantMsg.Content,
			"message_id":      assistantMsg.ID,
			"timestamp":       time.Now().Format(time.RFC3339),
			"done":            true,
		},
	}
	s.hub.BroadcastToProject(req.ProjectID, completionResponse)

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

// Helper methods

func (s *chatService) saveMessage(ctx context.Context, msg *Message) error {
	toolCallsJSON, _ := json.Marshal(msg.ToolCalls)
	metadataJSON, _ := json.Marshal(msg.Metadata)

	query := `
		INSERT INTO messages (id, conversation_id, role, content, metadata, tool_calls, created_at, user_id, project_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := s.db.Exec(ctx, query,
		msg.ID, msg.ConversationID, msg.Role, msg.Content,
		metadataJSON, toolCallsJSON, msg.CreatedAt, msg.UserID, msg.ProjectID,
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
