package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// DatabaseType represents supported database types
type DatabaseType string

const (
	DatabaseTypePostgreSQL DatabaseType = "postgresql"
	DatabaseTypeMySQL     DatabaseType = "mysql"
	DatabaseTypeSQLite    DatabaseType = "sqlite"
	DatabaseTypeSQLServer DatabaseType = "sqlserver"
	DatabaseTypeOracle    DatabaseType = "oracle"
	DatabaseTypeClickHouse DatabaseType = "clickhouse"
	DatabaseTypeTrino     DatabaseType = "trino"
	DatabaseTypeCSV       DatabaseType = "csv"
	DatabaseTypeExcel     DatabaseType = "excel"
)

// ValueType represents the type of a database value
type ValueType string

const (
	ValueTypeNull      ValueType = "null"
	ValueTypeInteger   ValueType = "integer"
	ValueTypeFloat     ValueType = "float"
	ValueTypeText      ValueType = "text"
	ValueTypeBoolean   ValueType = "boolean"
	ValueTypeBinary    ValueType = "binary"
	ValueTypeDate      ValueType = "date"
	ValueTypeTime      ValueType = "time"
	ValueTypeTimestamp ValueType = "timestamp"
)

// Value represents a unified database value
type Value struct {
	Type  ValueType
	Data  interface{}
	Valid bool
}

// Date represents a date value
type Date struct {
	Year  int
	Month int
	Day   int
}

// Time represents a time value
type Time struct {
	Hour   int
	Minute int
	Second int
	Nanosec int
}

// Timestamp represents a timestamp value
type Timestamp struct {
	time.Time
}

// NewNullValue creates a new null value
func NewNullValue() Value {
	return Value{
		Type:  ValueTypeNull,
		Data:  nil,
		Valid: false,
	}
}

// NewIntegerValue creates a new integer value
func NewIntegerValue(v int64) Value {
	return Value{
		Type:  ValueTypeInteger,
		Data:  v,
		Valid: true,
	}
}

// NewFloatValue creates a new float value
func NewFloatValue(v float64) Value {
	return Value{
		Type:  ValueTypeFloat,
		Data:  v,
		Valid: true,
	}
}

// NewTextValue creates a new text value
func NewTextValue(v string) Value {
	return Value{
		Type:  ValueTypeText,
		Data:  v,
		Valid: true,
	}
}

// NewBooleanValue creates a new boolean value
func NewBooleanValue(v bool) Value {
	return Value{
		Type:  ValueTypeBoolean,
		Data:  v,
		Valid: true,
	}
}

// NewBinaryValue creates a new binary value
func NewBinaryValue(v []byte) Value {
	return Value{
		Type:  ValueTypeBinary,
		Data:  v,
		Valid: true,
	}
}

// NewDateValue creates a new date value
func NewDateValue(year, month, day int) Value {
	return Value{
		Type: ValueTypeDate,
		Data: Date{
			Year:  year,
			Month: month,
			Day:   day,
		},
		Valid: true,
	}
}

// NewTimeValue creates a new time value
func NewTimeValue(hour, minute, second, nanosec int) Value {
	return Value{
		Type: ValueTypeTime,
		Data: Time{
			Hour:    hour,
			Minute:  minute,
			Second:  second,
			Nanosec: nanosec,
		},
		Valid: true,
	}
}

// NewTimestampValue creates a new timestamp value
func NewTimestampValue(t time.Time) Value {
	return Value{
		Type: ValueTypeTimestamp,
		Data: Timestamp{Time: t},
		Valid: true,
	}
}

// AsInt64 returns value as int64
func (v Value) AsInt64() (int64, bool) {
	if v.Type == ValueTypeInteger && v.Valid {
		return v.Data.(int64), true
	}
	return 0, false
}

// AsFloat64 returns value as float64
func (v Value) AsFloat64() (float64, bool) {
	if v.Type == ValueTypeFloat && v.Valid {
		return v.Data.(float64), true
	}
	return 0, false
}

// AsString returns value as string
func (v Value) AsString() (string, bool) {
	if v.Type == ValueTypeText && v.Valid {
		return v.Data.(string), true
	}
	return "", false
}

// AsBool returns value as bool
func (v Value) AsBool() (bool, bool) {
	if v.Type == ValueTypeBoolean && v.Valid {
		return v.Data.(bool), true
	}
	return false, false
}

// AsBytes returns value as []byte
func (v Value) AsBytes() ([]byte, bool) {
	if v.Type == ValueTypeBinary && v.Valid {
		return v.Data.([]byte), true
	}
	return nil, false
}

// AsDate returns value as Date
func (v Value) AsDate() (Date, bool) {
	if v.Type == ValueTypeDate && v.Valid {
		return v.Data.(Date), true
	}
	return Date{}, false
}

// AsTime returns value as Time
func (v Value) AsTime() (Time, bool) {
	if v.Type == ValueTypeTime && v.Valid {
		return v.Data.(Time), true
	}
	return Time{}, false
}

// AsTimestamp returns value as Timestamp
func (v Value) AsTimestamp() (Timestamp, bool) {
	if v.Type == ValueTypeTimestamp && v.Valid {
		return v.Data.(Timestamp), true
	}
	return Timestamp{}, false
}

// IsNull returns true if value is null
func (v Value) IsNull() bool {
	return v.Type == ValueTypeNull || !v.Valid
}

// ResultSet represents a query result set
type ResultSet struct {
	Rows     []Row
	Columns  []Column
	RowCount int
}

// Row represents a database row
type Row struct {
	Values []Value
}

// Column represents a database column
type Column struct {
	Name     string
	Type     ValueType
	Nullable bool
}

// ConvertSQLRowToResultSet converts sql.Rows to ResultSet
func ConvertSQLRowToResultSet(rows *sql.Rows) (*ResultSet, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	result := &ResultSet{
		Columns: make([]Column, len(columns)),
	}

	// Get column types
	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}

	for i, col := range columns {
		dbType := columnTypes[i].DatabaseTypeName()
		result.Columns[i] = Column{
			Name:     col,
			Type:     mapSQLTypeToValueType(dbType),
			Nullable: true, // Assume nullable for safety
		}
	}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		row := Row{Values: make([]Value, len(columns))}
		for i, val := range values {
			row.Values[i] = convertSQLValueToValue(val, result.Columns[i].Type)
		}

		result.Rows = append(result.Rows, row)
	}

	result.RowCount = len(result.Rows)
	return result, nil
}

// mapSQLTypeToValueType maps SQL type names to ValueType
func mapSQLTypeToValueType(sqlType string) ValueType {
	switch {
	case containsIgnoreCase(sqlType, "int"), containsIgnoreCase(sqlType, "serial"):
		return ValueTypeInteger
	case containsIgnoreCase(sqlType, "float"), containsIgnoreCase(sqlType, "double"), containsIgnoreCase(sqlType, "decimal"):
		return ValueTypeFloat
	case containsIgnoreCase(sqlType, "bool"):
		return ValueTypeBoolean
	case containsIgnoreCase(sqlType, "date"):
		return ValueTypeDate
	case containsIgnoreCase(sqlType, "time"):
		return ValueTypeTime
	case containsIgnoreCase(sqlType, "timestamp"):
		return ValueTypeTimestamp
	case containsIgnoreCase(sqlType, "blob"), containsIgnoreCase(sqlType, "binary"):
		return ValueTypeBinary
	default:
		return ValueTypeText
	}
}

// convertSQLValueToValue converts SQL value to Value
func convertSQLValueToValue(val interface{}, expectedType ValueType) Value {
	if val == nil {
		return NewNullValue()
	}

	switch v := val.(type) {
	case int64:
		if expectedType == ValueTypeBoolean {
			return NewBooleanValue(v != 0)
		}
		return NewIntegerValue(v)
	case float64:
		return NewFloatValue(v)
	case string:
		return NewTextValue(v)
	case bool:
		return NewBooleanValue(v)
	case []byte:
		return NewBinaryValue(v)
	case time.Time:
		if v.IsZero() {
			return NewNullValue()
		}
		return NewTimestampValue(v)
	default:
		return NewTextValue(string(fmt.Sprintf("%v", v)))
	}
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && 
		(s[:len(substr)] == substr || 
		 s[len(s)-len(substr):] == substr ||
		 findSubstringIgnoreCase(s, substr))
}

func findSubstringIgnoreCase(s, substr string) bool {
	s = strings.ToLower(s)
	substr = strings.ToLower(substr)
	return strings.Contains(s, substr)
}
