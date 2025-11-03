package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"zlay-backend/internal/db"
	"zlay-backend/internal/websocket"
)

func TestChatHandler(t *testing.T) {
	// Create a test app instance
	app := &App{
		Config: &Config{
			DatabaseURL: "postgresql://postgres:password@localhost:5432/zlay",
			Port:        "8080",
			WSPort:      "6070",
		},
		DomainCache: make(map[string]uuid.UUID),
	}

	// Initialize ZDB (this would need a test database)
	zdb, err := db.NewConnectionBuilder(db.DatabaseTypePostgreSQL).
		ConnectionString("postgresql://postgres:password@localhost:5432/zlay_test").
		Build()
	if err != nil {
		t.Skipf("Skipping test: cannot connect to test database: %v", err)
		return
	}
	app.ZDB = zdb
	defer app.ZDB.Close()

	// Initialize client config cache
	app.ClientConfigCache = websocket.NewClientConfigCache(app.ZDB)

	// Setup router
	gin.SetMode(gin.TestMode)
	app.InitRouter()

	// Create test request
	reqBody := map[string]string{
		"message": "Hello, how are you?",
	}
	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/api/chat", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	// Create response recorder
	w := httptest.NewRecorder()

	// Serve the request
	app.Router.ServeHTTP(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
		t.Errorf("Response body: %s", w.Body.String())
	}

	fmt.Printf("Test completed with response: %s\n", w.Body.String())
}