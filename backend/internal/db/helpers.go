package db

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Helper functions for common database operations

// Upsert performs an INSERT...ON CONFLICT operation (PostgreSQL)
func (db *Database) Upsert(ctx context.Context, table string, data map[string]interface{}, conflictColumns []string) (*Result, error) {
	columns := make([]string, 0, len(data))
	values := make([]interface{}, 0, len(data))
	placeholders := make([]string, 0, len(data))
	
	for col, val := range data {
		columns = append(columns, col)
		values = append(values, val)
		placeholders = append(placeholders, fmt.Sprintf("$%d", len(values)))
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", 
		table, 
		strings.Join(columns, ", "), 
		strings.Join(placeholders, ", "))
	
	if len(conflictColumns) > 0 {
		updateColumns := make([]string, 0, len(columns))
		for _, col := range columns {
			// Skip conflict columns
			skip := false
			for _, confCol := range conflictColumns {
				if col == confCol {
					skip = true
					break
				}
			}
			if !skip {
				updateColumns = append(updateColumns, fmt.Sprintf("%s = EXCLUDED.%s", col, col))
			}
		}
		
		if len(updateColumns) > 0 {
			query += fmt.Sprintf(" ON CONFLICT (%s) DO UPDATE SET %s", 
				strings.Join(conflictColumns, ", "), 
				strings.Join(updateColumns, ", "))
		} else {
			query += fmt.Sprintf(" ON CONFLICT (%s) DO NOTHING", strings.Join(conflictColumns, ", "))
		}
	}

	return db.Execute(ctx, query, values...)
}

// BatchInsert performs bulk insert operation
func (db *Database) BatchInsert(ctx context.Context, table string, columns []string, data [][]interface{}) (*Result, error) {
	if len(data) == 0 {
		return &Result{RowsAffected: 0, LastInsertID: 0}, nil
	}

	valueStrings := make([]string, 0, len(data))
	valueArgs := make([]interface{}, 0, len(columns)*len(data))
	
	for i, row := range data {
		valueStrings = append(valueStrings, fmt.Sprintf("(%s)", strings.Join(Placeholders(len(row), i*len(row)+1), ", ")))
		valueArgs = append(valueArgs, row...)
	}

	stmt := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s", 
		table, 
		strings.Join(columns, ", "), 
		strings.Join(valueStrings, ", "))

	return db.Execute(ctx, stmt, valueArgs...)
}

// Placeholders generates parameter placeholders
func Placeholders(count, start int) []string {
	placeholders := make([]string, count)
	for i := 0; i < count; i++ {
		placeholders[i] = fmt.Sprintf("$%d", start+i)
	}
	return placeholders
}

// UpdateWhere performs UPDATE with WHERE clause
func (db *Database) UpdateWhere(ctx context.Context, table string, set map[string]interface{}, where map[string]interface{}) (*Result, error) {
	if len(set) == 0 {
		return &Result{RowsAffected: 0}, nil
	}

	setClauses := make([]string, 0, len(set))
	setArgs := make([]interface{}, 0, len(set))
	
	for col, val := range set {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, len(setArgs)+1))
		setArgs = append(setArgs, val)
	}

	whereClauses := make([]string, 0, len(where))
	for col, val := range where {
		whereClauses = append(whereClauses, fmt.Sprintf("%s = $%d", col, len(setArgs)+len(whereClauses)+1))
		setArgs = append(setArgs, val)
	}

	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s", 
		table,
		strings.Join(setClauses, ", "),
		strings.Join(whereClauses, " AND "))

	return db.Execute(ctx, query, setArgs...)
}

// DeleteWhere performs DELETE with WHERE clause
func (db *Database) DeleteWhere(ctx context.Context, table string, where map[string]interface{}) (*Result, error) {
	if len(where) == 0 {
		return nil, fmt.Errorf("DELETE without WHERE clause is not allowed for safety")
	}

	whereClauses := make([]string, 0, len(where))
	args := make([]interface{}, 0, len(where))
	
	for col, val := range where {
		whereClauses = append(whereClauses, fmt.Sprintf("%s = $%d", col, len(args)+1))
		args = append(args, val)
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE %s", 
		table,
		strings.Join(whereClauses, " AND "))

	return db.Execute(ctx, query, args...)
}

// SelectWhere performs SELECT with WHERE clause
func (db *Database) SelectWhere(ctx context.Context, table string, columns []string, where map[string]interface{}, orderBy string, limit int) (*ResultSet, error) {
	if len(columns) == 0 {
		columns = []string{"*"}
	}

	query := fmt.Sprintf("SELECT %s FROM %s", 
		strings.Join(columns, ", "), 
		table)

	args := make([]interface{}, 0)
	
	if len(where) > 0 {
		whereClauses := make([]string, 0, len(where))
		for col, val := range where {
			whereClauses = append(whereClauses, fmt.Sprintf("%s = $%d", col, len(args)+1))
			args = append(args, val)
		}
		query += " WHERE " + strings.Join(whereClauses, " AND ")
	}

	if orderBy != "" {
		query += " ORDER BY " + orderBy
	}

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	return db.Query(ctx, query, args...)
}

// InsertAndReturn performs INSERT with RETURNING clause (PostgreSQL)
func (db *Database) InsertAndReturn(ctx context.Context, table string, data map[string]interface{}, returnColumns []string) (*Row, error) {
	columns := make([]string, 0, len(data))
	values := make([]interface{}, 0, len(data))
	placeholders := make([]string, 0, len(data))
	
	for col, val := range data {
		columns = append(columns, col)
		values = append(values, val)
		placeholders = append(placeholders, fmt.Sprintf("$%d", len(values)))
	}

	returnClause := ""
	if len(returnColumns) > 0 {
		returnClause = fmt.Sprintf(" RETURNING %s", strings.Join(returnColumns, ", "))
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)%s", 
		table, 
		strings.Join(columns, ", "), 
		strings.Join(placeholders, ", "),
		returnClause)

	return db.QueryRow(ctx, query, values...)
}

// GenerateUUID creates a new UUID string
func GenerateUUID() string {
	return uuid.New().String()
}

// Now returns current timestamp
func Now() time.Time {
	return time.Now()
}

// ParseTime parses time string
func ParseTime(timeStr string) (time.Time, error) {
	return time.Parse(time.RFC3339, timeStr)
}

// FormatTime formats time to RFC3339
func FormatTime(t time.Time) string {
	return t.Format(time.RFC3339)
}

// ConvertToMap converts struct to map using reflection
func ConvertToMap(obj interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	val := reflect.ValueOf(obj)
	
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	
	if val.Kind() == reflect.Struct {
		typ := val.Type()
		for i := 0; i < val.NumField(); i++ {
			field := typ.Field(i)
			jsonTag := field.Tag.Get("json")
			if jsonTag != "" && jsonTag != "-" {
				fieldName := strings.Split(jsonTag, ",")[0]
				if fieldName == "" {
					fieldName = field.Name
				}
				result[fieldName] = val.Field(i).Interface()
			}
		}
	}
	
	return result
}

// ConvertFromMap converts map to struct using reflection
func ConvertFromMap(data map[string]interface{}, obj interface{}) error {
	val := reflect.ValueOf(obj)
	if val.Kind() != reflect.Ptr || val.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("obj must be a pointer to a struct")
	}

	val = val.Elem()
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" && jsonTag != "-" {
			fieldName := strings.Split(jsonTag, ",")[0]
			if fieldName == "" {
				fieldName = field.Name
			}
			
			if value, exists := data[fieldName]; exists {
				fieldVal := val.Field(i)
				if fieldVal.CanSet() {
					convertedValue, err := convertValue(value, field.Type)
					if err != nil {
						return fmt.Errorf("failed to convert field %s: %w", fieldName, err)
					}
					fieldVal.Set(reflect.ValueOf(convertedValue))
				}
			}
		}
	}
	
	return nil
}

func convertValue(value interface{}, targetType reflect.Type) (interface{}, error) {
	if value == nil {
		return reflect.Zero(targetType).Interface(), nil
	}

	sourceType := reflect.TypeOf(value)
	
	if sourceType == targetType {
		return value, nil
	}

	// Handle string conversions
	if targetType.Kind() == reflect.String {
		return fmt.Sprintf("%v", value), nil
	}
	
	// Handle pointer types
	if targetType.Kind() == reflect.Ptr {
		if sourceType.Kind() == reflect.Ptr {
			if value == nil {
				return nil, nil
			}
			return convertValue(reflect.ValueOf(value).Elem().Interface(), targetType.Elem())
		} else {
			converted, err := convertValue(value, targetType.Elem())
			if err != nil {
				return nil, err
			}
			ptr := reflect.New(targetType.Elem())
			ptr.Elem().Set(reflect.ValueOf(converted))
			return ptr.Interface(), nil
		}
	}

	// Try direct conversion
	if sourceType.ConvertibleTo(targetType) {
		return reflect.ValueOf(value).Convert(targetType).Interface(), nil
	}

	return nil, fmt.Errorf("cannot convert %T to %v", value, targetType)
}
