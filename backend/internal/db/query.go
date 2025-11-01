package db

import (
	"context"
	"fmt"
)

// Execute executes a non-query SQL statement
func (db *Database) Execute(ctx context.Context, query string, args ...interface{}) (*Result, error) {
	if db.trinoAdapter != nil {
		return db.trinoAdapter.Execute(ctx, query, args...)
	}

	result, err := db.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	rowsAffected, _ := result.RowsAffected()
	lastInsertID, _ := result.LastInsertId()

	return &Result{
		RowsAffected: rowsAffected,
		LastInsertID: lastInsertID,
	}, nil
}

// Query executes a query and returns result set
func (db *Database) Query(ctx context.Context, query string, args ...interface{}) (*ResultSet, error) {
	if db.trinoAdapter != nil {
		return db.trinoAdapter.Query(ctx, query, args...)
	}

	rows, err := db.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return ConvertSQLRowToResultSet(rows)
}

// QueryRow executes a query that returns a single row
func (db *Database) QueryRow(ctx context.Context, query string, args ...interface{}) (*Row, error) {
	if db.trinoAdapter != nil {
		return db.trinoAdapter.QueryRow(ctx, query, args...)
	}

	// Execute the query with regular Query to get column information
	rows, err := db.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Get column types
	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}

	// Check if we have any rows
	if !rows.Next() {
		return nil, fmt.Errorf("no rows found")
	}

	// Prepare scan values
	columns := len(columnTypes)
	values := make([]interface{}, columns)
	valuePtrs := make([]interface{}, columns)
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	// Scan the row
	if err := rows.Scan(valuePtrs...); err != nil {
		return nil, err
	}

	// Convert to our Row format
	rowValues := make([]Value, columns)
	for i, val := range values {
		dbType := columnTypes[i].DatabaseTypeName()
		expectedType := mapSQLTypeToValueType(dbType)

		rowValues[i] = convertSQLValueToValue(val, expectedType)
	}

	return &Row{Values: rowValues}, nil
}

// QueryBuilder provides a fluent interface for building queries
type QueryBuilder struct {
	db      *Database
	query   string
	args    []interface{}
	err     error
	ctx     context.Context
}

// NewQueryBuilder creates a new query builder
func (db *Database) NewQueryBuilder() *QueryBuilder {
	return &QueryBuilder{
		db:  db,
		ctx: context.Background(),
	}
}

// WithContext sets the context for the query
func (qb *QueryBuilder) WithContext(ctx context.Context) *QueryBuilder {
	qb.ctx = ctx
	return qb
}

// Query sets the query string
func (qb *QueryBuilder) Query(query string) *QueryBuilder {
	qb.query = query
	return qb
}

// Args sets the query arguments
func (qb *QueryBuilder) Args(args ...interface{}) *QueryBuilder {
	qb.args = args
	return qb
}

// Execute executes the query and returns result
func (qb *QueryBuilder) Execute() (*Result, error) {
	if qb.err != nil {
		return nil, qb.err
	}
	return qb.db.Execute(qb.ctx, qb.query, qb.args...)
}

// QueryRows executes the query and returns rows
func (qb *QueryBuilder) QueryRows() (*ResultSet, error) {
	if qb.err != nil {
		return nil, qb.err
	}
	return qb.db.Query(qb.ctx, qb.query, qb.args...)
}

// QueryRow executes the query and returns a single row
func (qb *QueryBuilder) QueryRow() (*Row, error) {
	if qb.err != nil {
		return nil, qb.err
	}
	return qb.db.QueryRow(qb.ctx, qb.query, qb.args...)
}
