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
	"github.com/joho/godotenv"
	"zlay-backend/internal/db"
	"zlay-backend/internal/websocket"
)

type Config struct {
	DatabaseURL string
	Port        string
	WSPort      string
}

type App struct {
	Config   *Config
	ZDB      *db.Database // Zlay-db abstraction - SINGLE source of truth for database operations
	Router   *gin.Engine
	WSServer *websocket.Server
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
		Port:        getEnv("PORT", "8080"),
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

	// Start WebSocket server in separate goroutine
	go func() {
		log.Printf("WebSocket server starting on port %s", config.WSPort)
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

func (app *App) InitRouter() {
	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	app.Router = gin.New()
	app.Router.Use(gin.Logger())
	app.Router.Use(gin.Recovery())

	// Initialize WebSocket server with ZDB only
	wsServer := websocket.NewServer(app.ZDB, app.Config.WSPort)
	app.WSServer = wsServer

	// CORS configuration
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"*"}
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Client-ID", "X-Original-Origin"}
	config.AllowCredentials = true
	app.Router.Use(cors.New(config))

	// Health check
	app.Router.GET("/api/health", app.healthHandler)

	// Static routes for development
	app.Router.Static("/assets", "../frontend/dist/assets")
	app.Router.StaticFile("/", "../frontend/dist/index.html")
	app.Router.NoRoute(func(c *gin.Context) {
		c.File("../frontend/dist/index.html")
	})

	// API routes
	api := app.Router.Group("/api")
	{
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
		projects.Use(app.authMiddleware())
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
		datasources.Use(app.authMiddleware())
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
		admin.Use(app.authMiddleware())
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

// Helper function to extract client ID from request using ZDB
func (app *App) getClientID(c *gin.Context) (string, error) {
	ctx := c.Request.Context()

	// Try to get client_id from X-Client-ID header first
	if clientID := c.GetHeader("X-Client-ID"); clientID != "" {
		// Verify client exists using ZDB
		row, err := app.ZDB.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM clients WHERE id = $1 AND is_active = true)",
			clientID)
		if err != nil {
			return "", fmt.Errorf("database error: %w", err)
		}
		
		var exists bool
		if err := row.Values[0].GetBool(&exists); err != nil {
			return "", fmt.Errorf("failed to parse result: %w", err)
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
		// Look up client by domain using ZDB
		row, err := app.ZDB.QueryRow(ctx,
			"SELECT client_id FROM domains WHERE domain = $1 AND is_active = true LIMIT 1",
			domain)
		if err == nil && len(row.Values) > 0 {
			if clientID, err := row.Values[0].GetString(); err == nil {
				return clientID, nil
			}
		}

		// Try normalized domain lookup using ZDB
		resultSet, err := app.ZDB.Query(ctx, "SELECT client_id, domain FROM domains WHERE is_active = true")
		if err == nil {
			for _, row := range resultSet.Rows {
				if len(row.Values) >= 2 {
					dbClientID, _ := row.Values[0].GetString()
					dbDomain, _ := row.Values[1].GetString()
					normalizedDomain := extractDomainFromOrigin(dbDomain)
					if domain == normalizedDomain {
						return dbClientID, nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("no valid client found")
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
