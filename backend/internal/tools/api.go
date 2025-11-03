package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"zlay-backend/internal/db"
)

// APITool executes HTTP requests to REST/GraphQL endpoints
type APITool struct {
	zdb *db.Database
}

// NewAPITool creates a new API tool
func NewAPITool(zdb *db.Database) *APITool {
	return &APITool{
		zdb: zdb,
	}
}

// Name returns tool name
func (t *APITool) Name() string {
	return "api_request"
}

// Description returns tool description
func (t *APITool) Description() string {
	return "Execute HTTP requests to REST/REST API endpoints. Supports GET, POST, PUT, DELETE methods with custom headers and authentication."
}

// Parameters returns tool parameters
func (t *APITool) Parameters() map[string]ToolParameter {
	return map[string]ToolParameter{
		"datasource_id": {
			Type:        "string",
			Description: "ID of the API datasource to use (optional, defaults to project default)",
			Required:    false,
		},
		"method": {
			Type:        "string",
			Description: "HTTP method: GET, POST, PUT, DELETE, PATCH",
			Required:    true,
		},
		"url": {
			Type:        "string",
			Description: "Full URL or endpoint path (relative to datasource base URL)",
			Required:    true,
		},
		"headers": {
			Type:        "object",
			Description: "Additional HTTP headers as key-value pairs",
			Required:    false,
		},
		"body": {
			Type:        "string",
			Description: "Request body for POST/PUT/PATCH requests (JSON string)",
			Required:    false,
		},
		"timeout_seconds": {
			Type:        "number",
			Description: "Request timeout in seconds (default: 30)",
			Required:    false,
			Default:     30,
		},
	}
}

// ValidateAccess checks if user has access to this tool
func (t *APITool) ValidateAccess(userID, projectID string) bool {
	// For now, allow all authenticated users
	// TODO: Implement project-based access control
	return true
}

// GetCategory returns the tool category
func (t *APITool) GetCategory() string {
	return "api"
}

// Execute runs the API request
func (t *APITool) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	startTime := time.Now()

	// Get parameters
	datasourceID, _ := params["datasource_id"].(string)
	method, ok := params["method"].(string)
	if !ok {
		return NewToolError("Missing required parameter: method", nil), nil
	}
	url, ok := params["url"].(string)
	if !ok {
		return NewToolError("Missing required parameter: url", nil), nil
	}

	timeoutSecs := 30
	if timeout, hasTimeout := params["timeout_seconds"]; hasTimeout {
		if ts, ok := timeout.(float64); ok {
			timeoutSecs = int(ts)
		}
	}

	// Create context with timeout
	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSecs)*time.Second)
	defer cancel()

	// Get datasource configuration
	baseURL, authHeaders, err := t.getDatasourceConfig(reqCtx, datasourceID)
	if err != nil {
		return NewToolError("Failed to get datasource configuration", err), nil
	}

	// Prepare full URL
	fullURL := url
	if baseURL != "" && !strings.HasPrefix(url, "http") {
		fullURL = strings.TrimSuffix(baseURL, "/") + "/" + strings.TrimPrefix(url, "/")
	}

	// Prepare headers
	headers := make(map[string]string)
	if headersParam, hasHeaders := params["headers"]; hasHeaders {
		if headersMap, ok := headersParam.(map[string]interface{}); ok {
			for k, v := range headersMap {
				if str, ok := v.(string); ok {
					headers[k] = str
				}
			}
		}
	}

	// Add authentication headers from datasource
	for k, v := range authHeaders {
		headers[k] = v
	}

	// Prepare request body
	var bodyReader io.Reader
	if body, hasBody := params["body"].(string); hasBody && body != "" {
		bodyReader = strings.NewReader(body)
		// Set content-type if not provided
		if _, exists := headers["Content-Type"]; !exists {
			headers["Content-Type"] = "application/json"
		}
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(reqCtx, method, fullURL, bodyReader)
	if err != nil {
		return NewToolError("Failed to create request", err), nil
	}

	// Set headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Execute request
	client := &http.Client{Timeout: time.Duration(timeoutSecs) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return NewToolError("Request failed", err), nil
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return NewToolError("Failed to read response", err), nil
	}

	// Prepare response data
	responseData := map[string]interface{}{
		"status_code": resp.StatusCode,
		"status":      resp.Status,
		"headers":     t.getResponseHeaders(resp),
		"body":        string(respBody),
		"url":         fullURL,
		"method":      method,
	}

	// Try to parse JSON response
	var jsonBody interface{}
	if err := json.Unmarshal(respBody, &jsonBody); err == nil {
		responseData["json"] = jsonBody
	}

	return NewToolSuccess(responseData, int(time.Since(startTime).Milliseconds())), nil
}

func (t *APITool) getDatasourceConfig(ctx context.Context, datasourceID string) (string, map[string]string, error) {
	// If no datasource ID, return empty config (user must provide full URL)
	if datasourceID == "" {
		return "", nil, nil
	}

	// Get datasource details from database with project validation
	row, err := t.zdb.QueryRow(ctx,
		`SELECT d.config FROM datasources d 
		 JOIN projects p ON d.project_id = p.id 
		 WHERE d.id = $1 AND d.is_active = true AND p.is_active = true`,
		datasourceID)
	if err != nil {
		return "", nil, fmt.Errorf("failed to fetch datasource: %w", err)
	}

	if len(row.Values) == 0 {
		return "", nil, fmt.Errorf("datasource not found or not accessible")
	}

	configBytes, ok := row.Values[0].AsBytes()
	if !ok {
		return "", nil, fmt.Errorf("invalid datasource config")
	}

	// Parse API datasource config
	var apiConfig struct {
		BaseURL string            `json:"base_url"`
		Headers map[string]string `json:"headers"`
		Auth    struct {
			Type string `json:"type"`
			// Bearer token auth
			Token string `json:"token"`
			// Basic auth
			Username string `json:"username"`
			Password string `json:"password"`
			// API key auth
			APIKey   string `json:"api_key"`
			KeyHeader string `json:"key_header"` // default: X-API-Key
		} `json:"auth"`
	}

	if err := json.Unmarshal(configBytes, &apiConfig); err != nil {
		return "", nil, fmt.Errorf("failed to parse API config: %w", err)
	}

	// Prepare authentication headers
	authHeaders := make(map[string]string)
	if apiConfig.Headers != nil {
		authHeaders = apiConfig.Headers
	}

	// Add auth headers based on auth type
	switch strings.ToLower(apiConfig.Auth.Type) {
	case "bearer":
		if apiConfig.Auth.Token != "" {
			authHeaders["Authorization"] = "Bearer " + apiConfig.Auth.Token
		}
	case "basic":
		if apiConfig.Auth.Username != "" && apiConfig.Auth.Password != "" {
			// Note: Basic auth handled by HTTP client, but we could set header manually
			authHeaders["Authorization"] = "Basic " + apiConfig.Auth.Username + ":" + apiConfig.Auth.Password
		}
	case "api_key":
		keyHeader := apiConfig.Auth.KeyHeader
		if keyHeader == "" {
			keyHeader = "X-API-Key"
		}
		if apiConfig.Auth.APIKey != "" {
			authHeaders[keyHeader] = apiConfig.Auth.APIKey
		}
	}

	return apiConfig.BaseURL, authHeaders, nil
}

func (t *APITool) getResponseHeaders(resp *http.Response) map[string]string {
	headers := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}
	return headers
}