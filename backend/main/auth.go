package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
	"github.com/google/uuid"
	"github.com/gin-gonic/gin"
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
	clientID := req.ClientID
	if clientID == "" {
		var err error
		clientID, err = app.getClientID(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid client"})
			return
		}
	} else {
		// Validate client exists using ZDB
		row, err := app.ZDB.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM clients WHERE id = $1 AND is_active = true)",
			clientID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Database error"})
			return
		}
		
		var exists bool
		if err := row.Values[0].GetBool(&exists); err != nil {
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
	
	var exists bool
	if err := row.Values[0].GetBool(&exists); err != nil {
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
	userID := uuid.New().String()
	_, err = app.ZDB.Execute(ctx,
		"INSERT INTO users (id, client_id, username, password_hash, is_active, created_at) VALUES ($1, $2, $3, $4, true, CURRENT_TIMESTAMP)",
		userID, clientID, req.Username, string(hashedPassword))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}
	
	// Get created timestamp using ZDB
	row, err := app.ZDB.QueryRow(ctx,
		"SELECT created_at FROM users WHERE id = $1",
		userID)
	if err != nil || len(row.Values) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user details"})
		return
	}
	
	createdAt, err := row.Values[0].GetTime()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse timestamp"})
		return
	}

	user := User{
		ID:        userID,
		ClientID:  clientID,
		Username:  req.Username,
		IsActive:  true,
		CreatedAt: createdAt.Format(time.RFC3339),
	}

	c.JSON(http.StatusCreated, user)
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
	clientID := req.ClientID
	if clientID == "" {
		if isRoot {
			// For root user, get default client using ZDB
			row, err := app.ZDB.QueryRow(ctx,
				"SELECT id FROM clients WHERE is_active = true ORDER BY created_at ASC LIMIT 1")
			if err != nil || len(row.Values) == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid client"})
				return
			}
			
			clientID, err := row.Values[0].GetString()
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse client ID"})
				return
			}
		} else {
			var err error
			clientID, err = app.getClientID(c)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid client"})
				return
			}
		}
	} else if !isRoot {
		// Validate client exists for non-root users using ZDB
		row, err := app.ZDB.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM clients WHERE id = $1 AND is_active = true)",
			clientID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Database error"})
			return
		}
		
		var exists bool
		if err := row.Values[0].GetBool(&exists); err != nil {
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
	if err := row.Values[0].GetString(&user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse user ID"})
		return
	}
	if err := row.Values[1].GetString(&user.ClientID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse client ID"})
		return
	}
	if err := row.Values[2].GetString(&user.Username); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse username"})
		return
	}
	if err := row.Values[3].GetString(&user.PasswordHash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse password hash"})
		return
	}
	if err := row.Values[4].GetBool(&user.IsActive); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse active status"})
		return
	}
	if err := row.Values[5].GetTime(&user.CreatedAt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse created date"})
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
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

	// Set secure cookie
	c.SetCookie("session_token", token, int(24*time.Hour/time.Second), "/", "", false, true)

	response := LoginResponse{
		User: RequestUser{
			ID:       user.ID,
			ClientID: user.ClientID,
			Username: user.Username,
		},
		Token:     token,
		SessionID: sessionID,
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
	c.SetCookie("session_token", "", -1, "/", "", false, true)

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

func (app *App) profileHandler(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	c.JSON(http.StatusOK, user)
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
			JOIN users u ON s.user_id = u.id 
			WHERE s.token_hash = $1 AND s.expires_at > CURRENT_TIMESTAMP`,
			tokenHashStr)
		if err != nil || len(row.Values) < 6 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired session"})
			c.Abort()
			return
		}
		
		var user User
		if err := row.Values[0].GetString(&user.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse user ID"})
			c.Abort()
			return
		}
		if err := row.Values[1].GetString(&user.ClientID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse client ID"})
			c.Abort()
			return
		}
		if err := row.Values[2].GetString(&user.Username); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse username"})
			c.Abort()
			return
		}
		if err := row.Values[3].GetString(&user.PasswordHash); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse password hash"})
			c.Abort()
			return
		}
		if err := row.Values[4].GetBool(&user.IsActive); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse active status"})
			c.Abort()
			return
		}
		if err := row.Values[5].GetTime(&user.CreatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse created date"})
			c.Abort()
			return
		}

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
		user, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
			c.Abort()
			return
		}

		// Check if user is root
		u, ok := user.(User)
		if !ok || u.Username != "root" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			c.Abort()
			return
		}

		c.Next()
	}
}
