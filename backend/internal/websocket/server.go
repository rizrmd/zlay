package websocket

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"zlay-backend/internal/chat"
	"zlay-backend/internal/db"
	"zlay-backend/internal/llm"
	"zlay-backend/internal/tools"
)

// Server handles WebSocket server
type Server struct {
	hub              *Hub
	chatService       chat.ChatService
	router            *gin.Engine
	db                *db.Database
	port              string
	clientConfigCache *ClientConfigCache
}

// NewServer creates a new WebSocket server
func NewServer(zdb *db.Database, port string) *Server {
	// Create hub
	hub := NewHub()

	// Create client configuration cache
	clientConfigCache := NewClientConfigCache(zdb)
	
	// Initialize default LLM client for fallback
	defaultAPIKey := os.Getenv("OPENAI_API_KEY")
	if defaultAPIKey == "" {
		defaultAPIKey = "sk-no-key-required"
	}
	defaultBaseURL := os.Getenv("OPENAI_BASE_URL")
	defaultModel := os.Getenv("OPENAI_MODEL")
	if defaultModel == "" {
		defaultModel = "gpt-3.5-turbo"
	}
	defaultLLMClient := llm.NewOpenAIClient(defaultAPIKey, defaultBaseURL, defaultModel)

	// Initialize tool registry with built-in tools
	toolRegistry := tools.NewDefaultToolRegistry()

	// Create chat service with default LLM (will be replaced per-client)
	chatService := chat.NewChatService(
		&tools.ZlayDBAdapter{DB: zdb},
		&tools.WebSocketAdapter{Hub: hub},
		defaultLLMClient,
		toolRegistry,
	)

	server := &Server{
		hub:              hub,
		chatService:       chatService,
		db:                zdb,
		port:              port,
		clientConfigCache: clientConfigCache,
	}

	// Start cache cleanup routine
	clientConfigCache.StartCleanupRoutine()

	return server
}

// Start starts the WebSocket server
func (s *Server) Start() error {
	log.Printf("WebSocket server starting on port %s", s.port)

	// Start hub in separate goroutine
	go s.hub.Run()

	// Setup routes
	s.setupRoutes()

	// Start server
	addr := ":" + s.port
	log.Printf("WebSocket router listening on %s", addr)
	log.Printf("WebSocket server attempting to bind to address: %s", addr)
	err := s.router.Run(addr)
	if err != nil {
		log.Printf("WebSocket server failed to start: %v", err)
	}
	return err
}

// Stop gracefully stops the WebSocket server
func (s *Server) Stop() error {
	log.Printf("Stopping WebSocket server...")

	// TODO: Implement graceful shutdown logic
	// For now, just log
	log.Printf("WebSocket server stopped")
	return nil
}

// setupRoutes configures WebSocket routes
func (s *Server) setupRoutes() {
	// Create router
	s.router = gin.Default()

	// Enable CORS
	s.router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")
		c.Header("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	})

	// Create handler with chat service and client config cache
	handler := &Handler{
		hub:              s.hub,
		chatService:       s.chatService,
		db:                s.db,
		clientConfigCache: s.clientConfigCache,
	}

	// WebSocket endpoint
	s.router.GET("/ws/chat", handler.HandleWebSocket)

	// Health check endpoint
	s.router.GET("/ws/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":      "healthy",
			"timestamp":   time.Now().Unix(),
			"connections": s.hub.GetConnectionCount(),
			"port":        s.port,
		})
	})

	// Stats endpoint
	s.router.GET("/ws/stats", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"active_connections": s.hub.GetConnectionCount(),
			"timestamp":          time.Now().Unix(),
			"client_config_cache": s.clientConfigCache.GetCacheStats(),
		})
	})

	// Client config cache management endpoint
	s.router.POST("/ws/invalidate-client/:client_id", func(c *gin.Context) {
		clientID := c.Param("client_id")
		s.clientConfigCache.InvalidateClientConfig(clientID)
		c.JSON(200, gin.H{"message": "Client cache invalidated", "client_id": clientID})
	})

	// Token usage management endpoint
	wsAdmin := s.router.Group("/ws/admin")
	{
		wsAdmin.GET("/connections/:connection_id/tokens", func(c *gin.Context) {
			connectionID := c.Param("connection_id")
			
			// Find connection in hub
			for conn := range s.hub.GetConnections() {
				if conn.ID == connectionID {
					used, limit, remaining := conn.GetTokenUsage()
					c.JSON(200, gin.H{
						"connection_id": connectionID,
						"tokens_used":   used,
						"tokens_limit":  limit,
						"tokens_remaining": remaining,
						"exceeded":      conn.IsTokenLimitExceeded(),
					})
					return
				}
			}
			
			c.JSON(404, gin.H{"error": "Connection not found"})
		})

		wsAdmin.PUT("/connections/:connection_id/tokens/limit", func(c *gin.Context) {
			connectionID := c.Param("connection_id")
			
			var req struct {
				Limit int64 `json:"limit"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(400, gin.H{"error": "Invalid JSON"})
				return
			}

			// Find connection in hub
			for conn := range s.hub.GetConnections() {
				if conn.ID == connectionID {
					conn.SetTokenLimit(req.Limit)
					c.JSON(200, gin.H{
						"connection_id": connectionID,
						"tokens_limit":  req.Limit,
						"message":       "Token limit updated",
					})
					return
				}
			}
			
			c.JSON(404, gin.H{"error": "Connection not found"})
		})

		wsAdmin.POST("/connections/:connection_id/tokens/reset", func(c *gin.Context) {
			connectionID := c.Param("connection_id")
			
			// Find connection in hub
			for conn := range s.hub.GetConnections() {
				if conn.ID == connectionID {
					previous := conn.TokensUsed
					conn.ResetTokenUsage()
					c.JSON(200, gin.H{
						"connection_id": connectionID,
						"previous_tokens": previous,
						"tokens_used": 0,
						"message":       "Token usage reset",
					})
					return
				}
			}
			
			c.JSON(404, gin.H{"error": "Connection not found"})
		})
	}
}
