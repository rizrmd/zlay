package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type RegisterRequest struct {
	ClientID string `json:"client_id"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginRequest struct {
	ClientID string `json:"client_id"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	User      RequestUser `json:"user"`
	Token     string      `json:"token"`
	SessionID string      `json:"session_id"`
}

type User struct {
	ID           string `json:"id"`
	ClientID     string `json:"client_id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	IsActive     bool   `json:"is_active"`
	CreatedAt    string `json:"created_at"`
}

func (app *App) registerHandler(c *gin.Context) {
	ctx := c.Request.Context()

	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}

	// Get client ID from request if not provided
	var clientID uuid.UUID
	if req.ClientID == "" {
		var err error
		clientID, err = app.getClientID(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid client"})
			return
		}
	} else {
		var err error
		clientID, err = uuid.Parse(req.ClientID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid client ID format"})
			return
		}
		// Validate client exists using ZDB
		row, err := app.ZDB.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM clients WHERE id = $1 AND is_active = true)",
			clientID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Database error"})
			return
		}

		exists, ok := row.Values[0].AsBool()
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse result"})
			return
		}

		if !exists {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid client"})
			return
		}
	}

	// Check if user already exists using ZDB
	row, err := app.ZDB.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM users WHERE client_id = $1 AND username = $2)",
		clientID, req.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	exists, ok := row.Values[0].AsBool()
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse result"})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{"error": "User already exists"})
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	// Create user using ZDB
	userID := uuid.New()
	_, err = app.ZDB.Execute(ctx,
		"INSERT INTO users (id, client_id, username, password_hash, is_active, created_at) VALUES ($1, $2, $3, $4, true, CURRENT_TIMESTAMP)",
		userID, clientID, req.Username, string(hashedPassword))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Get created timestamp using ZDB
	row, err = app.ZDB.QueryRow(ctx,
		"SELECT created_at FROM users WHERE id = $1",
		userID)
	if err != nil || len(row.Values) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user details"})
		return
	}

	createdAt, ok := row.Values[0].AsTimestamp()
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse timestamp"})
		return
	}

	user := User{
		ID:       userID.String(),
		ClientID: clientID.String(), Username: req.Username,
		IsActive:  true,
		CreatedAt: createdAt.Format(time.RFC3339),
	}

	response := gin.H{
		"success": true,
		"user": gin.H{
			"id":         user.ID,
			"username":   user.Username,
			"created_at": user.CreatedAt,
		},
		"message": "Registration successful",
	}

	c.JSON(http.StatusCreated, response)
}

func (app *App) loginHandler(c *gin.Context) {
	ctx := c.Request.Context()

	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}

	// Special handling for root user
	isRoot := req.Username == "root"

	// Get client ID
	var clientID uuid.UUID
	if req.ClientID == "" {
		if isRoot {
			// For root user, get default client using ZDB
			row, err := app.ZDB.QueryRow(ctx,
				"SELECT id FROM clients WHERE is_active = true ORDER BY created_at ASC LIMIT 1")
			if err != nil || len(row.Values) == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid client"})
				return
			}

			var ok bool
			clientIDStr, ok := row.Values[0].AsString()
			if !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse client ID"})
				return
			}
			clientID = uuid.MustParse(clientIDStr)
		} else {
			var err error
			clientID, err = app.getClientID(c)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid client"})
				return
			}
		}
	} else if !isRoot {
		var err error
		clientID, err = uuid.Parse(req.ClientID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid client ID format"})
			return
		}
		// Validate client exists for non-root users using ZDB
		row, err := app.ZDB.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM clients WHERE id = $1 AND is_active = true)",
			clientID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Database error"})
			return
		}

		exists, ok := row.Values[0].AsBool()
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse result"})
			return
		}

		if !exists {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid client"})
			return
		}
	}

	// Get user using ZDB
	row, err := app.ZDB.QueryRow(ctx,
		"SELECT id, client_id, username, password_hash, is_active, created_at FROM users WHERE client_id = $1 AND username = $2 AND is_active = true",
		clientID, req.Username)
	if err != nil || len(row.Values) < 6 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	var user User

	// Convert database values to struct fields
	user.ID, _ = row.Values[0].AsString()
	user.ClientID, _ = row.Values[1].AsString()
	user.Username, _ = row.Values[2].AsString()
	user.PasswordHash, _ = row.Values[3].AsString()
	user.IsActive, _ = row.Values[4].AsBool()
	createdAt, _ := row.Values[5].AsTimestamp()
	if createdAt.IsZero() {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse created date"})
		return
	}
	user.CreatedAt = createdAt.Time.Format(time.RFC3339)

	// Verify password
	if bcryptErr := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); bcryptErr != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Generate session token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}
	token := base64.URLEncoding.EncodeToString(tokenBytes)

	// Hash token for storage
	tokenHash := sha256.Sum256([]byte(token))
	tokenHashStr := base64.StdEncoding.EncodeToString(tokenHash[:])

	// Create session using ZDB
	sessionID := uuid.New().String()
	expiresAt := time.Now().Add(24 * time.Hour) // 24 hours
	_, err = app.ZDB.Execute(ctx,
		"INSERT INTO sessions (id, client_id, user_id, token_hash, expires_at, created_at) VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP)",
		sessionID, clientID, user.ID, tokenHashStr, expiresAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
		return
	}

	// Set secure cookie - use domain without port for proxy forwarding
	c.SetCookie("session_token", token, int(24*time.Hour/time.Second), "/", "localhost", false, false)

	response := gin.H{
		"success": true,
		"user": gin.H{
			"id":         user.ID,
			"username":   user.Username,
			"created_at": user.CreatedAt,
		},
		"message": "Login successful",
	}

	c.JSON(http.StatusOK, response)
}

func (app *App) logoutHandler(c *gin.Context) {
	ctx := c.Request.Context()

	// Get session token from cookie
	token, err := c.Cookie("session_token")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No session found"})
		return
	}

	// Hash token to match database
	tokenHash := sha256.Sum256([]byte(token))
	tokenHashStr := base64.StdEncoding.EncodeToString(tokenHash[:])

	// Delete session using ZDB
	_, err = app.ZDB.Execute(ctx, "DELETE FROM sessions WHERE token_hash = $1", tokenHashStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to logout"})
		return
	}

	// Clear cookie
	c.SetCookie("session_token", "", -1, "/", "localhost", false, false)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Logged out successfully"})
}

func (app *App) profileHandler(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User not found in context"})
		return
	}

	u := user.(User)
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"user": gin.H{
			"id":         u.ID,
			"client_id":  u.ClientID,
			"username":   u.Username,
			"is_active":  u.IsActive,
			"created_at": u.CreatedAt,
		},
	})
}

func (app *App) getCurrentUser(c *gin.Context) (*User, error) {
	ctx := c.Request.Context()

	// Get session token from cookie
	token, err := c.Cookie("session_token")
	if err != nil {
		return nil, fmt.Errorf("no session found")
	}

	// Hash token to match database
	tokenHash := sha256.Sum256([]byte(token))
	tokenHashStr := base64.StdEncoding.EncodeToString(tokenHash[:])

	// Get session and user using ZDB
	row, err := app.ZDB.QueryRow(ctx,
		`SELECT u.id, u.client_id, u.username, u.password_hash, u.is_active, u.created_at 
		FROM sessions s 
		JOIN users u ON u.id::text = s.user_id::text
		WHERE s.token_hash = $1::text AND s.expires_at > CURRENT_TIMESTAMP`,
		tokenHashStr)
	if err != nil || len(row.Values) < 6 {
		return nil, fmt.Errorf("invalid or expired session")
	}

	var user User
	var ok bool
	user.ID, ok = row.Values[0].AsString()
	if !ok {
		return nil, fmt.Errorf("failed to parse user ID")
	}
	user.ClientID, ok = row.Values[1].AsString()
	if !ok {
		return nil, fmt.Errorf("failed to parse client ID")
	}
	user.Username, ok = row.Values[2].AsString()
	if !ok {
		return nil, fmt.Errorf("failed to parse username")
	}
	user.PasswordHash, ok = row.Values[3].AsString()
	if !ok {
		return nil, fmt.Errorf("failed to parse password hash")
	}
	user.IsActive, ok = row.Values[4].AsBool()
	if !ok {
		return nil, fmt.Errorf("failed to parse active status")
	}
	createdAt, ok := row.Values[5].AsTimestamp()
	if !ok {
		return nil, fmt.Errorf("failed to parse created date")
	}
	user.CreatedAt = createdAt.Time.Format(time.RFC3339)

	// Check if user is active
	if !user.IsActive {
		return nil, fmt.Errorf("user account is inactive")
	}

	return &user, nil
}

func (app *App) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// Get session token from cookie
		token, err := c.Cookie("session_token")
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "No session found"})
			c.Abort()
			return
		}

		// Hash token to match database
		tokenHash := sha256.Sum256([]byte(token))
		tokenHashStr := base64.StdEncoding.EncodeToString(tokenHash[:])

		// Get session and user using ZDB
		row, err := app.ZDB.QueryRow(ctx,
			`SELECT u.id, u.client_id, u.username, u.password_hash, u.is_active, u.created_at 
			FROM sessions s 
			JOIN users u ON u.id::text = s.user_id::text
			WHERE s.token_hash = $1::text AND s.expires_at > CURRENT_TIMESTAMP`,
			tokenHashStr)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired session"})
			c.Abort()
			return
		}
		if len(row.Values) < 6 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired session"})
			c.Abort()
			return
		}

		var user User
		var ok bool
		user.ID, ok = row.Values[0].AsString()
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse user ID"})
			c.Abort()
			return
		}
		user.ClientID, ok = row.Values[1].AsString()
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse client ID"})
			c.Abort()
			return
		}
		user.Username, ok = row.Values[2].AsString()
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse username"})
			c.Abort()
			return
		}
		user.PasswordHash, ok = row.Values[3].AsString()
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse password hash"})
			c.Abort()
			return
		}
		user.IsActive, ok = row.Values[4].AsBool()
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse active status"})
			c.Abort()
			return
		}
		createdAt, ok := row.Values[5].AsTimestamp()
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse created date"})
			c.Abort()
			return
		}
		user.CreatedAt = createdAt.Time.Format(time.RFC3339)

		// Check if user is active
		if !user.IsActive {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User account is inactive"})
			c.Abort()
			return
		}

		// Set user in context
		c.Set("user", user)
		c.Set("user_id", user.ID)
		c.Set("client_id", user.ClientID)
		c.Set("username", user.Username)

		c.Next()
	}
}

func (app *App) adminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// Get session token from cookie
		token, err := c.Cookie("session_token")
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "No session found"})
			c.Abort()
			return
		}

		// Hash token to match database
		tokenHash := sha256.Sum256([]byte(token))
		tokenHashStr := base64.StdEncoding.EncodeToString(tokenHash[:])

		// Get session and user using ZDB
		row, err := app.ZDB.QueryRow(ctx,
			`SELECT u.id, u.client_id, u.username, u.password_hash, u.is_active, u.created_at 
			FROM sessions s 
			JOIN users u ON u.id::text = s.user_id::text
			WHERE s.token_hash = $1::text AND s.expires_at > CURRENT_TIMESTAMP`,
			tokenHashStr)
		if err != nil || len(row.Values) < 6 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired session"})
			c.Abort()
			return
		}

		username, _ := row.Values[2].AsString()
		isActive, _ := row.Values[4].AsBool()

		// Check if user is root and active
		if !isActive || username != "root" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// Conversation structs for API
type Conversation struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	UserID    string `json:"user_id"`
	ProjectID string `json:"project_id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func (app *App) getConversationsHandler(c *gin.Context) {
	ctx := c.Request.Context()
	
	// Get user ID from auth middleware
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	
	// Get project ID from current session (this is a simplification - 
	// in production you might want to filter by project_id from query params)
	projectID := "d3eb9ece-48e7-45d0-a281-6b780351dedd" // Default project for now
	
	// Query conversations using ZDB
	resultSet, err := app.ZDB.Query(ctx, `
		SELECT id, title, user_id, project_id, created_at, updated_at 
		FROM conversations 
		WHERE user_id = $1 AND project_id = $2 
		ORDER BY updated_at DESC
	`, userID, projectID)
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to query conversations",
			"details": err.Error(),
		})
		return
	}
	
	conversations := []Conversation{}
	for _, row := range resultSet.Rows {
		conv := Conversation{}
		// Map row values to struct
		if len(row.Values) >= 6 {
			conv.ID, _ = row.Values[0].AsString()
			conv.Title, _ = row.Values[1].AsString()
			conv.UserID, _ = row.Values[2].AsString()
			conv.ProjectID, _ = row.Values[3].AsString()
			conv.CreatedAt, _ = row.Values[4].AsString()
			conv.UpdatedAt, _ = row.Values[5].AsString()
		}
		conversations = append(conversations, conv)
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"conversations": conversations,
	})
}

// Message struct for API
type Message struct {
	ID        string                 `json:"id"`
	ConversationID string              `json:"conversation_id"`
	Role      string                 `json:"role"`
	Content   string                 `json:"content"`
	Metadata  map[string]interface{}  `json:"metadata,omitempty"`
	ToolCalls []ToolCall             `json:"tool_calls,omitempty"`
	CreatedAt string                 `json:"created_at"`
}

type ToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function ToolCallFunction      `json:"function"`
	Status   string                 `json:"status,omitempty"`
	Result   interface{}            `json:"result,omitempty"`
	Error    string                 `json:"error,omitempty"`
}

type ToolCallFunction struct {
	Name      string      `json:"name"`
	Arguments interface{} `json:"arguments"`
}

func (app *App) getConversationMessagesHandler(c *gin.Context) {
	ctx := c.Request.Context()
	conversationID := c.Param("id")
	
	// Get user ID from auth middleware
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	
	// Validate conversation belongs to user
	convResult, err := app.ZDB.QueryRow(ctx, `
		SELECT id FROM conversations 
		WHERE id = $1 AND user_id = $2
	`, conversationID, userID)
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to validate conversation",
			"details": err.Error(),
		})
		return
	}
	
	if convResult.Values == nil || len(convResult.Values) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Conversation not found"})
		return
	}
	
	// Query messages for this conversation
	resultSet, err := app.ZDB.Query(ctx, `
		SELECT id, conversation_id, role, content, metadata, tool_calls, created_at 
		FROM messages 
		WHERE conversation_id = $1 
		ORDER BY created_at ASC
	`, conversationID)
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to query messages",
			"details": err.Error(),
		})
		return
	}
	
	messages := []Message{}
	for _, row := range resultSet.Rows {
		msg := Message{}
		if len(row.Values) >= 6 {
			msg.ID, _ = row.Values[0].AsString()
			msg.ConversationID, _ = row.Values[1].AsString()
			msg.Role, _ = row.Values[2].AsString()
			msg.Content, _ = row.Values[3].AsString()
			
			// Parse metadata JSON
			metadataStr, _ := row.Values[4].AsString()
			if metadataStr != "" {
				if err := json.Unmarshal([]byte(metadataStr), &msg.Metadata); err != nil {
					msg.Metadata = make(map[string]interface{})
				}
			}
			
			// Parse tool_calls JSON
			toolCallsStr, _ := row.Values[5].AsString()
			if toolCallsStr != "" {
				if err := json.Unmarshal([]byte(toolCallsStr), &msg.ToolCalls); err != nil {
					msg.ToolCalls = []ToolCall{}
				}
			}
			
			msg.CreatedAt, _ = row.Values[6].AsString()
		}
		messages = append(messages, msg)
	}
	
	// Also get conversation details
	convResultSet, err := app.ZDB.Query(ctx, `
		SELECT id, title, user_id, project_id, created_at, updated_at 
		FROM conversations 
		WHERE id = $1
	`, conversationID)
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get conversation details",
			"details": err.Error(),
		})
		return
	}
	
	var conversation Conversation
	if len(convResultSet.Rows) > 0 {
		convRow := convResultSet.Rows[0]
		if len(convRow.Values) >= 6 {
			conversation.ID, _ = convRow.Values[0].AsString()
			conversation.Title, _ = convRow.Values[1].AsString()
			conversation.UserID, _ = convRow.Values[2].AsString()
			conversation.ProjectID, _ = convRow.Values[3].AsString()
			conversation.CreatedAt, _ = convRow.Values[4].AsString()
			conversation.UpdatedAt, _ = convRow.Values[5].AsString()
		}
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"conversation": map[string]interface{}{
			"conversation": conversation,
			"messages": messages,
		},
	})
}
