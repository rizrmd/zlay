package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL pgx/v5 driver
	_ "github.com/lib/pq"            // PostgreSQL legacy (keep for compatibility)
	_ "github.com/go-sql-driver/mysql" // MySQL
	_ "github.com/mattn/go-sqlite3" // SQLite
)

// ConnectionConfig represents database connection configuration
type ConnectionConfig struct {
	DatabaseType DatabaseType

	// Either ConnectionString OR specific fields
	ConnectionString string

	// Database-specific fields
	Host     string
	Port     int
	Database string
	Username string
	Password string
	SSLMode  string

	// File-based databases
	FilePath string

	// Trino-specific fields
	Catalog    string // Trino catalog
	Schema     string // Trino schema
	ServerURL  string // Trino server URL

	// Pool configuration
	PoolSize       int
	MaxConnections int
	TimeoutMs      int
	IdleTimeoutMs  int
}

// Database represents a database connection
type Database struct {
	db     *sql.DB
	config ConnectionConfig
	trinoAdapter *TrinoAdapter
}

// ConnectionBuilder provides a fluent interface for building connections
type ConnectionBuilder struct {
	config ConnectionConfig
}

// NewConnectionBuilder creates a new connection builder
func NewConnectionBuilder(dbType DatabaseType) *ConnectionBuilder {
	return &ConnectionBuilder{
		config: ConnectionConfig{
			DatabaseType:  dbType,
			PoolSize:      10,
			MaxConnections: 100,
			TimeoutMs:      30000,
			IdleTimeoutMs:  300000,
		},
	}
}

// ConnectionString sets the connection string
func (cb *ConnectionBuilder) ConnectionString(connStr string) *ConnectionBuilder {
	cb.config.ConnectionString = connStr
	return cb
}

// Host sets the database host
func (cb *ConnectionBuilder) Host(host string) *ConnectionBuilder {
	cb.config.Host = host
	return cb
}

// Port sets the database port
func (cb *ConnectionBuilder) Port(port int) *ConnectionBuilder {
	cb.config.Port = port
	return cb
}

// Database sets the database name
func (cb *ConnectionBuilder) Database(database string) *ConnectionBuilder {
	cb.config.Database = database
	return cb
}

// Username sets the database username
func (cb *ConnectionBuilder) Username(username string) *ConnectionBuilder {
	cb.config.Username = username
	return cb
}

// Password sets the database password
func (cb *ConnectionBuilder) Password(password string) *ConnectionBuilder {
	cb.config.Password = password
	return cb
}

// SSLMode sets SSL mode
func (cb *ConnectionBuilder) SSLMode(sslMode string) *ConnectionBuilder {
	cb.config.SSLMode = sslMode
	return cb
}

// FilePath sets the file path for file-based databases
func (cb *ConnectionBuilder) FilePath(filePath string) *ConnectionBuilder {
	cb.config.FilePath = filePath
	return cb
}

// PoolSize sets the connection pool size
func (cb *ConnectionBuilder) PoolSize(size int) *ConnectionBuilder {
	cb.config.PoolSize = size
	return cb
}

// MaxConnections sets the maximum number of connections
func (cb *ConnectionBuilder) MaxConnections(max int) *ConnectionBuilder {
	cb.config.MaxConnections = max
	return cb
}

// Timeout sets the connection timeout in milliseconds
func (cb *ConnectionBuilder) Timeout(timeout int) *ConnectionBuilder {
	cb.config.TimeoutMs = timeout
	return cb
}

// Build creates and returns a database connection
func (cb *ConnectionBuilder) Build() (*Database, error) {
	return Connect(cb.config)
}

// Connect creates a new database connection using the provided configuration
func Connect(config ConnectionConfig) (*Database, error) {
	var dsn string
	var driverName string

	if config.ConnectionString != "" {
		// Try to detect database type from connection string
		if strings.Contains(config.ConnectionString, "postgres://") {
			driverName = "pgx"
			// pgx/v5 uses the connection string directly
			dsn = config.ConnectionString
		} else if strings.Contains(config.ConnectionString, "mysql://") {
			driverName = "mysql"
			dsn = config.ConnectionString
		} else if strings.Contains(config.ConnectionString, ".db") || strings.Contains(config.ConnectionString, ".sqlite") {
			driverName = "sqlite3"
			dsn = config.ConnectionString
		} else if strings.Contains(config.ConnectionString, "trino") || strings.Contains(config.ConnectionString, "presto") {
			driverName = "trino"
			dsn = config.ConnectionString
		} else {
			driverName = "pgx" // Default to pgx for PostgreSQL
			// pgx/v5 uses connection string directly  
			dsn = config.ConnectionString
		}
	} else {
		switch config.DatabaseType {
		case DatabaseTypePostgreSQL:
			dsn = buildPostgreSQLDSN(config)
			driverName = "pgx"
		case DatabaseTypeMySQL:
			dsn = buildMySQLDSN(config)
			driverName = "mysql"
		case DatabaseTypeSQLite:
			dsn = config.FilePath
			driverName = "sqlite3"
		case DatabaseTypeTrino:
			dsn = buildTrinoDSN(config)
			driverName = "trino"
		default:
			return nil, fmt.Errorf("unsupported database type: %s", config.DatabaseType)
		}
	}

	if config.DatabaseType == DatabaseTypeTrino {
		// Use Trino adapter
		trinoAdapter := NewTrinoAdapter(dsn, config.Username, config.Password, config.Catalog, config.Schema)
		
		// Test the connection
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.TimeoutMs)*time.Millisecond)
		defer cancel()

		if err := trinoAdapter.TestConnection(ctx); err != nil {
			return nil, fmt.Errorf("failed to connect to Trino: %w", err)
		}

		return &Database{
			db:           nil, // No standard sql.DB for Trino
			config:        config,
			trinoAdapter: trinoAdapter,
		}, nil
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.TimeoutMs)*time.Millisecond)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(config.MaxConnections)
	db.SetMaxIdleConns(config.PoolSize)
	db.SetConnMaxLifetime(time.Duration(config.IdleTimeoutMs) * time.Millisecond)

	return &Database{
		db:     db,
		config: config,
	}, nil
}

// buildPostgreSQLDSN builds PostgreSQL connection string
func buildPostgreSQLDSN(config ConnectionConfig) string {
	port := config.Port
	if port == 0 {
		port = 5432
	}

	dsn := fmt.Sprintf("host=%s port=%d user=%s", config.Host, port, config.Username)
	if config.Password != "" {
		dsn += fmt.Sprintf(" password=%s", config.Password)
	}
	if config.Database != "" {
		dsn += fmt.Sprintf(" dbname=%s", config.Database)
	}
	if config.SSLMode != "" {
		dsn += fmt.Sprintf(" sslmode=%s", config.SSLMode)
	} else {
		dsn += " sslmode=disable"
	}
	return dsn
}

// buildTrinoDSN builds Trino connection configuration
func buildTrinoDSN(config ConnectionConfig) string {
	// Use server URL if provided
	if config.ServerURL != "" {
		return config.ServerURL
	}

	// Build from individual components
	port := config.Port
	if port == 0 {
		port = 8080 // Default Trino port
	}

	serverURL := fmt.Sprintf("http://%s:%d", config.Host, port)
	
	// Add catalog and schema if provided
	if config.Catalog != "" {
		serverURL += fmt.Sprintf("?catalog=%s", config.Catalog)
		if config.Schema != "" {
			serverURL += fmt.Sprintf("&schema=%s", config.Schema)
		}
	}
	
	return serverURL
}



// buildMySQLDSN builds MySQL connection string
func buildMySQLDSN(config ConnectionConfig) string {
	port := config.Port
	if port == 0 {
		port = 3306
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)", config.Username, config.Password, config.Host, port)
	if config.Database != "" {
		dsn += fmt.Sprintf("/%s", config.Database)
	}
	
	return dsn + "?parseTime=true&loc=Local"
}

// Close closes the database connection
func (db *Database) Close() error {
	return db.db.Close()
}

// GetDB returns the underlying *sql.DB instance
func (db *Database) GetDB() *sql.DB {
	return db.db
}

// GetConfig returns the connection configuration
func (db *Database) GetConfig() ConnectionConfig {
	return db.config
}

// Begin starts a new transaction
func (db *Database) Begin() (*Transaction, error) {
	tx, err := db.db.Begin()
	if err != nil {
		return nil, err
	}
	return &Transaction{
		tx: tx,
		db: db,
	}, nil
}

// BeginTx starts a new transaction with the given options
func (db *Database) BeginTx(ctx context.Context, opts *sql.TxOptions) (*Transaction, error) {
	tx, err := db.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &Transaction{
		tx: tx,
		db: db,
	}, nil
}
