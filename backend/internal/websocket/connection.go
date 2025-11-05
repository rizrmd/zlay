package websocket

import (
	"encoding/json"
	"log"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Connection represents a WebSocket connection
type Connection struct {
	// WebSocket connection
	ws *websocket.Conn

	// Buffered channel of outbound messages
	send chan []byte

	// Connection metadata
	ID        string
	UserID    string
	ClientID  string
	ProjectID string

	// Token usage tracking
	TokensUsed int64
	TokensLimit int64

	// Hub reference for broadcasting
	hub *Hub

	// Reference to the WebSocket handler for routing messages
	handler *Handler
	
	// Track if send channel is closed to prevent double-close
	closed int32 // 0 = open, 1 = closed
	
	// Track if connection is unregistered to prevent double-unregister
	unregistered int32 // 0 = not unregistered, 1 = unregistered
}

// NewConnection creates a new connection instance
func NewConnection(ws *websocket.Conn, userID, clientID string, hub *Hub) *Connection {
	return &Connection{
		ws:          ws,
		send:        make(chan []byte, 256),
		ID:          uuid.New().String(),
		UserID:      userID,
		ClientID:    clientID,
		hub:         hub,
		TokensUsed:  0,
		TokensLimit: 1000000, // Default limit of 1M tokens per connection
		handler:     nil,
	}
}

// ReadPump pumps messages from the WebSocket connection to the hub
func (c *Connection) ReadPump() {
	defer func() {
		// 🔄 NEW: Check for active streaming and mark as interrupted
		c.hub.handleInterruptionForConnection(c)
		
		c.hub.unregister <- c
		// Close the outbound message channel to stop the WritePump
		c.closeSendChannel()
		c.ws.Close()
	}()

	// Set read deadline
	c.ws.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.ws.SetPongHandler(func(string) error {
		c.ws.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		// Read message
		_, messageData, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Parse and handle message
		var message WebSocketMessage
		if err := json.Unmarshal(messageData, &message); err != nil {
			log.Printf("Error unmarshaling message: %v", err)
			continue
		}

		// Add connection metadata to message
		message.Timestamp = time.Now().UnixMilli()

	// Route message based on type
		switch message.Type {
		case "user_message":
			if c.handler != nil {
				c.handler.handleUserMessage(c, &message)
			}
		case "join_project":
			c.handleProjectJoin(message)
		case "leave_project":
			c.handleProjectLeave(message)
		case "ping":
			c.handlePing()
		// New chat-related message types routed to handler methods
		case "get_conversations":
			if c.handler != nil {
				c.handler.handleGetConversations(c, &message)
			}
		case "create_conversation":
			if c.handler != nil {
				c.handler.handleCreateConversation(c, &message)
			}
		case "get_conversation":
			if c.handler != nil {
				c.handler.handleGetConversation(c, &message)
			}
		case "delete_conversation":
			if c.handler != nil {
				c.handler.handleDeleteConversation(c, &message)
			}
		case "get_streaming_conversation":
			if c.handler != nil {
				// c.handleGetStreamingConversation(conn, message)
				c.handler.handleGetStreamingConversation(c, &message)
			}
		default:
			// For unhandled message types, just log but don't error
			log.Printf("Received message type: %s (no handler yet)", message.Type)
		}

		// Reset read deadline
		c.ws.SetReadDeadline(time.Now().Add(60 * time.Second))
	}
}

// WritePump pumps messages from the hub to the WebSocket connection
func (c *Connection) WritePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.ws.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// Hub closed the channel
				c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.ws.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)
			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// JoinProject adds the connection to a project room
func (c *Connection) JoinProject(projectID string) {
	// Leave current project if any
	if c.ProjectID != "" {
		c.hub.projectLeave <- &ProjectLeave{
			Connection: c,
			ProjectID:  c.ProjectID,
		}
	}

	c.ProjectID = projectID
	c.hub.projectJoin <- &ProjectJoin{
		Connection: c,
		ProjectID:  projectID,
	}

	// Send confirmation matching AsyncAPI spec
	c.hub.SendToConnection(c, WebSocketMessage{
		Type: "project_joined",
		Data: ProjectJoinedData{
			ProjectID: projectID,
			Success:   true,
		},
		Timestamp: time.Now().UnixMilli(),
	})
}

// LeaveProject removes the connection from a project room
func (c *Connection) LeaveProject() {
	if c.ProjectID != "" {
		c.hub.projectLeave <- &ProjectLeave{
			Connection: c,
			ProjectID:  c.ProjectID,
		}

		// Send confirmation
		c.hub.SendToConnection(c, WebSocketMessage{
			Type: "project_left",
			Data: gin.H{
				"project_id": c.ProjectID,
				"success":    true,
			},
		})

		c.ProjectID = ""
	}
}

// handleProjectJoin processes project join requests
func (c *Connection) handleProjectJoin(message WebSocketMessage) {
	data, ok := message.Data.(map[string]interface{})
	if !ok {
		log.Printf("Invalid join_project data format")
		return
	}

	projectID, ok := data["project_id"].(string)
	if !ok {
		log.Printf("Missing project_id in join_project message")
		return
	}

	c.JoinProject(projectID)
	
	// Send project joined confirmation via hub
	// Note: This will be handled by the WebSocket handler
}

// handleProjectLeave processes project leave requests
func (c *Connection) handleProjectLeave(message WebSocketMessage) {
	data, ok := message.Data.(map[string]interface{})
	if !ok {
		log.Printf("Invalid leave_project data format")
		return
	}

	projectID, ok := data["project_id"].(string)
	if ok && projectID == c.ProjectID {
		c.LeaveProject()
	}
}

// AddTokens adds to the token usage count and returns true if within limit
func (c *Connection) AddTokens(tokens int64) bool {
	c.TokensUsed += tokens
	return c.TokensUsed <= c.TokensLimit
}

// GetTokenUsage returns current token usage statistics
func (c *Connection) GetTokenUsage() (used int64, limit int64, remaining int64) {
	return c.TokensUsed, c.TokensLimit, c.TokensLimit - c.TokensUsed
}

// IsTokenLimitExceeded checks if token limit has been exceeded
func (c *Connection) IsTokenLimitExceeded() bool {
	return c.TokensUsed > c.TokensLimit
}

// SetTokenLimit updates the token limit for this connection
func (c *Connection) SetTokenLimit(limit int64) {
	c.TokensLimit = limit
}

// ResetTokenUsage resets the token usage counter
func (c *Connection) ResetTokenUsage() {
	c.TokensUsed = 0
}

// handlePing processes ping messages
func (c *Connection) handlePing() {
	c.hub.SendToConnection(c, WebSocketMessage{
		Type:      "pong",
		Timestamp: time.Now().UnixMilli(),
		Data: PongData{
			Timestamp: time.Now().UnixMilli(),
		},
	})
}

// closeSendChannel safely closes the send channel if not already closed
func (c *Connection) closeSendChannel() {
	if atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		close(c.send)
	}
}

// shouldUnregister checks if this connection should be unregistered
func (c *Connection) shouldUnregister() bool {
	return atomic.CompareAndSwapInt32(&c.unregistered, 0, 1)
}

// isUnregistered checks if this connection is already unregistered
func (c *Connection) isUnregistered() bool {
	return atomic.LoadInt32(&c.unregistered) == 1
}
