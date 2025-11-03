package websocket

import (
	"bytes"
	"compress/flate"
	"encoding/json"
	"log"
	"sync"
	"time"
	
	"zlay-backend/internal/chat"
	"zlay-backend/internal/messages"
)

// WebSocketMessage represents a message sent over WebSocket (alias for shared package)
type WebSocketMessage = messages.WebSocketMessage

// UserMessageData represents data for user_message type
type UserMessageData struct {
	ConversationID string `json:"conversation_id"`
	Content        string `json:"content"`
	ConnectionID   string `json:"connection_id,omitempty"`
	UserID         string `json:"user_id,omitempty"`
	ProjectID      string `json:"project_id,omitempty"`
	ClientID       string `json:"client_id,omitempty"`
}

// AssistantResponseData represents data for assistant_response type
type AssistantResponseData struct {
	ConversationID string      `json:"conversation_id"`
	Content        string      `json:"content"`
	MessageID      string      `json:"message_id"`
	Timestamp      string      `json:"timestamp"`
	Done           bool        `json:"done"`
	ToolCalls      []ToolCall  `json:"tool_calls,omitempty"`
}

// ToolCall represents a tool call in the assistant response
type ToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function ToolCallFunction       `json:"function"`
	Status   string                 `json:"status,omitempty"`
	Result   interface{}            `json:"result,omitempty"`
	Error    string                 `json:"error,omitempty"`
}

// ToolCallFunction represents the function part of a tool call
type ToolCallFunction struct {
	Name      string      `json:"name"`
	Arguments interface{} `json:"arguments"`
}

// ProjectJoinedData represents data for project_joined type
type ProjectJoinedData struct {
	ProjectID string `json:"project_id"`
	Success   bool   `json:"success"`
}

// ConversationCreatedData represents data for conversation_created type
type ConversationCreatedData struct {
	Conversation Conversation `json:"conversation"`
	Success      bool         `json:"success"`
}

// ConversationsListData represents data for conversations_list type
type ConversationsListData struct {
	Conversations []Conversation `json:"conversations"`
}

// ConversationDetailsData represents data for conversation_details type
type ConversationDetailsData struct {
	Conversation ConversationWithMessages `json:"conversation"`
}

// ConversationWithMessages represents a conversation with its messages
type ConversationWithMessages struct {
	Conversation
	Messages []Message `json:"messages"`
}

// Conversation represents a conversation
type Conversation struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	UserID    string    `json:"user_id"`
	ProjectID string    `json:"project_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Message represents a chat message
type Message struct {
	ID        string                 `json:"id"`
	Role      string                 `json:"role"`
	Content   string                 `json:"content"`
	CreatedAt time.Time              `json:"created_at"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	ToolCalls []ToolCall             `json:"tool_calls,omitempty"`
}

// ErrorData represents data for error type
type ErrorData struct {
	Error   string                 `json:"error"`
	Code    string                 `json:"code,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// PongData represents data for pong type
type PongData struct {
	Timestamp int64 `json:"timestamp"`
}

// Tool execution data types

// ToolExecutionStartedData represents data for tool_execution_started type
type ToolExecutionStartedData struct {
	ToolName       string `json:"tool_name"`
	ToolCallID     string `json:"tool_call_id"`
	ConversationID string `json:"conversation_id"`
	MessageID      string `json:"message_id,omitempty"`
}

// ToolExecutionCompletedData represents data for tool_execution_completed type
type ToolExecutionCompletedData struct {
	ToolName         string      `json:"tool_name"`
	ToolCallID       string      `json:"tool_call_id"`
	ConversationID   string      `json:"conversation_id"`
	Success          bool        `json:"success"`
	Result           interface{} `json:"result"`
	ExecutionTimeMs  int         `json:"execution_time_ms,omitempty"`
}

// ToolExecutionFailedData represents data for tool_execution_failed type
type ToolExecutionFailedData struct {
	ToolName       string `json:"tool_name"`
	ToolCallID     string `json:"tool_call_id"`
	ConversationID string `json:"conversation_id"`
	Error          string `json:"error"`
	ErrorCode      string `json:"error_code,omitempty"`
}

// Hub maintains the set of active connections and broadcasts messages to them
type Hub struct {
	// Registered connections
	connections map[*Connection]bool

	// Project-based rooms for isolation
	projects map[string]map[*Connection]bool

	// Inbound messages from the connections
	broadcast chan []byte

	// Register requests from the connections
	register chan *Connection

	// Unregister requests from connections
	unregister chan *Connection

	// Project join/leave requests
	projectJoin  chan *ProjectJoin
	projectLeave chan *ProjectLeave

	// Chat service handler reference
	handler interface{}
	// Mutex for thread-safe operations
	mutex sync.RWMutex
}

// ProjectJoin represents a connection joining a project room
type ProjectJoin struct {
	Connection *Connection
	ProjectID  string
}

// ProjectLeave represents a connection leaving a project room
type ProjectLeave struct {
	Connection *Connection
	ProjectID  string
}

// NewHub creates a new hub instance
func NewHub() *Hub {
	return &Hub{
		connections:  make(map[*Connection]bool),
		projects:     make(map[string]map[*Connection]bool),
		broadcast:    make(chan []byte),
		register:     make(chan *Connection),
		unregister:   make(chan *Connection),
		projectJoin:  make(chan *ProjectJoin),
		projectLeave: make(chan *ProjectLeave),
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	for {
		select {
		case conn := <-h.register:
			h.mutex.Lock()
			h.connections[conn] = true
			h.mutex.Unlock()
			log.Printf("Connection registered: %s", conn.ID)

		case conn := <-h.unregister:
			// Check if connection is already unregistered to prevent double-unregister
			if conn.isUnregistered() {
				continue
			}
			
			h.mutex.Lock()
			if _, ok := h.connections[conn]; ok {
				delete(h.connections, conn)

				// Remove from all project rooms
				for projectID, conns := range h.projects {
					if _, inRoom := conns[conn]; inRoom {
						delete(conns, conn)
						if len(conns) == 0 {
							delete(h.projects, projectID)
						}
					}
				}

				// Mark as unregistered and close send channel safely
				conn.shouldUnregister()
				conn.closeSendChannel()
				log.Printf("Connection unregistered: %s", conn.ID)
			}
			h.mutex.Unlock()

		case join := <-h.projectJoin:
			h.mutex.Lock()
			if h.projects[join.ProjectID] == nil {
				h.projects[join.ProjectID] = make(map[*Connection]bool)
			}
			h.projects[join.ProjectID][join.Connection] = true
			h.mutex.Unlock()
			log.Printf("Connection %s joined project %s", join.Connection.ID, join.ProjectID)

		case leave := <-h.projectLeave:
			h.mutex.Lock()
			if conns, exists := h.projects[leave.ProjectID]; exists {
				delete(conns, leave.Connection)
				if len(conns) == 0 {
					delete(h.projects, leave.ProjectID)
				}
			}
			h.mutex.Unlock()
			log.Printf("Connection %s left project %s", leave.Connection.ID, leave.ProjectID)

		case message := <-h.broadcast:
			h.mutex.RLock()
			for conn := range h.connections {
				select {
				case conn.send <- message:
				default:
					// Connection send buffer is full, skip this connection
					conn.closeSendChannel()
					delete(h.connections, conn)
				}
			}
			h.mutex.RUnlock()
		}
	}
}

// BroadcastToProject sends a message to all connections in a project room
func (h *Hub) BroadcastToProject(projectID string, message interface{}) {
	data, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}

	// Send uncompressed data - WebSocket compression is handled by upgrader
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	if conns, exists := h.projects[projectID]; exists {
		for conn := range conns {
			select {
			case conn.send <- data:
			default:
				// Connection send buffer is full
				conn.closeSendChannel()
				delete(conns, conn)
				delete(h.connections, conn)
			}
		}
	}
}

// SendToConnection sends a message to a specific connection
func (h *Hub) SendToConnection(conn *Connection, message interface{}) {
	data, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}

	// Send uncompressed data - WebSocket compression is handled by the upgrader
	select {
	case conn.send <- data:
	default:
		// Connection send buffer is full
		conn.closeSendChannel()
		h.mutex.Lock()
		delete(h.connections, conn)
		h.mutex.Unlock()
		log.Printf("Connection %s removed due to full send buffer", conn.ID)
	}
}

// GetProjectConnectionCount returns the number of connections in a project room
func (h *Hub) GetProjectConnectionCount(projectID string) int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	if conns, exists := h.projects[projectID]; exists {
		return len(conns)
	}
	return 0
}

// GetConnectionCount returns the total number of active connections
func (h *Hub) GetConnectionCount() int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	return len(h.connections)
}

// GetConnections returns a copy of all active connections
func (h *Hub) GetConnections() map[*Connection]bool {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	// Return a copy to avoid race conditions
	connections := make(map[*Connection]bool)
	for conn := range h.connections {
		connections[conn] = true
	}
	return connections
}

// Convert chat conversation to websocket conversation
func convertConversation(conv *chat.Conversation) Conversation {
	return Conversation{
		ID:        conv.ID,
		Title:     conv.Title,
		UserID:    conv.UserID,
		ProjectID: conv.ProjectID,
		CreatedAt: conv.CreatedAt,
		UpdatedAt: conv.UpdatedAt,
	}
}

// Convert chat conversations to websocket conversations
func convertConversations(convs []*chat.Conversation) []Conversation {
	result := make([]Conversation, len(convs))
	for i, conv := range convs {
		result[i] = convertConversation(conv)
	}
	return result
}

// Convert chat message to websocket message
func convertMessage(msg *chat.Message) Message {
	return Message{
		ID:        msg.ID,
		Role:      msg.Role,
		Content:   msg.Content,
		CreatedAt: msg.CreatedAt,
		Metadata:  msg.Metadata,
		ToolCalls: convertToolCalls(msg.ToolCalls),
	}
}

// Convert chat tool calls to websocket tool calls
func convertToolCalls(toolCalls []chat.ToolCall) []ToolCall {
	result := make([]ToolCall, len(toolCalls))
	for i, tc := range toolCalls {
		result[i] = ToolCall{
			ID:     tc.ID,
			Type:   tc.Type,
			Function: ToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
			Status: tc.Status,
			Result: tc.Result,
			Error:  tc.Error,
		}
	}
	return result
}

// Convert chat conversation details to websocket format
func convertConversationDetails(details *chat.ConversationDetails) ConversationWithMessages {
	return ConversationWithMessages{
		Conversation: convertConversation(details.Conversation),
		Messages: func() []Message {
			result := make([]Message, len(details.Messages))
			for i, msg := range details.Messages {
				result[i] = convertMessage(msg)
			}
			return result
		}(),
	}
}

// compressMessage compresses JSON data for WebSocket transmission
func compressMessage(data interface{}) ([]byte, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	writer, err := flate.NewWriter(&buf, flate.BestCompression)
	if err != nil {
		return nil, err
	}

	if _, err := writer.Write(jsonData); err != nil {
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// shouldCompressMessage determines if a message should be compressed
func shouldCompressMessage(data []byte) bool {
	// Compress messages larger than 1KB
	return len(data) > 1024
}

// GetProjectConnections returns a copy of connections in a project room
func (h *Hub) GetProjectConnections(projectID string) []*Connection {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	var connections []*Connection
	if conns, exists := h.projects[projectID]; exists {
		for conn := range conns {
			connections = append(connections, conn)
		}
	}
	return connections
}
