package tools

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ToolParameter defines a tool parameter
type ToolParameter struct {
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Required    bool        `json:"required"`
	Default     interface{} `json:"default,omitempty"`
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	Status string                 `json:"status"` // completed, failed, error
	Data   map[string]interface{} `json:"data,omitempty"`
	Error  string                 `json:"error,omitempty"`
	TimeMs int                    `json:"time_ms,omitempty"`
}

// Tool defines the interface for all tools
type Tool interface {
	// Name returns the unique name of the tool
	Name() string

	// Description returns a human-readable description of what the tool does
	Description() string

	// Parameters returns the parameters this tool accepts
	Parameters() map[string]ToolParameter

	// Execute runs the tool with the given parameters
	Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error)

	// ValidateAccess checks if the given user has permission to use this tool
	ValidateAccess(userID, projectID string) bool

	// GetCategory returns the category of this tool (database, api, filesystem, etc.)
	GetCategory() string
}

// ToolRegistry defines the interface for tool management
type ToolRegistry interface {
	// RegisterTool adds a new tool to the registry
	RegisterTool(tool Tool) error

	// UnregisterTool removes a tool from the registry
	UnregisterTool(name string) error

	// GetTool retrieves a tool by name
	GetTool(name string) (Tool, bool)

	// GetAvailableTools returns all tools available for a project
	GetAvailableTools(projectID string) []Tool

	// ExecuteTool executes a tool by name with the given parameters
	ExecuteTool(ctx context.Context, userID, projectID, toolName string, params map[string]interface{}) (*ToolResult, error)

	// ListTools returns a list of all registered tools
	ListTools() []Tool
}

// WebSocketHub defines the interface for WebSocket communication
type WebSocketHub interface {
	// BroadcastToProject sends a message to all connections in a project room
	BroadcastToProject(projectID string, message interface{})

	// SendToConnection sends a message to a specific connection
	SendToConnection(conn interface{}, message interface{})
}

// DBConnection defines the interface for database operations
type DBConnection interface {
	Query(ctx context.Context, sql string, args ...interface{}) (*sql.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) *sql.Row
	Exec(ctx context.Context, sql string, args ...interface{}) (sql.Result, error)
	Begin(ctx context.Context) (*sql.Tx, error)
}

// WebSocketMessage represents a message sent over WebSocket
type WebSocketMessage struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp int64       `json:"timestamp"`
	ID        string      `json:"id,omitempty"`
}

// Error types
var (
	ErrToolNotFound        = errors.New("tool not found")
	ErrToolAccessDenied    = errors.New("access denied for tool")
	ErrInvalidParameters   = errors.New("invalid tool parameters")
	ErrToolExecutionFailed = errors.New("tool execution failed")
)

// Helper functions

// NewToolResult creates a new tool result
func NewToolResult(status string, data map[string]interface{}) *ToolResult {
	return &ToolResult{
		Status: status,
		Data:   data,
	}
}

// NewToolError creates a new tool error result
func NewToolError(message string, err error) *ToolResult {
	errorMsg := message
	if err != nil {
		errorMsg = fmt.Sprintf("%s: %v", message, err)
	}
	return &ToolResult{
		Status: "failed",
		Error:  errorMsg,
	}
}

// NewToolSuccess creates a new successful tool result
func NewToolSuccess(data map[string]interface{}, timeMs int) *ToolResult {
	return &ToolResult{
		Status: "completed",
		Data:   data,
		TimeMs: timeMs,
	}
}

// ValidateToolParameters checks if all required parameters are present
func ValidateToolParameters(params map[string]interface{}, toolParams map[string]ToolParameter) error {
	for name, param := range toolParams {
		if param.Required {
			if _, exists := params[name]; !exists {
				return fmt.Errorf("missing required parameter: %s", name)
			}
		}
	}
	return nil
}
