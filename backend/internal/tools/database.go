package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"zlay-backend/internal/db"
)

// DatabaseQueryTool executes SQL queries
type DatabaseQueryTool struct {
	db DBConnection
	zdb *db.Database
}

// NewDatabaseQueryTool creates a new database query tool
func NewDatabaseQueryTool(zdb *db.Database) *DatabaseQueryTool {
	return &DatabaseQueryTool{
		zdb: zdb,
	}
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
	// If no datasource ID, use default connection
	if datasourceID == "" {
		return t.db, nil
	}

	// Get datasource details from database with project validation
	row, err := t.zdb.QueryRow(ctx, 
		`SELECT d.type, d.config FROM datasources d 
		 JOIN projects p ON d.project_id = p.id 
		 WHERE d.id = $1 AND d.is_active = true AND p.is_active = true`, 
		datasourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch datasource: %w", err)
	}

	if len(row.Values) < 2 {
		return nil, fmt.Errorf("datasource not found or not accessible")
	}

	dsType, ok := row.Values[0].AsString()
	if !ok {
		return nil, fmt.Errorf("invalid datasource type")
	}

	configBytes, ok := row.Values[1].AsBytes()
	if !ok {
		return nil, fmt.Errorf("invalid datasource config")
	}

	// Parse config based on datasource type
	switch strings.ToLower(dsType) {
	case "postgres", "postgresql":
		return t.createPostgresConnection(configBytes)
	case "mysql":
		return t.createMySQLConnection(configBytes)
	case "sqlite", "sqlite3":
		return t.createSQLiteConnection(configBytes)
	case "sqlserver", "mssql":
		return t.createSQLServerConnection(configBytes)
	case "oracle":
		return t.createOracleConnection(configBytes)
	case "trino", "presto":
		return t.createTrinoConnection(configBytes)
	case "clickhouse":
		return t.createClickHouseConnection(configBytes)
	default:
		return nil, fmt.Errorf("unsupported datasource type: %s", dsType)
	}
}

func (t *DatabaseQueryTool) createPostgresConnection(config []byte) (DBConnection, error) {
	var pgConfig struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Database string `json:"database"`
		Username string `json:"username"`
		Password string `json:"password"`
		SSLMode  string `json:"ssl_mode"`
	}

	if err := json.Unmarshal(config, &pgConfig); err != nil {
		return nil, fmt.Errorf("failed to parse postgres config: %w", err)
	}

	if pgConfig.SSLMode == "" {
		pgConfig.SSLMode = "disable"
	}

	// Create connection using ZDB
	zdb, err := db.NewConnectionBuilder(db.DatabaseTypePostgreSQL).
		Host(pgConfig.Host).
		Port(pgConfig.Port).
		Database(pgConfig.Database).
		Username(pgConfig.Username).
		Password(pgConfig.Password).
		SSLMode(pgConfig.SSLMode).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres connection: %w", err)
	}

	return &ZlayDBAdapter{DB: zdb}, nil
}

func (t *DatabaseQueryTool) createMySQLConnection(config []byte) (DBConnection, error) {
	var mysqlConfig struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Database string `json:"database"`
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.Unmarshal(config, &mysqlConfig); err != nil {
		return nil, fmt.Errorf("failed to parse mysql config: %w", err)
	}

	// Create connection using ZDB
	zdb, err := db.NewConnectionBuilder(db.DatabaseTypeMySQL).
		Host(mysqlConfig.Host).
		Port(mysqlConfig.Port).
		Database(mysqlConfig.Database).
		Username(mysqlConfig.Username).
		Password(mysqlConfig.Password).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create mysql connection: %w", err)
	}

	return &ZlayDBAdapter{DB: zdb}, nil
}

func (t *DatabaseQueryTool) createSQLiteConnection(config []byte) (DBConnection, error) {
	var sqliteConfig struct {
		FilePath string `json:"file_path"`
	}

	if err := json.Unmarshal(config, &sqliteConfig); err != nil {
		return nil, fmt.Errorf("failed to parse sqlite config: %w", err)
	}

	// Create connection using ZDB
	zdb, err := db.NewConnectionBuilder(db.DatabaseTypeSQLite).
		FilePath(sqliteConfig.FilePath).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create sqlite connection: %w", err)
	}

	return &ZlayDBAdapter{DB: zdb}, nil
}

func (t *DatabaseQueryTool) createSQLServerConnection(config []byte) (DBConnection, error) {
	var sqlConfig struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Database string `json:"database"`
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.Unmarshal(config, &sqlConfig); err != nil {
		return nil, fmt.Errorf("failed to parse sqlserver config: %w", err)
	}

	// Create connection using ZDB
	zdb, err := db.NewConnectionBuilder(db.DatabaseTypeSQLServer).
		Host(sqlConfig.Host).
		Port(sqlConfig.Port).
		Database(sqlConfig.Database).
		Username(sqlConfig.Username).
		Password(sqlConfig.Password).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create sqlserver connection: %w", err)
	}

	return &ZlayDBAdapter{DB: zdb}, nil
}

func (t *DatabaseQueryTool) createOracleConnection(config []byte) (DBConnection, error) {
	var oracleConfig struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Database string `json:"database"`
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.Unmarshal(config, &oracleConfig); err != nil {
		return nil, fmt.Errorf("failed to parse oracle config: %w", err)
	}

	// Create connection using ZDB
	zdb, err := db.NewConnectionBuilder(db.DatabaseTypeOracle).
		Host(oracleConfig.Host).
		Port(oracleConfig.Port).
		Database(oracleConfig.Database).
		Username(oracleConfig.Username).
		Password(oracleConfig.Password).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create oracle connection: %w", err)
	}

	return &ZlayDBAdapter{DB: zdb}, nil
}

func (t *DatabaseQueryTool) createTrinoConnection(config []byte) (DBConnection, error) {
	var trinoConfig struct {
		ServerURL string `json:"server_url"`
		Catalog   string `json:"catalog"`
		Schema    string `json:"schema"`
		Username  string `json:"username"`
		Password  string `json:"password"`
	}

	if err := json.Unmarshal(config, &trinoConfig); err != nil {
		return nil, fmt.Errorf("failed to parse trino config: %w", err)
	}

	// For Trino, use ConnectionString since the builder methods may not exist for all fields
	connStr := trinoConfig.ServerURL
	
	// Create connection using ZDB with connection string
	zdb, err := db.NewConnectionBuilder(db.DatabaseTypeTrino).
		ConnectionString(connStr).
		Username(trinoConfig.Username).
		Password(trinoConfig.Password).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create trino connection: %w", err)
	}

	return &ZlayDBAdapter{DB: zdb}, nil
}

func (t *DatabaseQueryTool) createClickHouseConnection(config []byte) (DBConnection, error) {
	var clickConfig struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Database string `json:"database"`
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.Unmarshal(config, &clickConfig); err != nil {
		return nil, fmt.Errorf("failed to parse clickhouse config: %w", err)
	}

	// Create connection using ZDB
	zdb, err := db.NewConnectionBuilder(db.DatabaseTypeClickHouse).
		Host(clickConfig.Host).
		Port(clickConfig.Port).
		Database(clickConfig.Database).
		Username(clickConfig.Username).
		Password(clickConfig.Password).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create clickhouse connection: %w", err)
	}

	return &ZlayDBAdapter{DB: zdb}, nil
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

// DatasourceInfo provides unified information about any datasource
type DatasourceInfo struct {
	Type             string            `json:"type"`
	DatabaseName     string            `json:"database_name"`
	Host             string            `json:"host,omitempty"`
	Port             int               `json:"port,omitempty"`
	Version          string            `json:"version,omitempty"`
	Status           string            `json:"status"`
	ConnectionTimeMs int               `json:"connection_time_ms"`
	TableCount       int               `json:"table_count,omitempty"`
	Tables           []TableInfo       `json:"tables,omitempty"`
	Relations        []RelationInfo    `json:"relations,omitempty"`
	RelationGraph    *RelationGraph    `json:"relation_graph,omitempty"`
	Properties       map[string]interface{} `json:"properties,omitempty"`
}

// TableInfo provides unified table information across all database types
type TableInfo struct {
	Name        string       `json:"name"`
	Type        string       `json:"type"` // "table", "view", "materialized_view"
	RowCount    int64        `json:"row_count,omitempty"`
	SizeBytes   int64        `json:"size_bytes,omitempty"`
	Columns     []ColumnInfo `json:"columns,omitempty"`
	Indexes     []IndexInfo  `json:"indexes,omitempty"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
}

// ColumnInfo provides unified column information
type ColumnInfo struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	Nullable     bool   `json:"nullable"`
	DefaultValue *string `json:"default_value,omitempty"`
	PrimaryKey   bool   `json:"primary_key"`
	Description  string `json:"description,omitempty"`
}

// IndexInfo provides unified index information
type IndexInfo struct {
	Name     string   `json:"name"`
	Columns  []string `json:"columns"`
	Unique   bool     `json:"unique"`
	Primary  bool     `json:"primary"`
	Type     string   `json:"type,omitempty"`
}

// DatasourceInspector provides unified database inspection capabilities
type DatasourceInspector struct {
	db DBConnection
}

// NewDatasourceInspector creates a new datasource inspector
func NewDatasourceInspector(db DBConnection) *DatasourceInspector {
	return &DatasourceInspector{db: db}
}

// InspectDatasource returns comprehensive information about the datasource
func (i *DatasourceInspector) InspectDatasource(ctx context.Context, dbType string) (*DatasourceInfo, error) {
	startTime := time.Now()
	
	info := &DatasourceInfo{
		Type:       dbType,
		Status:     "connected",
		Properties: make(map[string]interface{}),
	}
	
	// Get basic database info
	if err := i.getDatabaseInfo(ctx, info); err != nil {
		info.Status = "error"
		info.Properties["error"] = err.Error()
	}
	
	// Get table list
	tables, err := i.getTables(ctx, info.Type)
	if err != nil {
		info.Properties["tables_error"] = err.Error()
	} else {
		info.Tables = tables
		info.TableCount = len(tables)
	}
	
	info.ConnectionTimeMs = int(time.Since(startTime).Milliseconds())
	
	return info, nil
}

// InspectTable returns detailed information about a specific table
func (i *DatasourceInspector) InspectTable(ctx context.Context, tableName string, includeStats bool) (*TableInfo, error) {
	tableInfo := &TableInfo{
		Name:       tableName,
		Properties: make(map[string]interface{}),
	}
	
	// Get table type
	tableType, err := i.getTableType(ctx, tableName)
	if err == nil {
		tableInfo.Type = tableType
	}
	
	// Get columns
	columns, err := i.getColumns(ctx, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}
	tableInfo.Columns = columns
	
	// Get indexes
	indexes, err := i.getIndexes(ctx, tableName)
	if err == nil {
		tableInfo.Indexes = indexes
	}
	
	// Get table statistics if requested
	if includeStats {
		if err := i.getTableStats(ctx, tableInfo); err != nil {
			tableInfo.Properties["stats_error"] = err.Error()
		}
	}
	
	return tableInfo, nil
}

// getDatabaseInfo retrieves basic database information
func (i *DatasourceInspector) getDatabaseInfo(ctx context.Context, info *DatasourceInfo) error {
	// Get database name - this query should work across most databases
	var databaseName string
	switch info.Type {
	case "postgres", "postgresql":
		row := i.db.QueryRow(ctx, "SELECT current_database()")
		if err := row.Scan(&databaseName); err == nil {
			info.DatabaseName = databaseName
		}
	case "mysql":
		row := i.db.QueryRow(ctx, "SELECT DATABASE()")
		if err := row.Scan(&databaseName); err == nil {
			info.DatabaseName = databaseName
		}
	case "sqlite", "sqlite3":
		// SQLite uses the database file path
		info.DatabaseName = "sqlite_db"
	case "sqlserver", "mssql":
		row := i.db.QueryRow(ctx, "SELECT DB_NAME()")
		if err := row.Scan(&databaseName); err == nil {
			info.DatabaseName = databaseName
		}
	case "oracle":
		row := i.db.QueryRow(ctx, "SELECT USER FROM DUAL")
		if err := row.Scan(&databaseName); err == nil {
			info.DatabaseName = databaseName
		}
	case "trino", "presto":
		row := i.db.QueryRow(ctx, "SELECT CURRENT_CATALOG")
		if err := row.Scan(&databaseName); err == nil {
			info.DatabaseName = databaseName
		}
	case "clickhouse":
		row := i.db.QueryRow(ctx, "SELECT currentDatabase()")
		if err := row.Scan(&databaseName); err == nil {
			info.DatabaseName = databaseName
		}
	}
	
	// Get version info
	if err := i.getVersionInfo(ctx, info); err != nil {
		info.Properties["version_error"] = err.Error()
	}
	
	return nil
}

// getVersionInfo retrieves database version information
func (i *DatasourceInspector) getVersionInfo(ctx context.Context, info *DatasourceInfo) error {
	var version string
	var err error
	
	switch info.Type {
	case "postgres", "postgresql":
		row := i.db.QueryRow(ctx, "SELECT version()")
		err = row.Scan(&version)
	case "mysql":
		row := i.db.QueryRow(ctx, "SELECT VERSION()")
		err = row.Scan(&version)
	case "sqlite", "sqlite3":
		row := i.db.QueryRow(ctx, "SELECT sqlite_version()")
		err = row.Scan(&version)
	case "sqlserver", "mssql":
		row := i.db.QueryRow(ctx, "SELECT @@VERSION")
		err = row.Scan(&version)
	case "oracle":
		row := i.db.QueryRow(ctx, "SELECT * FROM V$VERSION WHERE ROWNUM = 1")
		err = row.Scan(&version)
	case "trino", "presto":
		row := i.db.QueryRow(ctx, "SELECT version()")
		err = row.Scan(&version)
	case "clickhouse":
		row := i.db.QueryRow(ctx, "SELECT version()")
		err = row.Scan(&version)
	}
	
	if err == nil {
		info.Version = version
	}
	
	return nil
}

// getTables retrieves the list of tables/views from the database
func (i *DatasourceInspector) getTables(ctx context.Context, dbType string) ([]TableInfo, error) {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return i.getPostgresTables(ctx)
	case "mysql":
		return i.getMySQLTables(ctx)
	case "sqlite", "sqlite3":
		return i.getSQLiteTables(ctx)
	case "sqlserver", "mssql":
		return i.getSQLServerTables(ctx)
	case "oracle":
		return i.getOracleTables(ctx)
	case "trino", "presto":
		return i.getTrinoTables(ctx)
	case "clickhouse":
		return i.getClickHouseTables(ctx)
	default:
		return i.getGenericTables(ctx)
	}
}

// getPostgresTables retrieves tables from PostgreSQL
func (i *DatasourceInspector) getPostgresTables(ctx context.Context) ([]TableInfo, error) {
	query := `
		SELECT table_name, table_type 
		FROM information_schema.tables 
		WHERE table_schema NOT IN ('information_schema', 'pg_catalog')
		ORDER BY table_name`
	
	rows, err := i.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query postgres tables: %w", err)
	}
	defer rows.Close()
	
	var tables []TableInfo
	for rows.Next() {
		var tableName, tableType string
		if err := rows.Scan(&tableName, &tableType); err != nil {
			continue
		}
		
		tables = append(tables, TableInfo{
			Name: tableName,
			Type: tableType,
		})
	}
	
	return tables, nil
}

// getMySQLTables retrieves tables from MySQL
func (i *DatasourceInspector) getMySQLTables(ctx context.Context) ([]TableInfo, error) {
	query := `
		SELECT table_name, table_type 
		FROM information_schema.tables 
		WHERE table_schema NOT IN ('information_schema', 'performance_schema', 'mysql', 'sys')
		ORDER BY table_name`
	
	rows, err := i.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query mysql tables: %w", err)
	}
	defer rows.Close()
	
	var tables []TableInfo
	for rows.Next() {
		var tableName, tableType string
		if err := rows.Scan(&tableName, &tableType); err != nil {
			continue
		}
		
		tables = append(tables, TableInfo{
			Name: tableName,
			Type: tableType,
		})
	}
	
	return tables, nil
}

// getSQLiteTables retrieves tables from SQLite
func (i *DatasourceInspector) getSQLiteTables(ctx context.Context) ([]TableInfo, error) {
	query := `
		SELECT name, 'table' as table_type 
		FROM sqlite_master 
		WHERE type IN ('table', 'view') AND name NOT LIKE 'sqlite_%'
		ORDER BY name`
	
	rows, err := i.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query sqlite tables: %w", err)
	}
	defer rows.Close()
	
	var tables []TableInfo
	for rows.Next() {
		var tableName, tableType string
		if err := rows.Scan(&tableName, &tableType); err != nil {
			continue
		}
		
		tables = append(tables, TableInfo{
			Name: tableName,
			Type: tableType,
		})
	}
	
	return tables, nil
}

// getTableRelations retrieves foreign key relationships for a specific table
func (i *DatasourceInspector) getTableRelations(ctx context.Context, tableName, dbType string, includeReverse bool) ([]RelationInfo, error) {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return i.getPostgresRelations(ctx, tableName, includeReverse)
	case "mysql":
		return i.getMySQLRelations(ctx, tableName, includeReverse)
	case "sqlite", "sqlite3":
		return i.getSQLiteRelations(ctx, tableName, includeReverse)
	case "sqlserver", "mssql":
		return i.getSQLServerRelations(ctx, tableName, includeReverse)
	case "oracle":
		return i.getOracleRelations(ctx, tableName, includeReverse)
	case "trino", "presto":
		return i.getTrinoRelations(ctx, tableName, includeReverse)
	case "clickhouse":
		return i.getClickHouseRelations(ctx, tableName, includeReverse)
	default:
		return i.getGenericRelations(ctx, tableName, includeReverse)
	}
}

// getAllRelations retrieves all foreign key relationships in database
func (i *DatasourceInspector) getAllRelations(ctx context.Context, dbType string) ([]RelationInfo, error) {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return i.getPostgresAllRelations(ctx)
	case "mysql":
		return i.getMySQLAllRelations(ctx)
	case "sqlite", "sqlite3":
		return i.getSQLiteAllRelations(ctx)
	case "sqlserver", "mssql":
		return i.getSQLServerAllRelations(ctx)
	case "oracle":
		return i.getOracleAllRelations(ctx)
	case "trino", "presto":
		return i.getTrinoAllRelations(ctx)
	case "clickhouse":
		return i.getClickHouseAllRelations(ctx)
	default:
		return i.getGenericAllRelations(ctx)
	}
}

// getPostgresRelations retrieves foreign key relationships from PostgreSQL
func (i *DatasourceInspector) getPostgresRelations(ctx context.Context, tableName string, includeReverse bool) ([]RelationInfo, error) {
	query := `
		SELECT 
			tc.table_name,
			kcu.column_name,
			ccu.table_name AS foreign_table_name,
			ccu.column_name AS foreign_column_name,
			tc.constraint_name,
			rc.delete_rule,
			rc.update_rule
		FROM information_schema.table_constraints AS tc 
		JOIN information_schema.key_column_usage AS kcu
		  ON tc.constraint_name = kcu.constraint_name
		  AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage AS ccu
		  ON ccu.constraint_name = tc.constraint_name
		  AND ccu.table_schema = tc.table_schema
		LEFT JOIN information_schema.referential_constraints rc
		  ON tc.constraint_name = rc.constraint_name
		  AND tc.constraint_schema = rc.constraint_schema
		WHERE tc.constraint_type = 'FOREIGN KEY' 
		  AND tc.table_name = $1
		  AND tc.table_schema NOT IN ('information_schema', 'pg_catalog')`
	
	rows, err := i.db.Query(ctx, query, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query postgres relations: %w", err)
	}
	defer rows.Close()
	
	var relations []RelationInfo
	for rows.Next() {
		var fromTable, fromCol, toTable, toCol, constraintName string
		var onDelete, onUpdate sql.NullString
		
		if err := rows.Scan(&fromTable, &fromCol, &toTable, &toCol, &constraintName, &onDelete, &onUpdate); err != nil {
			continue
		}
		
		relation := RelationInfo{
			FromTable:      fromTable,
			FromColumns:    []string{fromCol},
			ToTable:        toTable,
			ToColumns:      []string{toCol},
			RelationType:   "foreign_key",
			ConstraintName: constraintName,
		}
		
		if onDelete.Valid {
			relation.OnDeleteAction = onDelete.String
		}
		if onUpdate.Valid {
			relation.OnUpdateAction = onUpdate.String
		}
		
		relations = append(relations, relation)
	}
	
	return relations, nil
}

// getPostgresAllRelations retrieves all foreign key relationships from PostgreSQL
func (i *DatasourceInspector) getPostgresAllRelations(ctx context.Context) ([]RelationInfo, error) {
	query := `
		SELECT 
			tc.table_name,
			kcu.column_name,
			ccu.table_name AS foreign_table_name,
			ccu.column_name AS foreign_column_name,
			tc.constraint_name,
			rc.delete_rule,
			rc.update_rule
		FROM information_schema.table_constraints AS tc 
		JOIN information_schema.key_column_usage AS kcu
		  ON tc.constraint_name = kcu.constraint_name
		  AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage AS ccu
		  ON ccu.constraint_name = tc.constraint_name
		  AND ccu.table_schema = tc.table_schema
		LEFT JOIN information_schema.referential_constraints rc
		  ON tc.constraint_name = rc.constraint_name
		  AND tc.constraint_schema = rc.constraint_schema
		WHERE tc.constraint_type = 'FOREIGN KEY' 
		  AND tc.table_schema NOT IN ('information_schema', 'pg_catalog')`
	
	rows, err := i.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query all postgres relations: %w", err)
	}
	defer rows.Close()
	
	var relations []RelationInfo
	for rows.Next() {
		var fromTable, fromCol, toTable, toCol, constraintName string
		var onDelete, onUpdate sql.NullString
		
		if err := rows.Scan(&fromTable, &fromCol, &toTable, &toCol, &constraintName, &onDelete, &onUpdate); err != nil {
			continue
		}
		
		relation := RelationInfo{
			FromTable:      fromTable,
			FromColumns:    []string{fromCol},
			ToTable:        toTable,
			ToColumns:      []string{toCol},
			RelationType:   "foreign_key",
			ConstraintName: constraintName,
		}
		
		if onDelete.Valid {
			relation.OnDeleteAction = onDelete.String
		}
		if onUpdate.Valid {
			relation.OnUpdateAction = onUpdate.String
		}
		
		relations = append(relations, relation)
	}
	
	return relations, nil
}

// getMySQLRelations retrieves foreign key relationships from MySQL
func (i *DatasourceInspector) getMySQLRelations(ctx context.Context, tableName string, includeReverse bool) ([]RelationInfo, error) {
	query := `
		SELECT 
			TABLE_NAME,
			COLUMN_NAME,
			REFERENCED_TABLE_NAME,
			REFERENCED_COLUMN_NAME,
			CONSTRAINT_NAME,
			ON_DELETE,
			ON_UPDATE
		FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE
		WHERE REFERENCED_TABLE_SCHEMA = DATABASE()
		  AND REFERENCED_TABLE_NAME IS NOT NULL
		  AND TABLE_NAME = ?
		  AND REFERENCED_TABLE_SCHEMA = DATABASE()`
	
	rows, err := i.db.Query(ctx, query, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query mysql relations: %w", err)
	}
	defer rows.Close()
	
	var relations []RelationInfo
	for rows.Next() {
		var fromTable, fromCol, toTable, toCol, constraintName string
		var onDelete, onUpdate sql.NullString
		
		if err := rows.Scan(&fromTable, &fromCol, &toTable, &toCol, &constraintName, &onDelete, &onUpdate); err != nil {
			continue
		}
		
		relation := RelationInfo{
			FromTable:      fromTable,
			FromColumns:    []string{fromCol},
			ToTable:        toTable,
			ToColumns:      []string{toCol},
			RelationType:   "foreign_key",
			ConstraintName: constraintName,
		}
		
		if onDelete.Valid {
			relation.OnDeleteAction = onDelete.String
		}
		if onUpdate.Valid {
			relation.OnUpdateAction = onUpdate.String
		}
		
		relations = append(relations, relation)
	}
	
	return relations, nil
}

// getMySQLAllRelations retrieves all foreign key relationships from MySQL
func (i *DatasourceInspector) getMySQLAllRelations(ctx context.Context) ([]RelationInfo, error) {
	query := `
		SELECT 
			TABLE_NAME,
			COLUMN_NAME,
			REFERENCED_TABLE_NAME,
			REFERENCED_COLUMN_NAME,
			CONSTRAINT_NAME,
			ON_DELETE,
			ON_UPDATE
		FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE
		WHERE REFERENCED_TABLE_SCHEMA = DATABASE()
		  AND REFERENCED_TABLE_NAME IS NOT NULL`
	
	rows, err := i.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query all mysql relations: %w", err)
	}
	defer rows.Close()
	
	var relations []RelationInfo
	for rows.Next() {
		var fromTable, fromCol, toTable, toCol, constraintName string
		var onDelete, onUpdate sql.NullString
		
		if err := rows.Scan(&fromTable, &fromCol, &toTable, &toCol, &constraintName, &onDelete, &onUpdate); err != nil {
			continue
		}
		
		relation := RelationInfo{
			FromTable:      fromTable,
			FromColumns:    []string{fromCol},
			ToTable:        toTable,
			ToColumns:      []string{toCol},
			RelationType:   "foreign_key",
			ConstraintName: constraintName,
		}
		
		if onDelete.Valid {
			relation.OnDeleteAction = onDelete.String
		}
		if onUpdate.Valid {
			relation.OnUpdateAction = onUpdate.String
		}
		
		relations = append(relations, relation)
	}
	
	return relations, nil
}

// getSQLiteRelations retrieves foreign key relationships from SQLite
func (i *DatasourceInspector) getSQLiteRelations(ctx context.Context, tableName string, includeReverse bool) ([]RelationInfo, error) {
	query := `PRAGMA foreign_key_list(` + tableName + `)`
	
	rows, err := i.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query sqlite relations: %w", err)
	}
	defer rows.Close()
	
	var relations []RelationInfo
	for rows.Next() {
		var id int
		var seq int
		var table string
		var fromCol string
		var toCol string
		var onUpdate string
		var onDelete string
		var match string
		
		if err := rows.Scan(&id, &seq, &table, &fromCol, &toCol, &onUpdate, &onDelete, &match); err != nil {
			continue
		}
		
		relation := RelationInfo{
			FromTable:      tableName,
			FromColumns:    []string{fromCol},
			ToTable:        table,
			ToColumns:      []string{toCol},
			RelationType:   "foreign_key",
			ConstraintName: fmt.Sprintf("fk_%d", id),
			OnDeleteAction: onDelete,
			OnUpdateAction: onUpdate,
		}
		
		relations = append(relations, relation)
	}
	
	return relations, nil
}

// getSQLiteAllRelations retrieves all foreign key relationships from SQLite
func (i *DatasourceInspector) getSQLiteAllRelations(ctx context.Context) ([]RelationInfo, error) {
	// Get all tables first
	tables, err := i.getSQLiteTables(ctx)
	if err != nil {
		return nil, err
	}
	
	var allRelations []RelationInfo
	for _, table := range tables {
		relations, err := i.getSQLiteRelations(ctx, table.Name, false)
		if err != nil {
			continue
		}
		allRelations = append(allRelations, relations...)
	}
	
	return allRelations, nil
}

// getGenericRelations fallback implementation
func (i *DatasourceInspector) getGenericRelations(ctx context.Context, tableName string, includeReverse bool) ([]RelationInfo, error) {
	query := `
		SELECT 
			tc.table_name,
			kcu.column_name,
			ccu.table_name AS foreign_table_name,
			ccu.column_name AS foreign_column_name,
			tc.constraint_name
		FROM information_schema.table_constraints AS tc 
		JOIN information_schema.key_column_usage AS kcu
		  ON tc.constraint_name = kcu.constraint_name
		  AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage AS ccu
		  ON ccu.constraint_name = tc.constraint_name
		  AND ccu.table_schema = tc.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY' 
		  AND tc.table_name = $1`
	
	rows, err := i.db.Query(ctx, query, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query generic relations: %w", err)
	}
	defer rows.Close()
	
	var relations []RelationInfo
	for rows.Next() {
		var fromTable, fromCol, toTable, toCol, constraintName string
		
		if err := rows.Scan(&fromTable, &fromCol, &toTable, &toCol, &constraintName); err != nil {
			continue
		}
		
		relations = append(relations, RelationInfo{
			FromTable:      fromTable,
			FromColumns:    []string{fromCol},
			ToTable:        toTable,
			ToColumns:      []string{toCol},
			RelationType:   "foreign_key",
			ConstraintName: constraintName,
		})
	}
	
	return relations, nil
}

// getGenericAllRelations fallback implementation
func (i *DatasourceInspector) getGenericAllRelations(ctx context.Context) ([]RelationInfo, error) {
	query := `
		SELECT 
			tc.table_name,
			kcu.column_name,
			ccu.table_name AS foreign_table_name,
			ccu.column_name AS foreign_column_name,
			tc.constraint_name
		FROM information_schema.table_constraints AS tc 
		JOIN information_schema.key_column_usage AS kcu
		  ON tc.constraint_name = kcu.constraint_name
		  AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage AS ccu
		  ON ccu.constraint_name = tc.constraint_name
		  AND ccu.table_schema = tc.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'`
	
	rows, err := i.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query all generic relations: %w", err)
	}
	defer rows.Close()
	
	var relations []RelationInfo
	for rows.Next() {
		var fromTable, fromCol, toTable, toCol, constraintName string
		
		if err := rows.Scan(&fromTable, &fromCol, &toTable, &toCol, &constraintName); err != nil {
			continue
		}
		
		relations = append(relations, RelationInfo{
			FromTable:      fromTable,
			FromColumns:    []string{fromCol},
			ToTable:        toTable,
			ToColumns:      []string{toCol},
			RelationType:   "foreign_key",
			ConstraintName: constraintName,
		})
	}
	
	return relations, nil
}

// buildRelationGraph creates a graph representation of relationships
func (i *DatasourceInspector) buildRelationGraph(ctx context.Context, relations []RelationInfo, depth int) (*RelationGraph, error) {
	graph := &RelationGraph{
		Nodes: []RelationNode{},
		Edges: []RelationEdge{},
	}
	
	// Build table set
	tables := make(map[string]bool)
	for _, rel := range relations {
		tables[rel.FromTable] = true
		tables[rel.ToTable] = true
	}
	
	// Create nodes
	for table := range tables {
		graph.Nodes = append(graph.Nodes, RelationNode{
			Table: table,
			Type:  "table",
		})
	}
	
	// Create edges
	for _, rel := range relations {
		edge := RelationEdge{
			From:    rel.FromTable,
			To:      rel.ToTable,
			Type:    rel.RelationType,
			Columns: rel.FromColumns,
		}
		graph.Edges = append(graph.Edges, edge)
	}
	
	return graph, nil
}

// Placeholder methods for databases not yet fully implemented
func (i *DatasourceInspector) getSQLServerRelations(ctx context.Context, tableName string, includeReverse bool) ([]RelationInfo, error) {
	return i.getGenericRelations(ctx, tableName, includeReverse)
}

func (i *DatasourceInspector) getSQLServerAllRelations(ctx context.Context) ([]RelationInfo, error) {
	return i.getGenericAllRelations(ctx)
}

func (i *DatasourceInspector) getOracleRelations(ctx context.Context, tableName string, includeReverse bool) ([]RelationInfo, error) {
	return i.getGenericRelations(ctx, tableName, includeReverse)
}

func (i *DatasourceInspector) getOracleAllRelations(ctx context.Context) ([]RelationInfo, error) {
	return i.getGenericAllRelations(ctx)
}

func (i *DatasourceInspector) getTrinoRelations(ctx context.Context, tableName string, includeReverse bool) ([]RelationInfo, error) {
	return i.getGenericRelations(ctx, tableName, includeReverse)
}

func (i *DatasourceInspector) getTrinoAllRelations(ctx context.Context) ([]RelationInfo, error) {
	return i.getGenericAllRelations(ctx)
}

func (i *DatasourceInspector) getClickHouseRelations(ctx context.Context, tableName string, includeReverse bool) ([]RelationInfo, error) {
	return i.getGenericRelations(ctx, tableName, includeReverse)
}

func (i *DatasourceInspector) getClickHouseAllRelations(ctx context.Context) ([]RelationInfo, error) {
	return i.getGenericAllRelations(ctx)
}

// getSQLServerTables retrieves tables from SQL Server
func (i *DatasourceInspector) getSQLServerTables(ctx context.Context) ([]TableInfo, error) {
	query := `
		SELECT TABLE_NAME, TABLE_TYPE 
		FROM INFORMATION_SCHEMA.TABLES 
		WHERE TABLE_CATALOG NOT IN ('master', 'tempdb', 'model', 'msdb')
		ORDER BY TABLE_NAME`
	
	rows, err := i.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query sqlserver tables: %w", err)
	}
	defer rows.Close()
	
	var tables []TableInfo
	for rows.Next() {
		var tableName, tableType string
		if err := rows.Scan(&tableName, &tableType); err != nil {
			continue
		}
		
		tables = append(tables, TableInfo{
			Name: tableName,
			Type: tableType,
		})
	}
	
	return tables, nil
}

// getOracleTables retrieves tables from Oracle
func (i *DatasourceInspector) getOracleTables(ctx context.Context) ([]TableInfo, error) {
	query := `
		SELECT OBJECT_NAME, DECODE(OBJECT_TYPE, 'TABLE', 'BASE TABLE', OBJECT_TYPE) 
		FROM ALL_OBJECTS 
		WHERE OWNER = USER AND OBJECT_TYPE IN ('TABLE', 'VIEW')
		ORDER BY OBJECT_NAME`
	
	rows, err := i.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query oracle tables: %w", err)
	}
	defer rows.Close()
	
	var tables []TableInfo
	for rows.Next() {
		var tableName, tableType string
		if err := rows.Scan(&tableName, &tableType); err != nil {
			continue
		}
		
		tables = append(tables, TableInfo{
			Name: tableName,
			Type: tableType,
		})
	}
	
	return tables, nil
}

// getTrinoTables retrieves tables from Trino
func (i *DatasourceInspector) getTrinoTables(ctx context.Context) ([]TableInfo, error) {
	query := `SELECT table_name, 'table' FROM information_schema.tables ORDER BY table_name`
	
	rows, err := i.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query trino tables: %w", err)
	}
	defer rows.Close()
	
	var tables []TableInfo
	for rows.Next() {
		var tableName, tableType string
		if err := rows.Scan(&tableName, &tableType); err != nil {
			continue
		}
		
		tables = append(tables, TableInfo{
			Name: tableName,
			Type: tableType,
		})
	}
	
	return tables, nil
}

// getClickHouseTables retrieves tables from ClickHouse
func (i *DatasourceInspector) getClickHouseTables(ctx context.Context) ([]TableInfo, error) {
	query := `SELECT name, engine FROM system.tables ORDER BY name`
	
	rows, err := i.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query clickhouse tables: %w", err)
	}
	defer rows.Close()
	
	var tables []TableInfo
	for rows.Next() {
		var tableName, engine string
		if err := rows.Scan(&tableName, &engine); err != nil {
			continue
		}
		
		tables = append(tables, TableInfo{
			Name: tableName,
			Type: engine, // ClickHouse uses engine as table type
		})
	}
	
	return tables, nil
}

// getGenericTables retrieves tables using generic SQL approach
func (i *DatasourceInspector) getGenericTables(ctx context.Context) ([]TableInfo, error) {
	query := `
		SELECT table_name, table_type 
		FROM information_schema.tables 
		ORDER BY table_name`
	
	rows, err := i.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query generic tables: %w", err)
	}
	defer rows.Close()
	
	var tables []TableInfo
	for rows.Next() {
		var tableName, tableType string
		if err := rows.Scan(&tableName, &tableType); err != nil {
			continue
		}
		
		tables = append(tables, TableInfo{
			Name: tableName,
			Type: tableType,
		})
	}
	
	return tables, nil
}

// detectDatabaseType tries to detect the database type from the connection
func (i *DatasourceInspector) detectDatabaseType(ctx context.Context) string {
	// Try to detect database type by running version queries
	if row := i.db.QueryRow(ctx, "SELECT version()"); row != nil {
		var version string
		if row.Scan(&version) == nil {
			if strings.Contains(strings.ToLower(version), "postgresql") {
				return "postgresql"
			} else if strings.Contains(strings.ToLower(version), "mysql") {
				return "mysql"
			} else if strings.Contains(strings.ToLower(version), "clickhouse") {
				return "clickhouse"
			}
		}
	}
	
	// Try SQLite detection
	if row := i.db.QueryRow(ctx, "SELECT sqlite_version()"); row != nil {
		var version string
		if row.Scan(&version) == nil {
			return "sqlite"
		}
	}
	
	// Default to generic SQL
	return "sql"
}

// getTableType determines if a table is actually a table, view, etc.
func (i *DatasourceInspector) getTableType(ctx context.Context, tableName string) (string, error) {
	dbType := i.detectDatabaseType(ctx)
	
	var query string
	switch dbType {
	case "postgres", "postgresql":
		query = `
			SELECT table_type 
			FROM information_schema.tables 
			WHERE table_name = $1 AND table_schema NOT IN ('information_schema', 'pg_catalog')`
	case "mysql":
		query = `
			SELECT table_type 
			FROM information_schema.tables 
			WHERE table_name = ? AND table_schema NOT IN ('information_schema', 'performance_schema', 'mysql', 'sys')`
	case "sqlite", "sqlite3":
		query = `SELECT type FROM sqlite_master WHERE name = ?`
	default:
		query = `
			SELECT table_type 
			FROM information_schema.tables 
			WHERE table_name = ?`
	}
	
	row := i.db.QueryRow(ctx, query, tableName)
	if row == nil {
		return "table", nil // Default
	}
	
	var tableType string
	err := row.Scan(&tableType)
	if err != nil {
		return "table", nil // Default on error
	}
	
	return tableType, nil
}

// getColumns retrieves column information for a table
func (i *DatasourceInspector) getColumns(ctx context.Context, tableName string) ([]ColumnInfo, error) {
	dbType := i.detectDatabaseType(ctx)
	
	var query string
	switch dbType {
	case "postgres", "postgresql":
		query = `
			SELECT column_name, data_type, is_nullable, column_default
			FROM information_schema.columns 
			WHERE table_name = $1 AND table_schema NOT IN ('information_schema', 'pg_catalog')
			ORDER BY ordinal_position`
	case "mysql":
		query = `
			SELECT column_name, data_type, is_nullable, column_default
			FROM information_schema.columns 
			WHERE table_name = ? AND table_schema NOT IN ('information_schema', 'performance_schema', 'mysql', 'sys')
			ORDER BY ordinal_position`
	case "sqlite", "sqlite3":
		query = `PRAGMA table_info(` + tableName + `)`
	case "sqlserver", "mssql":
		query = `
			SELECT COLUMN_NAME, DATA_TYPE, IS_NULLABLE, COLUMN_DEFAULT
			FROM INFORMATION_SCHEMA.COLUMNS 
			WHERE TABLE_NAME = ?
			ORDER BY ORDINAL_POSITION`
	default:
		query = `
			SELECT column_name, data_type, is_nullable, column_default
			FROM information_schema.columns 
			WHERE table_name = ?
			ORDER BY ordinal_position`
	}
	
	rows, err := i.db.Query(ctx, query, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query columns: %w", err)
	}
	defer rows.Close()
	
	var columns []ColumnInfo
	for rows.Next() {
		var col ColumnInfo
		
		if dbType == "sqlite" || dbType == "sqlite3" {
			// SQLite uses different PRAGMA format
			var cid int
			var name, dataType string
			var notNull int
			var dfltValue *string
			var pk int
			
			if err := rows.Scan(&cid, &name, &dataType, &notNull, &dfltValue, &pk); err != nil {
				continue
			}
			
			col = ColumnInfo{
				Name:         name,
				Type:         dataType,
				Nullable:     notNull == 0,
				DefaultValue: dfltValue,
				PrimaryKey:   pk == 1,
			}
		} else {
			var name, dataType, nullable string
			var dfltValue *string
			
			if err := rows.Scan(&name, &dataType, &nullable, &dfltValue); err != nil {
				continue
			}
			
			col = ColumnInfo{
				Name:         name,
				Type:         dataType,
				Nullable:     nullable == "YES",
				DefaultValue: dfltValue,
				PrimaryKey:   false, // Will be detected from indexes
			}
		}
		
		columns = append(columns, col)
	}
	
	// Get primary key information
	indexes, err := i.getIndexes(ctx, tableName)
	if err == nil {
		for _, index := range indexes {
			if index.Primary {
				for _, colName := range index.Columns {
					for i, col := range columns {
						if col.Name == colName {
							columns[i].PrimaryKey = true
							break
						}
					}
				}
			}
		}
	}
	
	return columns, nil
}

// getIndexes retrieves index information for a table
func (i *DatasourceInspector) getIndexes(ctx context.Context, tableName string) ([]IndexInfo, error) {
	dbType := i.detectDatabaseType(ctx)
	
	var query string
	switch dbType {
	case "postgres", "postgresql":
		query = `
			SELECT 
				i.relname as index_name,
				array_agg(a.attname ORDER BY c.ordinality) as columns,
				i.indisunique as unique,
				i.indisprimary as primary
			FROM pg_index i
			JOIN pg_class t ON i.indrelid = t.oid
			JOIN pg_class i_rel ON i.indexrelid = i_rel.oid
			JOIN unnest(i.indkey) WITH ORDINALITY c(colnum, ordinality) ON true
			JOIN pg_attribute a ON a.attnum = c.colnum AND a.attrelid = t.oid
			WHERE t.relname = $1
			GROUP BY i.indexrelid, i.relname, i.indisunique, i.indisprimary`
	case "mysql":
		query = `
			SELECT 
				INDEX_NAME as index_name,
				GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX) as columns,
				NON_UNIQUE = 0 as unique,
				INDEX_NAME = 'PRIMARY' as primary
			FROM INFORMATION_SCHEMA.STATISTICS 
			WHERE TABLE_NAME = ?
			GROUP BY INDEX_NAME, NON_UNIQUE`
	case "sqlite", "sqlite3":
		query = `PRAGMA index_list(` + tableName + `)`
	default:
		// Generic approach for other databases
		query = `
			SELECT 
				INDEX_NAME,
				COLUMN_NAME,
				UNIQUE,
				PRIMARY_KEY
			FROM INFORMATION_SCHEMA.INDEXES 
			WHERE TABLE_NAME = ?`
	}
	
	if dbType == "sqlite" || dbType == "sqlite3" {
		// Handle SQLite PRAGMA index_list
		rows, err := i.db.Query(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("failed to query indexes: %w", err)
		}
		defer rows.Close()
		
		var indexes []IndexInfo
		for rows.Next() {
			var seq int
			var name string
			var unique int
			var origin string
			var partial int
			
			if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
				continue
			}
			
			// Get columns for this index
			columnQuery := `PRAGMA index_info(` + name + `)`
			colRows, err := i.db.Query(ctx, columnQuery)
			if err != nil {
				continue
			}
			
			var columns []string
			for colRows.Next() {
				var cid, nid int
				var colName string
				if colRows.Scan(&cid, &colName, &nid) == nil {
					columns = append(columns, colName)
				}
			}
			colRows.Close()
			
			indexes = append(indexes, IndexInfo{
				Name:    name,
				Columns: columns,
				Unique:  unique != 0,
				Primary: name == "sqlite_autoindex_" + tableName + "_1",
			})
		}
		
		return indexes, nil
	}
	
	// Handle standard SQL databases
	if strings.Contains(strings.ToUpper(query), "GROUP_CONCAT") {
		// MySQL format
		rows, err := i.db.Query(ctx, query, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to query indexes: %w", err)
		}
		defer rows.Close()
		
		var indexes []IndexInfo
		for rows.Next() {
			var name, columnsStr string
			var unique, primary bool
			
			if err := rows.Scan(&name, &columnsStr, &unique, &primary); err != nil {
				continue
			}
			
			columns := strings.Split(columnsStr, ",")
			for i, col := range columns {
				columns[i] = strings.TrimSpace(col)
			}
			
			indexes = append(indexes, IndexInfo{
				Name:    name,
				Columns: columns,
				Unique:  unique,
				Primary: primary,
			})
		}
		
		return indexes, nil
	}
	
	// Generic fallback
	rows, err := i.db.Query(ctx, query, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query indexes: %w", err)
	}
	defer rows.Close()
	
	// For now, return empty indexes for other database types
	return []IndexInfo{}, nil
}

// getTableStats retrieves table statistics like row count and size
func (i *DatasourceInspector) getTableStats(ctx context.Context, tableInfo *TableInfo) error {
	dbType := i.detectDatabaseType(ctx)
	
	// Get row count
	var countQuery string
	switch dbType {
	case "postgres", "postgresql":
		countQuery = `SELECT COUNT(*) FROM "` + tableInfo.Name + `"`
	case "mysql":
		countQuery = `SELECT COUNT(*) FROM ` + tableInfo.Name
	case "sqlite", "sqlite3":
		countQuery = `SELECT COUNT(*) FROM ` + tableInfo.Name
	case "sqlserver", "mssql":
		countQuery = `SELECT COUNT(*) FROM ` + tableInfo.Name
	default:
		countQuery = `SELECT COUNT(*) FROM ` + tableInfo.Name
	}
	
	row := i.db.QueryRow(ctx, countQuery)
	if row != nil {
		var count int64
		if err := row.Scan(&count); err == nil {
			tableInfo.RowCount = count
		}
	}
	
	// Get table size (if supported)
	if dbType == "postgres" || dbType == "postgresql" {
		sizeQuery := `
			SELECT pg_total_relation_size('"' || $1 || '"')`
		sizeRow := i.db.QueryRow(ctx, sizeQuery, tableInfo.Name)
		if sizeRow != nil {
			var size int64
		if sizeRow.Scan(&size) == nil {
				tableInfo.SizeBytes = size
			}
		}
	}
	
	return nil
}

// DatasourceInspectTool inspects database schemas and metadata
type DatasourceInspectTool struct {
	zdb *db.Database
}

// NewDatasourceInspectTool creates a new datasource inspection tool
func NewDatasourceInspectTool(zdb *db.Database) *DatasourceInspectTool {
	return &DatasourceInspectTool{
		zdb: zdb,
	}
}

// Name returns tool name
func (t *DatasourceInspectTool) Name() string {
	return "datasource_inspect"
}

// Description returns tool description
func (t *DatasourceInspectTool) Description() string {
	return "Inspect database schemas, tables, columns, and metadata. Works with all supported database types for unified data discovery."
}

// Parameters returns tool parameters
func (t *DatasourceInspectTool) Parameters() map[string]ToolParameter {
	return map[string]ToolParameter{
		"datasource_id": {
			Type:        "string",
			Description: "ID of the datasource to inspect (optional, inspects default if not provided)",
			Required:    false,
		},
		"table_name": {
			Type:        "string",
			Description: "Specific table to inspect (optional, lists all tables if not provided)",
			Required:    false,
		},
		"include_stats": {
			Type:        "boolean",
			Description: "Include table statistics like row count and size (default: false)",
			Required:    false,
			Default:     false,
		},
		"include_columns": {
			Type:        "boolean",
			Description: "Include detailed column information (default: true)",
			Required:    false,
			Default:     true,
		},
		"include_indexes": {
			Type:        "boolean",
			Description: "Include index information (default: true)",
			Required:    false,
			Default:     true,
		},
		"include_relations": {
			Type:        "boolean",
			Description: "Include foreign key relationships and dependencies (default: false)",
			Required:    false,
			Default:     false,
		},
		"relations_depth": {
			Type:        "number",
			Description: "How deep to follow relation chains (default: 1, max: 3)",
			Required:    false,
			Default:     1,
		},
		"include_reverse_relations": {
			Type:        "boolean",
			Description: "Include reverse references (tables that reference this table) (default: true)",
			Required:    false,
			Default:     true,
		},
	}
}

// ValidateAccess checks if user has access to this tool
func (t *DatasourceInspectTool) ValidateAccess(userID, projectID string) bool {
	return true
}

// GetCategory returns the tool category
func (t *DatasourceInspectTool) GetCategory() string {
	return "database"
}

// Execute runs the datasource inspection
func (t *DatasourceInspectTool) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	startTime := time.Now()

	// Get parameters
	datasourceID, _ := params["datasource_id"].(string)
	tableName, _ := params["table_name"].(string)
	includeStats, hasStats := params["include_stats"].(bool)
	if !hasStats {
		includeStats = false // Default to false
	}
	
	includeColumns, hasColumns := params["include_columns"].(bool)
	if !hasColumns {
		includeColumns = true // Default to true
	}
	
	includeIndexes, hasIndexes := params["include_indexes"].(bool)
	if !hasIndexes {
		includeIndexes = true // Default to true
	}
	
	includeRelations, hasRelations := params["include_relations"].(bool)
	if !hasRelations {
		includeRelations = false // Default to false
	}
	
	relationsDepth := 1 // Default
	if depth, hasDepth := params["relations_depth"].(float64); hasDepth {
		if depth < 1 {
			relationsDepth = 1
		} else if depth > 3 {
			relationsDepth = 3
		} else {
			relationsDepth = int(depth)
		}
	}
	
	includeReverseRelations, hasReverse := params["include_reverse_relations"].(bool)
	if !hasReverse {
		includeReverseRelations = true // Default to true
	}

	// Create context with timeout
	inspectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Get datasource connection
	dbConn, err := t.getDatasourceConnection(inspectCtx, datasourceID)
	if err != nil {
		return NewToolError("Failed to get datasource connection", err), nil
	}

	// Create inspector
	inspector := NewDatasourceInspector(dbConn)

	// Get datasource type for inspection
	datasourceType, err := t.getDatasourceType(inspectCtx, datasourceID)
	if err != nil {
		return NewToolError("Failed to determine datasource type", err), nil
	}

	if tableName != "" {
		// Inspect specific table
		tableInfo, err := inspector.InspectTable(inspectCtx, tableName, includeStats)
		if err != nil {
			return NewToolError("Failed to inspect table", err), nil
		}

		// Optionally filter columns/indexes based on parameters
		if !includeColumns {
			tableInfo.Columns = nil
		}
		if !includeIndexes {
			tableInfo.Indexes = nil
		}

		result := map[string]interface{}{
			"datasource_id":   datasourceID,
			"datasource_type": datasourceType,
			"table":          tableInfo,
		}

		// Add relations if requested
		if includeRelations {
			relations, err := inspector.getTableRelations(inspectCtx, tableName, datasourceType, includeReverseRelations)
			if err != nil {
				result["relations_error"] = err.Error()
			} else {
				result["relations"] = relations
			}
		}

		return NewToolSuccess(result, int(time.Since(startTime).Milliseconds())), nil
	} else {
		// Inspect entire datasource
		datasourceInfo, err := inspector.InspectDatasource(inspectCtx, datasourceType)
		if err != nil {
			return NewToolError("Failed to inspect datasource", err), nil
		}

		// Get detailed table information if requested
		if includeColumns || includeIndexes || includeStats {
			for i, table := range datasourceInfo.Tables {
				tableInfo, err := inspector.InspectTable(inspectCtx, table.Name, includeStats)
				if err != nil {
					table.Properties["inspection_error"] = err.Error()
					continue
				}

				// Filter based on parameters
				if !includeColumns {
					tableInfo.Columns = nil
				}
				if !includeIndexes {
					tableInfo.Indexes = nil
				}

				datasourceInfo.Tables[i] = *tableInfo
			}
		}

		// Add relations if requested
		if includeRelations {
			relations, err := inspector.getAllRelations(inspectCtx, datasourceType)
			if err != nil {
				datasourceInfo.Properties["relations_error"] = err.Error()
			} else {
				datasourceInfo.Relations = relations

				// Build relation graph if depth > 1
				if relationsDepth > 1 {
					graph, err := inspector.buildRelationGraph(inspectCtx, relations, relationsDepth)
					if err != nil {
						datasourceInfo.Properties["graph_error"] = err.Error()
					} else {
						datasourceInfo.RelationGraph = graph
					}
				}
			}
		}

		return NewToolSuccess(map[string]interface{}{
			"datasource_id": datasourceID,
			"datasource":   datasourceInfo,
		}, int(time.Since(startTime).Milliseconds())), nil
	}
}

func (t *DatasourceInspectTool) getDatasourceConnection(ctx context.Context, datasourceID string) (DBConnection, error) {
	// Reuse database tool's connection logic
	dbTool := &DatabaseQueryTool{zdb: t.zdb}
	return dbTool.getDatasourceConnection(ctx, datasourceID)
}

func (t *DatasourceInspectTool) getDatasourceType(ctx context.Context, datasourceID string) (string, error) {
	// If no datasource ID, default to postgres
	if datasourceID == "" {
		return "postgresql", nil
	}

	// Get datasource type from database
	row, err := t.zdb.QueryRow(ctx,
		`SELECT d.type FROM datasources d 
		 JOIN projects p ON d.project_id = p.id 
		 WHERE d.id = $1 AND d.is_active = true AND p.is_active = true`,
		datasourceID)
	if err != nil {
		return "", fmt.Errorf("failed to fetch datasource: %w", err)
	}

	if len(row.Values) == 0 {
		return "", fmt.Errorf("datasource not found or not accessible")
	}

	dsType, ok := row.Values[0].AsString()
	if !ok {
		return "", fmt.Errorf("invalid datasource type")
	}

	return dsType, nil
}

// ZlayDBAdapter adapts zlay-db Database to DBConnection interface
type ZlayDBAdapter struct {
	DB *db.Database
}

// RelationInfo represents a database relationship
type RelationInfo struct {
	FromTable      string   `json:"from_table"`
	FromColumns    []string `json:"from_columns"`
	ToTable        string   `json:"to_table"`
	ToColumns      []string `json:"to_columns"`
	RelationType   string   `json:"relation_type"`
	ConstraintName string   `json:"constraint_name"`
	OnDeleteAction string   `json:"on_delete_action,omitempty"`
	OnUpdateAction string   `json:"on_update_action,omitempty"`
}

// RelationGraph represents relationships as a graph
type RelationGraph struct {
	Nodes []RelationNode `json:"nodes"`
	Edges []RelationEdge `json:"edges"`
}

// RelationNode represents a table in the relation graph
type RelationNode struct {
	Table    string `json:"table"`
	Type     string `json:"type"`
	Columns  []string `json:"columns,omitempty"`
}

// RelationEdge represents a relationship between tables
type RelationEdge struct {
	From    string   `json:"from"`
	To      string   `json:"to"`
	Type     string   `json:"type"`
	Columns  []string `json:"columns"`
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
