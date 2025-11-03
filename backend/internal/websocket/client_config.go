package websocket

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"zlay-backend/internal/db"
	"zlay-backend/internal/llm"
)

// ClientConfig represents LLM configuration for a client
type ClientConfig struct {
	ClientID   string
	APIKey    string
	BaseURL    string
	Model      string
	LastUsed   time.Time
	LLMClient llm.LLMClient
}

// ClientConfigCache manages cached LLM configurations for clients
type ClientConfigCache struct {
	cache map[string]*ClientConfig
	mutex sync.RWMutex
	db    *db.Database
	
	// Default configuration from environment variables
	defaultAPIKey  string
	defaultBaseURL string
	defaultModel   string
}

// NewClientConfigCache creates a new client configuration cache
func NewClientConfigCache(zdb *db.Database) *ClientConfigCache {
	// Get default values from environment variables
	defaultAPIKey := os.Getenv("OPENAI_API_KEY")
	if defaultAPIKey == "" {
		defaultAPIKey = "sk-no-key-required"
	}
	defaultBaseURL := os.Getenv("OPENAI_BASE_URL")
	defaultModel := os.Getenv("OPENAI_MODEL")
	if defaultModel == "" {
		defaultModel = "gpt-3.5-turbo"
	}

	return &ClientConfigCache{
		cache:         make(map[string]*ClientConfig),
		db:            zdb,
		defaultAPIKey:  defaultAPIKey,
		defaultBaseURL: defaultBaseURL,
		defaultModel:   defaultModel,
	}
}

// GetClientConfig retrieves or creates LLM configuration for a client
func (c *ClientConfigCache) GetClientConfig(ctx context.Context, clientID string) (*ClientConfig, error) {
	// Check cache first with proper lock management
	c.mutex.RLock()
	config, exists := c.cache[clientID]
	if exists {
		// Check if cache is still valid (5 minutes)
		if time.Since(config.LastUsed) < 5*time.Minute {
			config.LastUsed = time.Now()
			c.mutex.RUnlock()
			log.Printf("Using cached LLM config for client %s", clientID)
			return config, nil
		}
	}
	c.mutex.RUnlock()

	// Need to fetch from database
	config, err := c.fetchClientConfig(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch client config: %w", err)
	}

	// Cache the configuration with proper lock management
	c.mutex.Lock()
	c.cache[clientID] = config
	c.mutex.Unlock()

	log.Printf("Loaded LLM config for client %s: model=%s, baseURL=%s", clientID, config.Model, config.BaseURL)
	return config, nil
}

// fetchClientConfig retrieves client configuration from database
func (c *ClientConfigCache) fetchClientConfig(ctx context.Context, clientID string) (*ClientConfig, error) {
	// Query client configuration
	row, err := c.db.QueryRow(ctx,
		`SELECT id, ai_api_key, ai_api_url, ai_api_model 
		FROM clients 
		WHERE id = $1 AND is_active = true`,
		clientID)
	
	if err != nil {
		return nil, fmt.Errorf("database query error: %w", err)
	}

	if len(row.Values) != 4 {
		return nil, fmt.Errorf("client not found or inactive: %s", clientID)
	}

	// Extract values
	clientIDFromDB, ok := row.Values[0].AsString()
	if !ok {
		return nil, fmt.Errorf("invalid client ID in database")
	}

	apiKey, ok := row.Values[1].AsString()
	if !ok || apiKey == "" {
		apiKey = c.defaultAPIKey
	}

	baseURL, ok := row.Values[2].AsString()
	if !ok || baseURL == "" {
		baseURL = c.defaultBaseURL
	}

	model, ok := row.Values[3].AsString()
	if !ok || model == "" {
		model = c.defaultModel
	}

	// Create LLM client with client-specific configuration
	llmClient := llm.NewOpenAIClient(apiKey, baseURL, model)

	// Validate the connection if possible (with timeout)
	validateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := llmClient.ValidateConnection(validateCtx); err != nil {
		log.Printf("Warning: LLM connection validation failed for client %s: %v", clientID, err)
		// Don't return error, just log it - the config might work for some requests
	}

	return &ClientConfig{
		ClientID:   clientIDFromDB,
		APIKey:    apiKey,
		BaseURL:    baseURL,
		Model:      model,
		LastUsed:   time.Now(),
		LLMClient:  llmClient,
	}, nil
}

// InvalidateClientConfig removes a client from cache (useful for configuration updates)
func (c *ClientConfigCache) InvalidateClientConfig(clientID string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	if config, exists := c.cache[clientID]; exists {
		// Close any resources if needed
		if config.LLMClient != nil {
			// Note: OpenAI client doesn't have explicit close method in current implementation
		}
		delete(c.cache, clientID)
		log.Printf("Invalidated LLM config cache for client %s", clientID)
	}
}

// CleanupExpiredConfigs removes expired configurations from cache
func (c *ClientConfigCache) CleanupExpiredConfigs() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	now := time.Now()
	for clientID, config := range c.cache {
		if now.Sub(config.LastUsed) > 30*time.Minute {
			delete(c.cache, clientID)
			log.Printf("Cleaned up expired LLM config cache for client %s", clientID)
		}
	}
}

// GetCacheStats returns cache statistics
func (c *ClientConfigCache) GetCacheStats() map[string]interface{} {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	return map[string]interface{}{
		"cached_clients": len(c.cache),
		"default_model":  c.defaultModel,
		"default_url":    c.defaultBaseURL,
	}
}

// StartCleanupRoutine starts a background routine to clean up expired configurations
func (c *ClientConfigCache) StartCleanupRoutine() {
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		
		for range ticker.C {
			c.CleanupExpiredConfigs()
		}
	}()
	log.Printf("Started client config cache cleanup routine")
}