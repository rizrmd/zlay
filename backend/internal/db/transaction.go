package db

import (
	"context"
	"database/sql"
)

// Transaction represents a database transaction
type Transaction struct {
	tx *sql.Tx
	db *Database
}

// Execute executes a non-query SQL statement
func (tx *Transaction) Execute(ctx context.Context, query string, args ...interface{}) (*Result, error) {
	result, err := tx.tx.ExecContext(ctx, query, args...)
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

// Query executes a query and returns a result set
func (tx *Transaction) Query(ctx context.Context, query string, args ...interface{}) (*ResultSet, error) {
	rows, err := tx.tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return ConvertSQLRowToResultSet(rows)
}

// QueryRow executes a query that returns a single row
func (tx *Transaction) QueryRow(ctx context.Context, query string, args ...interface{}) (*Row, error) {
	row := tx.tx.QueryRowContext(ctx, query, args...)
	
	// For a single row, scan into a map since sql.Row doesn't expose column info
	var result map[string]interface{}
	if err := row.Scan(&result); err != nil {
		return nil, err
	}

	// Convert map to Row
	rowValues := make([]Value, 0, len(result))
	for _, val := range result {
		expectedType := mapSQLTypeToValueType("")
		rowValues = append(rowValues, convertSQLValueToValue(val, expectedType))
	}

	return &Row{Values: rowValues}, nil
}

// Commit commits the transaction
func (tx *Transaction) Commit() error {
	return tx.tx.Commit()
}

// Rollback rolls back the transaction
func (tx *Transaction) Rollback() error {
	return tx.tx.Rollback()
}

// Result represents the result of an Execute operation
type Result struct {
	RowsAffected int64
	LastInsertID int64
}
