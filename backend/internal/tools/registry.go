package tools

import (
	"context"
	"fmt"
	"log"
	"sync"
)

// DefaultToolRegistry implements ToolRegistry
type DefaultToolRegistry struct {
	tools map[string]Tool
	mutex sync.RWMutex
}

// NewDefaultToolRegistry creates a new default tool registry
func NewDefaultToolRegistry() *DefaultToolRegistry {
	registry := &DefaultToolRegistry{
		tools: make(map[string]Tool),
	}
	
	// Register built-in tools
	registry.RegisterBuiltInTools()
	
	return registry
}

// RegisterTool adds a new tool to the registry
func (r *DefaultToolRegistry) RegisterTool(tool Tool) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	name := tool.Name()
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool '%s' is already registered", name)
	}
	
	r.tools[name] = tool
	log.Printf("Registered tool: %s", name)
	return nil
}

// UnregisterTool removes a tool from the registry
func (r *DefaultToolRegistry) UnregisterTool(name string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	if _, exists := r.tools[name]; !exists {
		return fmt.Errorf("tool '%s' is not registered", name)
	}
	
	delete(r.tools, name)
	log.Printf("Unregistered tool: %s", name)
	return nil
}

// GetTool retrieves a tool by name
func (r *DefaultToolRegistry) GetTool(name string) (Tool, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	tool, exists := r.tools[name]
	return tool, exists
}

// GetAvailableTools returns all tools available for a project
func (r *DefaultToolRegistry) GetAvailableTools(projectID string) []Tool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	var availableTools []Tool
	for _, tool := range r.tools {
		// For now, make all tools available to all projects
		// TODO: Implement project-based tool restrictions
		availableTools = append(availableTools, tool)
	}
	
	return availableTools
}

// ExecuteTool executes a tool by name with given parameters
func (r *DefaultToolRegistry) ExecuteTool(ctx context.Context, userID, projectID, toolName string, params map[string]interface{}) (*ToolResult, error) {
	tool, exists := r.GetTool(toolName)
	if !exists {
		return nil, ErrToolNotFound
	}
	
	// Validate user access
	if !tool.ValidateAccess(userID, projectID) {
		return nil, ErrToolAccessDenied
	}
	
	// Validate parameters
	if err := ValidateToolParameters(params, tool.Parameters()); err != nil {
		return nil, fmt.Errorf("invalid parameters for tool %s: %w", toolName, err)
	}
	
	// Execute tool
	log.Printf("Executing tool %s for user %s in project %s", toolName, userID, projectID)
	result, err := tool.Execute(ctx, params)
	
	if err != nil {
		return NewToolError(fmt.Sprintf("Tool %s failed", toolName), err), nil
	}
	
	return result, nil
}

// ListTools returns a list of all registered tools
func (r *DefaultToolRegistry) ListTools() []Tool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	var tools []Tool
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	
	return tools
}

// RegisterBuiltInTools registers all built-in tools
func (r *DefaultToolRegistry) RegisterBuiltInTools() {
	// Register database tools
	dbTool := NewDatabaseQueryTool()
	if err := r.RegisterTool(dbTool); err != nil {
		log.Printf("Failed to register DatabaseQueryTool: %v", err)
	}
}

// EmptyToolRegistry implements ToolRegistry with no tools
type EmptyToolRegistry struct{}

// NewToolRegistry creates a new tool registry (alias for backwards compatibility)
func NewToolRegistry() ToolRegistry {
	return NewDefaultToolRegistry()
}

// RegisterTool adds a new tool to registry
func (r *EmptyToolRegistry) RegisterTool(tool Tool) error {
	// Do nothing for empty registry
	return nil
}

// UnregisterTool removes a tool from registry
func (r *EmptyToolRegistry) UnregisterTool(name string) error {
	// Do nothing for empty registry
	return nil
}

// GetTool retrieves a tool by name
func (r *EmptyToolRegistry) GetTool(name string) (Tool, bool) {
	// Always return not found for empty registry
	return nil, false
}

// GetAvailableTools returns all tools available for a project
func (r *EmptyToolRegistry) GetAvailableTools(projectID string) []Tool {
	// Always return empty for empty registry
	return []Tool{}
}

// ExecuteTool executes a tool by name with given parameters
func (r *EmptyToolRegistry) ExecuteTool(ctx context.Context, userID, projectID, toolName string, params map[string]interface{}) (*ToolResult, error) {
	// Always return not found for empty registry
	return nil, ErrToolNotFound
}

// ListTools returns a list of all registered tools
func (r *EmptyToolRegistry) ListTools() []Tool {
	// Always return empty for empty registry
	return []Tool{}
}
