package db

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

// Pool represents a connection pool
type Pool struct {
	db     *sql.DB
	config ConnectionConfig
	mu     sync.RWMutex
}

// NewPool creates a new connection pool
func NewPool(config ConnectionConfig) (*Pool, error) {
	db, err := Connect(config)
	if err != nil {
		return nil, err
	}

	// Configure connection pool based on database type
	maxOpenConns := config.MaxConnections
	maxIdleConns := config.PoolSize

	// Adjust pool settings based on database type
	switch config.DatabaseType {
	case DatabaseTypeSQLite:
		// SQLite doesn't benefit from connection pooling
		maxOpenConns = 1
		maxIdleConns = 1
	case DatabaseTypeCSV, DatabaseTypeExcel:
		// File-based databases don't need pooling
		maxOpenConns = 1
		maxIdleConns = 1
	}

	sqlDB := db.GetDB()
	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(config.IdleTimeoutMs) * time.Millisecond)

	return &Pool{
		db:     sqlDB,
		config: config,
	}, nil
}

// GetConnection gets a connection from the pool
func (pool *Pool) GetConnection() (*Database, error) {
	return &Database{
		db:     pool.db,
		config: pool.config,
	}, nil
}

// GetConnectionWithTimeout gets a connection with timeout
func (pool *Pool) GetConnectionWithTimeout(timeout time.Duration) (*Database, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Test connection health
	if err := pool.db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to get healthy connection: %w", err)
	}

	return &Database{
		db:     pool.db,
		config: pool.config,
	}, nil
}

// Close closes all connections in the pool
func (pool *Pool) Close() error {
	return pool.db.Close()
}

// Stats returns connection pool statistics
func (pool *Pool) Stats() sql.DBStats {
	return pool.db.Stats()
}

// Ping tests the connection health
func (pool *Pool) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return pool.db.PingContext(ctx)
}

// Config returns the pool configuration
func (pool *Pool) Config() ConnectionConfig {
	return pool.config
}

// Database represents a pooled database connection
type PooledDatabase struct {
	pool *Pool
	db   *Database
}

// NewPooledDatabase creates a new pooled database connection
func NewPooledDatabase(pool *Pool) *PooledDatabase {
	db, _ := pool.GetConnection()
	return &PooledDatabase{
		pool: pool,
		db:   db,
	}
}

// Execute executes a non-query SQL statement
func (pdb *PooledDatabase) Execute(ctx context.Context, query string, args ...interface{}) (*Result, error) {
	return pdb.db.Execute(ctx, query, args...)
}

// Query executes a query and returns result set
func (pdb *PooledDatabase) Query(ctx context.Context, query string, args ...interface{}) (*ResultSet, error) {
	return pdb.db.Query(ctx, query, args...)
}

// QueryRow executes a query that returns a single row
func (pdb *PooledDatabase) QueryRow(ctx context.Context, query string, args ...interface{}) (*Row, error) {
	return pdb.db.QueryRow(ctx, query, args...)
}

// Begin starts a new transaction
func (pdb *PooledDatabase) Begin() (*Transaction, error) {
	return pdb.db.Begin()
}

// BeginTx starts a new transaction with the given options
func (pdb *PooledDatabase) BeginTx(ctx context.Context, opts *sql.TxOptions) (*Transaction, error) {
	return pdb.db.BeginTx(ctx, opts)
}

// Close returns the connection to the pool
func (pdb *PooledDatabase) Close() error {
	// For pooled connections, we don't actually close the underlying connection
	// The connection is returned to the pool automatically
	pdb.db = nil
	return nil
}

// GetPool returns the underlying pool
func (pdb *PooledDatabase) GetPool() *Pool {
	return pdb.pool
}
