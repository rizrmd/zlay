package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/openai/openai-go"
	"zlay-backend/internal/db"
	"zlay-backend/internal/llm"
	"zlay-backend/internal/websocket"
)

type Config struct {
	DatabaseURL string
	Port        string
	WSPort      string
}

type App struct {
	Config             *Config
	ZDB                *db.Database // Zlay-db abstraction - SINGLE source of truth for database operations
	Router             *gin.Engine
	WSServer           *websocket.Server
	DomainCache        map[string]uuid.UUID // Cache for domain -> client_id mapping
	ClientConfigCache  *websocket.ClientConfigCache
}

type RequestUser struct {
	ID       string `json:"id"`
	ClientID string `json:"client_id"`
	Username string `json:"username"`
}

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	config := &Config{
		DatabaseURL: getEnv("DATABASE_URL", "postgresql://postgres:password@localhost:5432/zlay"),
		Port:        getEnv("PORT", "8080"), // Backend runs on 8080
		// Add WebSocket port
		WSPort: getEnv("WS_PORT", "6070"),
	}

	app := &App{
		Config: config,
	}

	// Initialize zlay-db abstraction (SINGLE database connection)
	if err := app.InitZDB(); err != nil {
		log.Fatalf("Failed to initialize zlay-db: %v", err)
	}
	defer app.ZDB.Close()

	// Initialize router
	app.InitRouter()

	// Initialize client config cache
	app.ClientConfigCache = websocket.NewClientConfigCache(app.ZDB)

	// Start WebSocket server in separate goroutine
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("WebSocket server panic recovered: %v", r)
			}
		}()
		log.Printf("Starting WebSocket server on port %s", config.WSPort)
		if err := app.WSServer.Start(); err != nil {
			log.Printf("WebSocket server error: %v", err)
		}
	}()

	// Start HTTP server
	addr := ":" + config.Port
	log.Printf("HTTP server starting on port %s", config.Port)
	if err := app.Router.Run(addr); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func (app *App) InitZDB() error {
	// Create zlay-db connection using the same database configuration
	zdb, err := db.NewConnectionBuilder(db.DatabaseTypePostgreSQL).
		ConnectionString(app.Config.DatabaseURL).
		Build()
	if err != nil {
		return fmt.Errorf("failed to initialize zlay-db: %w", err)
	}

	app.ZDB = zdb
	return nil
}

func extractDomainFromOrigin(origin string) string {
	// Remove protocol
	if strings.HasPrefix(origin, "https://") {
		origin = origin[8:]
	} else if strings.HasPrefix(origin, "http://") {
		origin = origin[7:]
	}

	// Remove port
	if colonIndex := strings.Index(origin, ":"); colonIndex != -1 {
		origin = origin[:colonIndex]
	}

	return origin
}

func extractDomainFromHost(host string) string {
	if colonIndex := strings.Index(host, ":"); colonIndex != -1 {
		return host[:colonIndex]
	}
	return host
}

func (app *App) loadDomainCache() {
	ctx := context.Background()
	app.DomainCache = make(map[string]uuid.UUID)

	resultSet, err := app.ZDB.Query(ctx, "SELECT client_id, domain FROM domains WHERE is_active = true")
	if err != nil {
		log.Printf("Failed to load domain cache: %v", err)
		return
	}

	for _, row := range resultSet.Rows {
		if len(row.Values) >= 2 {
			// Handle client_id - could be binary (UUID) or text
			var clientID string
			if row.Values[0].Type == "binary" {
				bytes, ok := row.Values[0].AsBytes()
				if ok {
					// PostgreSQL UUIDs come as ASCII bytes of the UUID string
					clientID = string(bytes)
				}
			} else {
				clientID, _ = row.Values[0].AsString()
			}
			
			domain, _ := row.Values[1].AsString()
			if clientID != "" && domain != "" {
				parsedClientID, err := uuid.Parse(clientID)
				if err == nil {
					normalizedDomain := extractDomainFromOrigin(domain)
					app.DomainCache[normalizedDomain] = parsedClientID
				}
			}
		}
	}

	log.Printf("Loaded %d domain entries into cache", len(app.DomainCache))
}

func (app *App) InitRouter() {
	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	app.Router = gin.New()
	app.Router.Use(gin.Logger())
	app.Router.Use(gin.Recovery())

	// Initialize WebSocket server with ZDB only
	wsServer := websocket.NewServer(app.ZDB, app.Config.WSPort)
	app.WSServer = wsServer

	// Load domain cache
	app.loadDomainCache()

	// CORS configuration
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"*"}
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Client-ID", "X-Original-Origin"}
	config.AllowCredentials = true
	app.Router.Use(cors.New(config))

	// Health check
	app.Router.GET("/api/health", app.healthHandler)

	// Conversations API
	app.Router.GET("/api/conversations", app.authMiddleware(), app.getConversationsHandler)
	app.Router.GET("/api/conversations/:id/messages", app.authMiddleware(), app.getConversationMessagesHandler)

	// Static routes for development
	app.Router.Static("/assets", "../frontend/dist/assets")
	app.Router.StaticFile("/", "../frontend/dist/index.html")
	app.Router.NoRoute(func(c *gin.Context) {
		c.File("../frontend/dist/index.html")
	})

	// API routes
	api := app.Router.Group("/api")
	{
	api.GET("/hello", app.helloHandler)
		api.POST("/chat", app.authMiddleware(), app.chatHandler)
		// Auth routes
		auth := api.Group("/auth")
		{
			auth.POST("/register", app.registerHandler)
			auth.POST("/login", app.loginHandler)
			auth.POST("/logout", app.logoutHandler)
			auth.GET("/profile", app.authMiddleware(), app.profileHandler)
			auth.OPTIONS("/register", app.corsHandler)
			auth.OPTIONS("/login", app.corsHandler)
			auth.OPTIONS("/logout", app.corsHandler)
			auth.OPTIONS("/profile", app.corsHandler)
		}

		// Project routes
		projects := api.Group("/projects")
		{
			projects.GET("", app.getProjectsHandler)
			projects.POST("", app.createProjectHandler)
			projects.GET("/:id", app.getProjectHandler)
			projects.PUT("/:id", app.updateProjectHandler)
			projects.DELETE("/:id", app.deleteProjectHandler)
			projects.OPTIONS("", app.corsHandler)
			projects.OPTIONS("/:id", app.corsHandler)
		}

		// Datasource routes
		datasources := api.Group("/datasources")
		{
			datasources.GET("", app.getDatasourcesHandler)
			datasources.POST("", app.createDatasourceHandler)
			datasources.GET("/:id", app.getDatasourceHandler)
			datasources.PUT("/:id", app.updateDatasourceHandler)
			datasources.DELETE("/:id", app.deleteDatasourceHandler)
			datasources.OPTIONS("", app.corsHandler)
			datasources.OPTIONS("/:id", app.corsHandler)
		}

		// Admin routes
		admin := api.Group("/admin")
		{
			admin.GET("/clients", app.adminMiddleware(), app.getClientsHandler)
			admin.POST("/clients", app.adminMiddleware(), app.createClientHandler)
			admin.PUT("/clients/:id", app.adminMiddleware(), app.updateClientHandler)
			admin.DELETE("/clients/:id", app.adminMiddleware(), app.deleteClientHandler)
			admin.GET("/domains", app.adminMiddleware(), app.getDomainsHandler)
			admin.POST("/domains", app.adminMiddleware(), app.createDomainHandler)
			admin.PUT("/domains/:id", app.adminMiddleware(), app.updateDomainHandler)
			admin.DELETE("/domains/:id", app.adminMiddleware(), app.deleteDomainHandler)
			admin.OPTIONS("/clients", app.corsHandler)
			admin.OPTIONS("/clients/:id", app.corsHandler)
			admin.OPTIONS("/domains", app.corsHandler)
			admin.OPTIONS("/domains/:id", app.corsHandler)
		}
	}
}

func (app *App) corsHandler(c *gin.Context) {
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Client-ID, X-Original-Origin")
	c.Header("Access-Control-Allow-Credentials", "true")
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (app *App) healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"version":   "1.0.0",
	})
}

// Hello World endpoint
func (app *App) helloHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Hello World"})
}

// Chat endpoint - one-shot LLM chat without persistence
func (app *App) chatHandler(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse request
	var req struct {
		Message string `json:"message" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format: " + err.Error()})
		return
	}

	// Get client ID for LLM configuration
	clientID, err := app.getClientID(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to determine client: " + err.Error()})
		return
	}

	// Get client-specific LLM configuration with timeout protection
	configCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	
	clientConfig, err := app.ClientConfigCache.GetClientConfig(configCtx, clientID.String())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load client configuration: " + err.Error()})
		return
	}

	// Create LLM request with single message
	llmReq := &llm.LLMRequest{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(req.Message),
		},
	}

	// Make LLM call with timeout protection
	llmCtx, llmCancel := context.WithTimeout(ctx, 30*time.Second)
	defer llmCancel()
	
	response, err := clientConfig.LLMClient.Chat(llmCtx, llmReq)
	if err != nil {
		// Check if this is a context cancellation error
		if ctx.Err() == context.Canceled {
			c.JSON(499, gin.H{"error": "Request was cancelled by client"})
			return
		}
		
		// Check if this looks like a connection error and invalidate cache
		if strings.Contains(err.Error(), "connection") || 
		   strings.Contains(err.Error(), "timeout") ||
		   strings.Contains(err.Error(), "network") {
			log.Printf("Connection error detected for client %s, invalidating cache: %v", clientID.String(), err)
			app.ClientConfigCache.InvalidateClientConfig(clientID.String())
		}
		
		c.JSON(http.StatusInternalServerError, gin.H{"error": "LLM call failed: " + err.Error()})
		return
	}

	// Return response
	c.JSON(http.StatusOK, gin.H{
		"response":    response.Content,
		"tokens_used": response.TokensUsed,
		"model":       response.Model,
	})
}

// Helper function to extract client ID from request using ZDB
func (app *App) getClientID(c *gin.Context) (uuid.UUID, error) {
	ctx := c.Request.Context()

	// Try X-Client-ID header first
	if clientIDStr := c.GetHeader("X-Client-ID"); clientIDStr != "" {
		clientID, err := uuid.Parse(clientIDStr)
		if err != nil {
			return uuid.Nil, fmt.Errorf("invalid client ID format: %w", err)
		}

		// Verify client exists using ZDB
		row, err := app.ZDB.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM clients WHERE id = $1 AND is_active = true)",
			clientID)
		if err != nil {
			return uuid.Nil, fmt.Errorf("database error: %w", err)
		}

		exists, ok := row.Values[0].AsBool()
		if !ok {
			return uuid.Nil, fmt.Errorf("failed to parse result")
		}

		if exists {
			return clientID, nil
		}
	}

	// Extract domain from headers
	var domain string
	if origin := c.GetHeader("X-Original-Origin"); origin != "" {
		domain = extractDomainFromOrigin(origin)
	} else if origin := c.GetHeader("Origin"); origin != "" {
		domain = extractDomainFromOrigin(origin)
	} else if referer := c.GetHeader("Referer"); referer != "" {
		domain = extractDomainFromOrigin(referer)
	} else if host := c.GetHeader("Host"); host != "" {
		domain = extractDomainFromHost(host)
	}

	if domain != "" {
		// Check cache first
		if clientID, exists := app.DomainCache[domain]; exists {
			return clientID, nil
		}

		// Fallback to database lookup
		row, err := app.ZDB.QueryRow(ctx,
			"SELECT client_id FROM domains WHERE domain = $1 AND is_active = true LIMIT 1",
			domain)
		if err == nil && len(row.Values) > 0 {
			if clientIDStr, ok := row.Values[0].AsString(); ok {
				clientID := uuid.MustParse(clientIDStr)
				// Update cache
				app.DomainCache[domain] = clientID
				return clientID, nil
			}
		}

		// Try normalized domain lookup using ZDB
		resultSet, err := app.ZDB.Query(ctx, "SELECT client_id, domain FROM domains WHERE is_active = true")
		if err == nil {
			for _, row := range resultSet.Rows {
				if len(row.Values) >= 2 {
					dbClientIDStr, _ := row.Values[0].AsString()
					dbDomain, _ := row.Values[1].AsString()
					normalizedDomain := extractDomainFromOrigin(dbDomain)
					if domain == normalizedDomain {
						dbClientID := uuid.MustParse(dbClientIDStr)
						// Update cache
						app.DomainCache[domain] = dbClientID
						return dbClientID, nil
					}
				}
			}
		}
	}

	// Fallback: if no domain match, use first active client (for development)
	row, err := app.ZDB.QueryRow(ctx,
		"SELECT id FROM clients WHERE is_active = true ORDER BY created_at ASC LIMIT 1")
	if err == nil && len(row.Values) > 0 {
		if clientIDStr, ok := row.Values[0].AsString(); ok {
			clientID := uuid.MustParse(clientIDStr)
			return clientID, nil
		}
	}

	panic("not implemented")
}
