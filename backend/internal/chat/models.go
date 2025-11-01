package chat

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Message represents a chat message
type Message struct {
	ID           string            `json:"id" db:"id"`
	ConversationID string            `json:"conversation_id" db:"conversation_id"`
	Role         string            `json:"role" db:"role"` // user, assistant, system
	Content      string            `json:"content" db:"content"`
	Metadata     map[string]interface{} `json:"metadata" db:"metadata"`
	ToolCalls    []ToolCall        `json:"tool_calls,omitempty" db:"tool_calls"`
	CreatedAt    time.Time         `json:"created_at" db:"created_at"`
	UserID       string            `json:"user_id,omitempty" db:"user_id"`
	ProjectID    string            `json:"project_id,omitempty" db:"project_id"`
}

// ToolCall represents a function/tool call from the LLM
type ToolCall struct {
	ID       string                 `json:"id" db:"id"`
	Type     string                 `json:"type" db:"type"`
	Function ToolCallFunction       `json:"function" db:"function"`
	Status   string                 `json:"status,omitempty" db:"status"` // pending, executing, completed, failed
	Result   map[string]interface{} `json:"result,omitempty" db:"result"`
	Error    string                 `json:"error,omitempty" db:"error"`
}

// ToolCallFunction represents a function call
type ToolCallFunction struct {
	Name      string      `json:"name" db:"name"`
	Arguments interface{} `json:"arguments" db:"arguments"`
}

// Conversation represents a chat conversation
type Conversation struct {
	ID        string    `json:"id" db:"id"`
	ProjectID string    `json:"project_id" db:"project_id"`
	UserID   string    `json:"user_id" db:"user_id"`
	Title    string    `json:"title" db:"title"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// ToolExecution represents a tool execution record
type ToolExecution struct {
	ID              string        `json:"id" db:"id"`
	MessageID       string        `json:"message_id" db:"message_id"`
	ToolName        string        `json:"tool_name" db:"tool_name"`
	ToolParameters  string        `json:"tool_parameters" db:"tool_parameters"` // JSON string
	ToolResult      string        `json:"tool_result,omitempty" db:"tool_result"` // JSON string
	ExecutionStatus string        `json:"execution_status" db:"execution_status"` // pending, executing, completed, failed
	ExecutionTimeMs int           `json:"execution_time_ms,omitempty" db:"execution_time_ms"`
	CreatedAt       time.Time     `json:"created_at" db:"created_at"`
}

// ChatRequest represents an incoming chat request
type ChatRequest struct {
	ConversationID string `json:"conversation_id"`
	Content       string `json:"content"`
	UserID        string `json:"user_id"`
	ClientID      string `json:"client_id"`
	ProjectID     string `json:"project_id"`
	ConnectionID  string `json:"connection_id"`
	
	// Token tracking function (optional)
	AddTokensFunc func(tokens int64) bool
	
	// Connection reference for real-time token info
	Connection interface {
		GetTokenUsage() (used int64, limit int64, remaining int64)
	}
}

// ChatResponse represents a streaming chat response
type ChatResponse struct {
	ConversationID string    `json:"conversation_id"`
	Content       string    `json:"content"`
	Role          string    `json:"role"`
	MessageID     string    `json:"message_id"`
	Timestamp     time.Time `json:"timestamp"`
	Done          bool      `json:"done"`
	ToolCalls     []ToolCall `json:"tool_calls,omitempty"`
	Error         string    `json:"error,omitempty"`
}

// ConversationDetails represents a conversation with its messages
type ConversationDetails struct {
	Conversation *Conversation `json:"conversation"`
	Messages     []*Message     `json:"messages"`
	ToolStatus   map[string]string `json:"tool_status,omitempty"`
}

// Helper functions

// NewMessage creates a new message instance
func NewMessage(conversationID, role, content, userID, projectID string) *Message {
	return &Message{
		ID:             uuid.New().String(),
		ConversationID: conversationID,
		Role:           role,
		Content:        content,
		Metadata:       make(map[string]interface{}),
		ToolCalls:       make([]ToolCall, 0),
		CreatedAt:       time.Now(),
		UserID:         userID,
		ProjectID:      projectID,
	}
}

// NewConversation creates a new conversation instance
func NewConversation(projectID, userID, title string) *Conversation {
	now := time.Now()
	return &Conversation{
		ID:        uuid.New().String(),
		ProjectID: projectID,
		UserID:   userID,
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// NewToolCall creates a new tool call instance
func NewToolCall(toolID, toolType, name string, arguments interface{}) *ToolCall {
	return &ToolCall{
		ID:   toolID,
		Type: toolType,
		Function: ToolCallFunction{
			Name:      name,
			Arguments: arguments,
		},
		Status: "pending",
	}
}

// NewToolExecution creates a new tool execution instance
func NewToolExecution(messageID, toolName, toolParameters string) *ToolExecution {
	return &ToolExecution{
		ID:              uuid.New().String(),
		MessageID:       messageID,
		ToolName:        toolName,
		ToolParameters:  toolParameters,
		ExecutionStatus: "pending",
		CreatedAt:       time.Now(),
	}
}

// WebSocketMessage represents a WebSocket message
type WebSocketMessage struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp int64       `json:"timestamp,omitempty"`
}

// Tool represents an available tool/function
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	Type        string                 `json:"type"`
}

// IsUserMessage checks if message is from user
func (m *Message) IsUserMessage() bool {
	return m.Role == "user"
}

// IsAssistantMessage checks if message is from assistant
func (m *Message) IsAssistantMessage() bool {
	return m.Role == "assistant"
}

// IsSystemMessage checks if message is from system
func (m *Message) IsSystemMessage() bool {
	return m.Role == "system"
}

// HasToolCalls checks if message has tool calls
func (m *Message) HasToolCalls() bool {
	return len(m.ToolCalls) > 0
}

// GetPendingToolCalls returns tool calls with pending status
func (m *Message) GetPendingToolCalls() []ToolCall {
	var pending []ToolCall
	for _, call := range m.ToolCalls {
		if call.Status == "pending" {
			pending = append(pending, call)
		}
	}
	return pending
}

// UpdateToolCallStatus updates the status of a specific tool call
func (m *Message) UpdateToolCallStatus(toolID, status, result, errorMsg string) {
	for i, call := range m.ToolCalls {
		if call.ID == toolID {
			m.ToolCalls[i].Status = status
			if result != "" {
				if m.ToolCalls[i].Result == nil {
					m.ToolCalls[i].Result = make(map[string]interface{})
				}
				// Parse JSON result into map
				var resultData map[string]interface{}
				if err := json.Unmarshal([]byte(result), &resultData); err == nil {
					m.ToolCalls[i].Result = resultData
				}
			}
			if errorMsg != "" {
				m.ToolCalls[i].Error = errorMsg
			}
			break
		}
	}
}
