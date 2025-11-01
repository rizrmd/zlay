package tools

import (
	"context"
	"runtime"
	"time"
)

// SystemInfoTool provides system information
type SystemInfoTool struct{}

// NewSystemInfoTool creates a new system info tool
func NewSystemInfoTool() *SystemInfoTool {
	return &SystemInfoTool{}
}

// Name returns tool name
func (t *SystemInfoTool) Name() string {
	return "system_info"
}

// Description returns tool description
func (t *SystemInfoTool) Description() string {
	return "Get system information including OS, architecture, Go version, and resource usage"
}

// Parameters returns tool parameters
func (t *SystemInfoTool) Parameters() map[string]ToolParameter {
	return map[string]ToolParameter{
		"include_memory": {
			Type:        "boolean",
			Description: "Include memory usage information",
			Required:    false,
			Default:     true,
		},
		"include_disk": {
			Type:        "boolean",
			Description: "Include disk usage information",
			Required:    false,
			Default:     false,
		},
	}
}

// Execute runs the system info tool
func (t *SystemInfoTool) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	startTime := time.Now()
	
	// Get parameters
	includeMemory := true
	if mem, ok := params["include_memory"].(bool); ok {
		includeMemory = mem
	}
	
	includeDisk := false
	if disk, ok := params["include_disk"].(bool); ok {
		includeDisk = disk
	}
	
	// Collect system information
	info := map[string]interface{}{
		"os":           runtime.GOOS,
		"architecture": runtime.GOARCH,
		"go_version":   runtime.Version(),
		"num_goroutines": runtime.NumGoroutine(),
		"num_cpu":      runtime.NumCPU(),
		"timestamp":     time.Now().Format(time.RFC3339),
	}
	
	// Add memory info if requested
	if includeMemory {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		
		info["memory"] = map[string]interface{}{
			"alloc_bytes":      m.Alloc,
			"total_alloc":      m.TotalAlloc,
			"sys_bytes":       m.Sys,
			"num_gc":         m.NumGC,
			"heap_alloc":      m.HeapAlloc,
			"heap_sys":       m.HeapSys,
			"heap_idle":      m.HeapIdle,
			"heap_inuse":     m.HeapInuse,
			"heap_released":  m.HeapReleased,
			"heap_objects":   m.HeapObjects,
			"stack_inuse":    m.StackInuse,
			"stack_sys":      m.StackSys,
			"gc_cpu_fraction": m.GCCPUFraction,
		}
	}
	
	// Add disk info if requested
	if includeDisk {
		// TODO: Implement disk usage collection
		info["disk"] = map[string]interface{}{
			"message": "Disk information not yet implemented",
		}
	}
	
	return NewToolSuccess(info, int(time.Since(startTime).Milliseconds())), nil
}

// ValidateAccess checks if user has access to this tool
func (t *SystemInfoTool) ValidateAccess(userID, projectID string) bool {
	// System info tool is generally safe and useful for debugging
	return true
}

// GetCategory returns tool category
func (t *SystemInfoTool) GetCategory() string {
	return "system"
}
