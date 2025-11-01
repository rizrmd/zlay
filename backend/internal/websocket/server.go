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
	hub         *Hub
	chatService chat.ChatService
	router      *gin.Engine
	db          *db.Database
	port        string
}

// NewServer creates a new WebSocket server
func NewServer(zdb *db.Database, port string) *Server {
	// Create hub
	hub := NewHub()

	// Initialize LLM client
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		apiKey = "sk-no-key-required" // Default for local development
	}
	baseURL := os.Getenv("OPENAI_BASE_URL")
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-3.5-turbo"
	}

	llmClient := llm.NewOpenAIClient(apiKey, baseURL, model)

	// Initialize tool registry with built-in tools
	toolRegistry := tools.NewDefaultToolRegistry()

	// Create chat service with zlay-db adapter
	chatService := chat.NewPostgreSQLChatService(
		&tools.ZlayDBAdapter{DB: zdb},
		&tools.WebSocketAdapter{Hub: hub},
		llmClient,
		toolRegistry,
	)

	server := &Server{
		hub:         hub,
		chatService: chatService,
		db:          db,
		port:        port,
	}

	return server
}

// Start starts the WebSocket server
func (s *Server) Start() error {
	log.Printf("Starting WebSocket server on port %s", s.port)

	// Start hub in separate goroutine
	go s.hub.Run()

	// Setup routes
	s.setupRoutes()

	// Start server
	log.Printf("WebSocket server listening on %s", s.port)
	return s.router.Run(":" + s.port)
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

	// Create handler with chat service
	handler := &Handler{
		hub:         s.hub,
		chatService: s.chatService,
	}

	// WebSocket endpoint
	s.router.GET("/ws/chat", handler.HandleWebSocket)

	// Health check endpoint
	s.router.GET("/ws/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":      "healthy",
			"timestamp":   time.Now().Unix(),
			"connections": s.hub.GetConnectionCount(),
		})
	})

	// Stats endpoint
	s.router.GET("/ws/stats", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"active_connections": s.hub.GetConnectionCount(),
			"timestamp":          time.Now().Unix(),
		})
	})
}
