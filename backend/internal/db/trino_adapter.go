package db

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// TrinoAdapter provides basic Trino support using HTTP API
type TrinoAdapter struct {
	serverURL  string
	username   string
	password   string
	catalog    string
	schema     string
	httpClient *http.Client
}

// NewTrinoAdapter creates a new Trino adapter
func NewTrinoAdapter(serverURL, username, password, catalog, schema string) *TrinoAdapter {
	return &TrinoAdapter{
		serverURL: serverURL,
		username:  username,
		password:  password,
		catalog:   catalog,
		schema:    schema,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Execute executes a statement using Trino HTTP API
func (ta *TrinoAdapter) Execute(ctx context.Context, query string, args ...interface{}) (*Result, error) {
	// Prepare query
	finalQuery := query
	for i, arg := range args {
		placeholder := fmt.Sprintf("$%d", i+1)
		finalQuery = strings.ReplaceAll(finalQuery, placeholder, fmt.Sprintf("'%v'", arg))
	}

	// Prepare request body
	requestBody := map[string]interface{}{
		"query": finalQuery,
	}
	
	if ta.catalog != "" {
		requestBody["catalog"] = ta.catalog
	}
	if ta.schema != "" {
		requestBody["schema"] = ta.schema
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", ta.serverURL+"/v1/statement", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Trino-User", "zlay-db")
	
	// Set authentication if provided
	if ta.username != "" && ta.password != "" {
		req.SetBasicAuth(ta.username, ta.password)
	}

	// Send request
	resp, err := ta.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Trino returned status: %d", resp.StatusCode)
	}

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response for DML results
	var trinoResponse struct {
		ID          string        `json:"id"`
		UpdateCount int64         `json:"updateCount"`
		Rows        []map[string]interface{} `json:"data"`
		Error       *struct {
			Message string `json:"message"`
			Code    string `json:"errorCode"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &trinoResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if trinoResponse.Error != nil {
		return nil, fmt.Errorf("Trino error (%s): %s", trinoResponse.Error.Code, trinoResponse.Error.Message)
	}

	// Return proper result based on what Trino reports
	result := &Result{
		RowsAffected: trinoResponse.UpdateCount,
		LastInsertID: 0, // Trino doesn't have auto-increment IDs
	}

	// If this was a SELECT that returned data, treat it as if it affected those rows
	if len(trinoResponse.Rows) > 0 {
		result.RowsAffected = int64(len(trinoResponse.Rows))
	}

	return result, nil
}

// Query executes a query using Trino HTTP API
func (ta *TrinoAdapter) Query(ctx context.Context, query string, args ...interface{}) (*ResultSet, error) {
	// Prepare the query
	finalQuery := query
	for i, arg := range args {
		placeholder := fmt.Sprintf("$%d", i+1)
		finalQuery = strings.ReplaceAll(finalQuery, placeholder, fmt.Sprintf("'%v'", arg))
	}

	// Prepare request body
	requestBody := map[string]interface{}{
		"query": finalQuery,
	}
	
	if ta.catalog != "" {
		requestBody["catalog"] = ta.catalog
	}
	if ta.schema != "" {
		requestBody["schema"] = ta.schema
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", ta.serverURL+"/v1/statement", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Trino-User", "zlay-db")
	
	// Set authentication if provided
	if ta.username != "" && ta.password != "" {
		req.SetBasicAuth(ta.username, ta.password)
	}

	// Send request
	resp, err := ta.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Trino returned status: %d", resp.StatusCode)
	}

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response
	var trinoResponse struct {
		ID     string                 `json:"id"`
		Data   []map[string]interface{} `json:"data"`
		Columns []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"columns"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &trinoResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if trinoResponse.Error != nil {
		return nil, fmt.Errorf("Trino error: %s", trinoResponse.Error.Message)
	}

	// Convert to our ResultSet
	result := &ResultSet{}

	// Convert columns
	for _, col := range trinoResponse.Columns {
		result.Columns = append(result.Columns, Column{
			Name:     col.Name,
			Type:     mapTrinoTypeToValueType(col.Type),
			Nullable: true,
		})
	}

	// Convert rows
	for _, rowData := range trinoResponse.Data {
		row := Row{Values: make([]Value, len(trinoResponse.Columns))}
		
		for i, col := range trinoResponse.Columns {
			if val, exists := rowData[col.Name]; exists {
				row.Values[i] = convertInterfaceToTrinoValue(val, result.Columns[i].Type)
			} else {
				row.Values[i] = NewNullValue()
			}
		}
		
		result.Rows = append(result.Rows, row)
	}

	result.RowCount = len(result.Rows)
	return result, nil
}

// QueryRow executes a query that returns a single row using Trino HTTP API
func (ta *TrinoAdapter) QueryRow(ctx context.Context, query string, args ...interface{}) (*Row, error) {
	result, err := ta.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	if result.RowCount == 0 {
		return nil, fmt.Errorf("no rows found")
	}

	return &result.Rows[0], nil
}

// mapTrinoTypeToValueType maps Trino data types to our ValueType
func mapTrinoTypeToValueType(trinoType string) ValueType {
	switch {
	case containsIgnoreCase(trinoType, "integer"), containsIgnoreCase(trinoType, "bigint"), containsIgnoreCase(trinoType, "smallint"):
		return ValueTypeInteger
	case containsIgnoreCase(trinoType, "double"), containsIgnoreCase(trinoType, "real"), containsIgnoreCase(trinoType, "decimal"):
		return ValueTypeFloat
	case containsIgnoreCase(trinoType, "boolean"):
		return ValueTypeBoolean
	case containsIgnoreCase(trinoType, "date"):
		return ValueTypeDate
	case containsIgnoreCase(trinoType, "time"):
		return ValueTypeTime
	case containsIgnoreCase(trinoType, "timestamp"):
		return ValueTypeTimestamp
	case containsIgnoreCase(trinoType, "varchar"), containsIgnoreCase(trinoType, "char"):
		return ValueTypeText
	case containsIgnoreCase(trinoType, "varbinary"), containsIgnoreCase(trinoType, "binary"):
		return ValueTypeBinary
	default:
		return ValueTypeText
	}
}

// convertInterfaceToTrinoValue converts interface{} to our Value type
func convertInterfaceToTrinoValue(val interface{}, expectedType ValueType) Value {
	if val == nil {
		return NewNullValue()
	}

	// Convert based on expected type
	switch expectedType {
	case ValueTypeInteger:
		// Parse as integer
		if intVal, ok := val.(int64); ok {
			return NewIntegerValue(intVal)
		} else if intVal, ok := val.(int); ok {
			return NewIntegerValue(int64(intVal))
		} else if floatVal, ok := val.(float64); ok {
			return NewIntegerValue(int64(floatVal))
		} else if strVal, ok := val.(string); ok {
			if intVal, err := strconv.ParseInt(strVal, 10, 64); err == nil {
				return NewIntegerValue(intVal)
			}
		}
	case ValueTypeFloat:
		// Parse as float
		if floatVal, ok := val.(float64); ok {
			return NewFloatValue(floatVal)
		} else if intVal, ok := val.(int64); ok {
			return NewFloatValue(float64(intVal))
		} else if strVal, ok := val.(string); ok {
			if floatVal, err := strconv.ParseFloat(strVal, 64); err == nil {
				return NewFloatValue(floatVal)
			}
		}
	case ValueTypeBoolean:
		// Parse as boolean
		if boolVal, ok := val.(bool); ok {
			return NewBooleanValue(boolVal)
		} else if strVal, ok := val.(string); ok {
			if boolVal, err := strconv.ParseBool(strVal); err == nil {
				return NewBooleanValue(boolVal)
			}
		}
	case ValueTypeTimestamp:
		// Parse as timestamp
		if tsVal, ok := val.(time.Time); ok {
			return NewTimestampValue(tsVal)
		} else if strVal, ok := val.(string); ok {
			if tsVal, err := time.Parse(time.RFC3339, strVal); err == nil {
				return NewTimestampValue(tsVal)
			} else if tsVal, err := time.Parse("2006-01-02 15:04:05", strVal); err == nil {
				return NewTimestampValue(tsVal)
			}
		}
	case ValueTypeBinary:
		// Parse as binary
		if bytesVal, ok := val.([]byte); ok {
			return NewBinaryValue(bytesVal)
		}
	default:
		// Default to text
		if textVal, ok := val.(string); ok {
			return NewTextValue(textVal)
		} else {
			return NewTextValue(fmt.Sprintf("%v", val))
		}
	}

	// Fallback to text
	return NewTextValue(fmt.Sprintf("%v", val))
}

// TestConnection tests Trino connection
func (ta *TrinoAdapter) TestConnection(ctx context.Context) error {
	_, err := ta.Query(ctx, "SELECT 1")
	return err
}
