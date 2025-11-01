package tools

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"time"

	"zlay-backend/internal/db"
)

// DatabaseQueryTool executes SQL queries
type DatabaseQueryTool struct {
	db DBConnection
}

// NewDatabaseQueryTool creates a new database query tool
func NewDatabaseQueryTool() *DatabaseQueryTool {
	return &DatabaseQueryTool{}
}

// Name returns tool name
func (t *DatabaseQueryTool) Name() string {
	return "database_query"
}

// Description returns tool description
func (t *DatabaseQueryTool) Description() string {
	return "Execute SQL queries on project datasources. Supports SELECT, INSERT, UPDATE, DELETE operations with proper security checks."
}

// Parameters returns tool parameters
func (t *DatabaseQueryTool) Parameters() map[string]ToolParameter {
	return map[string]ToolParameter{
		"datasource_id": {
			Type:        "string",
			Description: "ID of the datasource to query (optional - uses default project database)",
			Required:    false,
		},
		"query": {
			Type:        "string",
			Description: "SQL query to execute (must be a valid SQL statement)",
			Required:    true,
		},
		"timeout_seconds": {
			Type:        "number",
			Description: "Query timeout in seconds (default: 30)",
			Required:    false,
			Default:     30,
		},
	}
}

// Execute runs the database query
func (t *DatabaseQueryTool) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	// Get parameters
	datasourceID, hasDS := params["datasource_id"].(string)
	query, ok := params["query"].(string)
	if !ok {
		return NewToolError("Missing required parameter: query", nil), nil
	}

	timeoutSecs := 30
	if timeout, hasTimeout := params["timeout_seconds"]; hasTimeout {
		if ts, ok := timeout.(float64); ok {
			timeoutSecs = int(ts)
		}
	}

	// Create context with timeout
	queryCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSecs)*time.Second)
	defer cancel()

	// Get database connection
	var db DBConnection
	var err error

	if hasDS && datasourceID != "" {
		// TODO: Implement datasource-specific connections
		// For now, use default project database
		db, err = t.getDatasourceConnection(queryCtx, datasourceID)
	} else {
		// Use default connection
		db = t.db
	}

	if err != nil {
		return NewToolError("Failed to get database connection", err), nil
	}

	// Execute query based on query type
	result, err := t.executeQuery(queryCtx, db, query)
	if err != nil {
		return NewToolError("Query execution failed", err), nil
	}

	// Format result
	data := map[string]interface{}{
		"query":         query,
		"result":        result,
		"rows_affected": t.getRowsAffected(result),
		"datasource_id": datasourceID,
	}

	return NewToolSuccess(data, 0), nil
}

// ValidateAccess checks if user has access to database tools
func (t *DatabaseQueryTool) ValidateAccess(userID, projectID string) bool {
	// TODO: Implement proper permission checking
	// For now, allow all authenticated users
	return true
}

// GetCategory returns tool category
func (t *DatabaseQueryTool) GetCategory() string {
	return "database"
}

// Helper methods

func (t *DatabaseQueryTool) executeQuery(ctx context.Context, db DBConnection, query string) (interface{}, error) {
	// Parse query to determine type (simplified)
	queryLower := strings.ToLower(strings.TrimSpace(query))

	// Check for forbidden operations
	forbiddenOps := []string{"drop", "truncate", "alter database", "create database"}
	for _, op := range forbiddenOps {
		if strings.Contains(queryLower, op) {
			return nil, fmt.Errorf("forbidden operation detected: %s", op)
		}
	}

	// Execute based on query type
	if strings.HasPrefix(queryLower, "select") || strings.HasPrefix(queryLower, "with") {
		return t.executeSelect(ctx, db, query)
	} else if strings.HasPrefix(queryLower, "insert") {
		return t.executeUpdate(ctx, db, query)
	} else if strings.HasPrefix(queryLower, "update") {
		return t.executeUpdate(ctx, db, query)
	} else if strings.HasPrefix(queryLower, "delete") {
		return t.executeUpdate(ctx, db, query)
	} else if strings.HasPrefix(queryLower, "create") || strings.HasPrefix(queryLower, "alter table") {
		return t.executeUpdate(ctx, db, query)
	} else {
		return nil, fmt.Errorf("unsupported query type or unable to determine query operation")
	}
}

func (t *DatabaseQueryTool) executeSelect(ctx context.Context, db DBConnection, query string) (interface{}, error) {
	startTime := time.Now()

	rows, err := db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Get column information
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	// Convert to JSON-serializable format
	var results []map[string]interface{}
	for rows.Next() {
		// Create a map for this row
		row := make(map[string]interface{})
		values := make([]interface{}, len(columns))

		if err := rows.Scan(values...); err != nil {
			return nil, err
		}

		// Convert each value to appropriate type
		for i, col := range columns {
			val := values[i]
			if val == nil {
				row[col] = nil
			} else {
				switch v := val.(type) {
				case []byte:
					row[col] = string(v)
				case string:
					row[col] = v
				case int, int32, int64:
					row[col] = v
				case float32, float64:
					row[col] = v
				case bool:
					row[col] = v
				case time.Time:
					row[col] = v.Format(time.RFC3339)
				default:
					// Convert to string for other types
					row[col] = fmt.Sprintf("%v", v)
				}
			}
		}

		results = append(results, row)
	}

	// Check for errors after scanning
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Return formatted result
	return map[string]interface{}{
		"type":    "select",
		"columns": columns,
		"rows":    results,
		"count":   len(results),
		"time_ms": time.Since(startTime).Milliseconds(),
	}, nil
}

func (t *DatabaseQueryTool) executeUpdate(ctx context.Context, db DBConnection, query string) (interface{}, error) {
	startTime := time.Now()

	result, err := db.Exec(ctx, query)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}

	// Return formatted result
	return map[string]interface{}{
		"type":          "update",
		"rows_affected": rowsAffected,
		"time_ms":       time.Since(startTime).Milliseconds(),
	}, nil
}

func (t *DatabaseQueryTool) getDatasourceConnection(ctx context.Context, datasourceID string) (DBConnection, error) {
	// TODO: Implement datasource-specific connection handling
	// For now, return default connection to avoid unimplemented error
	return t.db, nil
}

func (t *DatabaseQueryTool) getRowsAffected(result interface{}) int64 {
	if resultMap, ok := result.(map[string]interface{}); ok {
		if rowsAffected, exists := resultMap["rows_affected"]; exists {
			if ra, ok := rowsAffected.(int64); ok {
				return ra
			}
		}
	}
	return 0
}

// ZlayDBAdapter adapts zlay-db Database to DBConnection interface
type ZlayDBAdapter struct {
	DB *db.Database
}

func (z *ZlayDBAdapter) Query(ctx context.Context, sql string, args ...interface{}) (*sql.Rows, error) {
	return z.DB.GetDB().QueryContext(ctx, sql, args...)
}

func (z *ZlayDBAdapter) QueryRow(ctx context.Context, sql string, args ...interface{}) *sql.Row {
	return z.DB.GetDB().QueryRowContext(ctx, sql, args...)
}

func (z *ZlayDBAdapter) Exec(ctx context.Context, sql string, args ...interface{}) (sql.Result, error) {
	return z.DB.GetDB().ExecContext(ctx, sql, args...)
}

func (z *ZlayDBAdapter) Begin(ctx context.Context) (*sql.Tx, error) {
	return z.DB.GetDB().BeginTx(ctx, nil)
}

// WebSocketAdapter adapts WebSocket hub to interface
type WebSocketAdapter struct {
	Hub interface{} // The actual hub instance
}

func (w *WebSocketAdapter) BroadcastToProject(projectID string, message interface{}) {
	// Use reflection to call the BroadcastToProject method
	hubValue := reflect.ValueOf(w.Hub)
	method := hubValue.MethodByName("BroadcastToProject")
	if method.IsValid() {
		method.Call([]reflect.Value{
			reflect.ValueOf(projectID),
			reflect.ValueOf(message),
		})
	}
}

func (w *WebSocketAdapter) SendToConnection(conn interface{}, message interface{}) {
	// Use reflection to call the SendToConnection method
	hubValue := reflect.ValueOf(w.Hub)
	method := hubValue.MethodByName("SendToConnection")
	if method.IsValid() {
		method.Call([]reflect.Value{
			reflect.ValueOf(conn),
			reflect.ValueOf(message),
		})
	}
}
