package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ORM-like helper functions for common operations using zlay-db

// FindOne finds a single record by ID
func (db *Database) FindOne(ctx context.Context, table string, id string) (*Row, error) {
	return db.QueryRow(ctx, 
		fmt.Sprintf("SELECT * FROM %s WHERE id = $1", table), 
		id)
}

// FindMany finds multiple records with optional WHERE clause
func (db *Database) FindMany(ctx context.Context, table string, where map[string]interface{}, orderBy string, limit int) (*ResultSet, error) {
	if where == nil {
		where = make(map[string]interface{})
	}
	return db.SelectWhere(ctx, table, []string{"*"}, where, orderBy, limit)
}

// Create creates a new record and returns the created record
func (db *Database) Create(ctx context.Context, table string, data map[string]interface{}) (*Row, error) {
	// Add ID if not present
	if _, exists := data["id"]; !exists {
		data["id"] = uuid.New().String()
	}
	
	// Add created_at if not present
	if _, exists := data["created_at"]; !exists {
		data["created_at"] = time.Now()
	}

	return db.InsertAndReturn(ctx, table, data, []string{"*"})
}

// Update updates a record by ID
func (db *Database) Update(ctx context.Context, table string, id string, data map[string]interface{}) (*Result, error) {
	// Add updated_at if not present
	if _, exists := data["updated_at"]; !exists {
		data["updated_at"] = time.Now()
	}
	
	return db.UpdateWhere(ctx, table, data, map[string]interface{}{"id": id})
}

// Delete soft deletes a record by ID (sets is_active = false)
func (db *Database) Delete(ctx context.Context, table string, id string) (*Result, error) {
	data := map[string]interface{}{
		"is_active":   false,
		"updated_at":  time.Now(),
	}
	
	return db.UpdateWhere(ctx, table, data, map[string]interface{}{"id": id})
}

// HardDelete permanently deletes a record by ID
func (db *Database) HardDelete(ctx context.Context, table string, id string) (*Result, error) {
	return db.DeleteWhere(ctx, table, map[string]interface{}{"id": id})
}

// Count counts records with optional WHERE clause
func (db *Database) Count(ctx context.Context, table string, where map[string]interface{}) (int64, error) {
	result, err := db.QueryRow(ctx, 
		fmt.Sprintf("SELECT COUNT(*) as count FROM %s", table), 
		convertMapToArgs(where)...)
	if err != nil {
		return 0, err
	}
	
	count, ok := result.Values[0].AsInt64()
	if !ok {
		return 0, fmt.Errorf("failed to convert count to int64")
	}
	
	return count, nil
}

// Exists checks if a record exists
func (db *Database) Exists(ctx context.Context, table string, where map[string]interface{}) (bool, error) {
	count, err := db.Count(ctx, table, where)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// First finds the first record with optional WHERE clause
func (db *Database) First(ctx context.Context, table string, where map[string]interface{}, orderBy string) (*Row, error) {
	result, err := db.FindMany(ctx, table, where, orderBy, 1)
	if err != nil {
		return nil, err
	}
	
	if result.RowCount == 0 {
		return nil, fmt.Errorf("no records found")
	}
	
	return &result.Rows[0], nil
}

// Paginate paginates results
func (db *Database) Paginate(ctx context.Context, table string, where map[string]interface{}, orderBy string, page, limit int) (*ResultSet, error) {
	offset := (page - 1) * limit
	if offset < 0 {
		offset = 0
	}
	
	result, err := db.SelectWhere(ctx, table, []string{"*"}, where, orderBy, limit+offset)
	if err != nil {
		return nil, err
	}
	
	// Apply pagination manually
	if offset > 0 && result.RowCount > offset {
		result.Rows = result.Rows[offset:]
		result.RowCount = len(result.Rows)
	} else if offset > 0 {
		result.Rows = []Row{}
		result.RowCount = 0
	}
	
	return result, nil
}

// TransactionFunc represents a function to be executed within a transaction
type TransactionFunc func(*Transaction) error

// WithTransaction executes a function within a transaction
func (db *Database) WithTransaction(ctx context.Context, fn TransactionFunc) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	
	defer func() {
		if p := recover(); p != nil {
			// Panic occurred, rollback
			tx.Rollback()
			panic(p) // Re-panic after rollback
		}
	}()
	
	if err := fn(tx); err != nil {
		// Error occurred, rollback
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("transaction failed: %v, rollback failed: %w", err, rbErr)
		}
		return err
	}
	
	// Success, commit
	return tx.Commit()
}

// BatchOperation represents a batch operation
type BatchOperation struct {
	Type string // "insert", "update", "delete"
	Data interface{}
	Table string
	Where map[string]interface{} // for update/delete
}

// BatchExecute executes multiple operations in a transaction
func (db *Database) BatchExecute(ctx context.Context, operations []BatchOperation) error {
	return db.WithTransaction(ctx, func(tx *Transaction) error {
		for _, op := range operations {
			var err error
			
			switch op.Type {
			case "insert":
				if data, ok := op.Data.(map[string]interface{}); ok {
					_, err = tx.Execute(ctx, 
						fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", 
							op.Table,
							getKeys(data),
							getPlaceholders(data)),
						getValues(data)...)
				}
			case "update":
				if data, ok := op.Data.(map[string]interface{}); ok {
					_, err = tx.Execute(ctx,
						fmt.Sprintf("UPDATE %s SET %s WHERE %s",
							op.Table,
							getSetClause(data),
							getWhereClause(op.Where)),
						getValues(data, op.Where)...)
				}
			case "delete":
				_, err = tx.Execute(ctx,
					fmt.Sprintf("DELETE FROM %s WHERE %s",
						op.Table,
						getWhereClause(op.Where)),
					getValues(op.Where)...)
			}
			
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// Helper functions for SQL generation

func getKeys(data map[string]interface{}) string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	return strings.Join(keys, ", ")
}

func getPlaceholders(data map[string]interface{}) string {
	placeholders := make([]string, 0, len(data))
	for i := 0; i < len(data); i++ {
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
	}
	return strings.Join(placeholders, ", ")
}

func getSetClause(data map[string]interface{}) string {
	clauses := make([]string, 0, len(data))
	for k := range data {
		clauses = append(clauses, fmt.Sprintf("%s = $%d", k, len(clauses)+1))
	}
	return strings.Join(clauses, ", ")
}

func getWhereClause(where map[string]interface{}) string {
	if len(where) == 0 {
		return "1=1"
	}
	clauses := make([]string, 0, len(where))
	for k := range where {
		clauses = append(clauses, fmt.Sprintf("%s = $%d", k, len(clauses)+1))
	}
	return strings.Join(clauses, " AND ")
}

func getValues(data ...map[string]interface{}) []interface{} {
	var values []interface{}
	for _, d := range data {
		for _, v := range d {
			values = append(values, v)
		}
	}
	return values
}

func convertMapToArgs(where map[string]interface{}) []interface{} {
	if where == nil {
		return nil
	}
	args := make([]interface{}, 0, len(where))
	for _, v := range where {
		args = append(args, v)
	}
	return args
}

func convertSQLValueToGoValue(v Value) interface{} {
	if v.IsNull() {
		return nil
	}
	
	switch v.Type {
	case ValueTypeInteger:
		val, _ := v.AsInt64()
		return val
	case ValueTypeFloat:
		val, _ := v.AsFloat64()
		return val
	case ValueTypeText:
		val, _ := v.AsString()
		return val
	case ValueTypeBoolean:
		val, _ := v.AsBool()
		return val
	case ValueTypeBinary:
		val, _ := v.AsBytes()
		return val
	case ValueTypeTimestamp:
		val, _ := v.AsTimestamp()
		return val.Time
	default:
		return nil
	}
}
