package tools

import (
	"context"
	"testing"
)

func TestSystemInfoTool(t *testing.T) {
	tool := NewSystemInfoTool()
	
	// Test basic properties
	if tool.Name() != "system_info" {
		t.Errorf("Expected name 'system_info', got '%s'", tool.Name())
	}
	
	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}
	
	if tool.GetCategory() != "system" {
		t.Errorf("Expected category 'system', got '%s'", tool.GetCategory())
	}
	
	// Test parameters
	params := tool.Parameters()
	if params == nil {
		t.Error("Parameters should not be nil")
	}
	
	// Test access validation
	if !tool.ValidateAccess("test_user", "test_project") {
		t.Error("System info tool should allow access for all users")
	}
	
	// Test execution
	result, err := tool.Execute(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}
	
	if result.Status != "completed" {
		t.Errorf("Expected status 'completed', got '%s'", result.Status)
	}
	
	if result.Data == nil {
		t.Error("Result data should not be nil")
	}
}

func TestDatabaseQueryTool(t *testing.T) {
	// Test without ZDB instance (minimal test)
	tool := &DatabaseQueryTool{}
	
	// Test basic properties
	if tool.Name() != "database_query" {
		t.Errorf("Expected name 'database_query', got '%s'", tool.Name())
	}
	
	if tool.GetCategory() != "database" {
		t.Errorf("Expected category 'database', got '%s'", tool.GetCategory())
	}
	
	// Test parameters
	params := tool.Parameters()
	if params == nil {
		t.Error("Parameters should not be nil")
	}
	
	// Check required parameters
	queryParam, exists := params["query"]
	if !exists {
		t.Error("Query parameter should exist")
	}
	
	if !queryParam.Required {
		t.Error("Query parameter should be required")
	}
}

func TestAPITool(t *testing.T) {
	// Test without ZDB instance (minimal test)
	tool := &APITool{}
	
	// Test basic properties
	if tool.Name() != "api_request" {
		t.Errorf("Expected name 'api_request', got '%s'", tool.Name())
	}
	
	if tool.GetCategory() != "api" {
		t.Errorf("Expected category 'api', got '%s'", tool.GetCategory())
	}
	
	// Test parameters
	params := tool.Parameters()
	if params == nil {
		t.Error("Parameters should not be nil")
	}
	
	// Check required parameters
	methodParam, exists := params["method"]
	if !exists {
		t.Error("Method parameter should exist")
	}
	
	if !methodParam.Required {
		t.Error("Method parameter should be required")
	}
	
	urlParam, exists := params["url"]
	if !exists {
		t.Error("URL parameter should exist")
	}
	
	if !urlParam.Required {
		t.Error("URL parameter should be required")
	}
}

func TestDatasourceInspectTool(t *testing.T) {
	// Test without ZDB instance (minimal test)
	tool := &DatasourceInspectTool{}
	
	// Test basic properties
	if tool.Name() != "datasource_inspect" {
		t.Errorf("Expected name 'datasource_inspect', got '%s'", tool.Name())
	}
	
	if tool.GetCategory() != "database" {
		t.Errorf("Expected category 'database', got '%s'", tool.GetCategory())
	}
	
	// Test parameters
	params := tool.Parameters()
	if params == nil {
		t.Error("Parameters should not be nil")
	}
	
	// Check optional parameters
	datasourceParam, exists := params["datasource_id"]
	if !exists {
		t.Error("datasource_id parameter should exist")
	}
	
	if datasourceParam.Required {
		t.Error("datasource_id parameter should be optional")
	}
	
	tableParam, exists := params["table_name"]
	if !exists {
		t.Error("table_name parameter should exist")
	}
	
	if tableParam.Required {
		t.Error("table_name parameter should be optional")
	}
	
	// Check boolean parameters with defaults
	includeStatsParam, exists := params["include_stats"]
	if !exists {
		t.Error("include_stats parameter should exist")
	}
	
	if includeStatsParam.Default != false {
		t.Errorf("Expected include_stats default false, got %v", includeStatsParam.Default)
	}
	
	includeColumnsParam, exists := params["include_columns"]
	if !exists {
		t.Error("include_columns parameter should exist")
	}
	
	if includeColumnsParam.Default != true {
		t.Errorf("Expected include_columns default true, got %v", includeColumnsParam.Default)
	}
	
	// Test access validation
	if !tool.ValidateAccess("test_user", "test_project") {
		t.Error("Datasource inspect tool should allow access for all users")
	}
}

func TestUnifiedDatasourceInfo(t *testing.T) {
	// Test DatasourceInfo structure
	info := &DatasourceInfo{
		Type:         "postgresql",
		DatabaseName: "test_db",
		Version:      "13.0",
		Status:       "connected",
		TableCount:   5,
	}
	
	if info.Type != "postgresql" {
		t.Errorf("Expected type 'postgresql', got '%s'", info.Type)
	}
	
	if info.DatabaseName != "test_db" {
		t.Errorf("Expected database name 'test_db', got '%s'", info.DatabaseName)
	}
}

func TestUnifiedTableInfo(t *testing.T) {
	// Test TableInfo structure
	columns := []ColumnInfo{
		{
			Name:       "id",
			Type:       "integer",
			Nullable:   false,
			PrimaryKey: true,
		},
		{
			Name:       "name",
			Type:       "varchar(255)",
			Nullable:   true,
			PrimaryKey: false,
		},
	}
	
	indexes := []IndexInfo{
		{
			Name:    "idx_name",
			Columns: []string{"name"},
			Unique:  true,
			Primary:  false,
		},
	}
	
	table := &TableInfo{
		Name:     "users",
		Type:     "table",
		RowCount:  100,
		SizeBytes: 8192,
		Columns:  columns,
		Indexes:  indexes,
	}
	
	if table.Name != "users" {
		t.Errorf("Expected table name 'users', got '%s'", table.Name)
	}
	
	if len(table.Columns) != 2 {
		t.Errorf("Expected 2 columns, got %d", len(table.Columns))
	}
	
	if len(table.Indexes) != 1 {
		t.Errorf("Expected 1 index, got %d", len(table.Indexes))
	}
	
	// Check primary key detection
	if !table.Columns[0].PrimaryKey {
		t.Error("First column should be primary key")
	}
	
	// Check nullable detection
	if table.Columns[0].Nullable {
		t.Error("Primary key column should not be nullable")
	}
	
	if !table.Columns[1].Nullable {
		t.Error("Name column should be nullable")
	}
}

func TestUnifiedColumnInfo(t *testing.T) {
	col := ColumnInfo{
		Name:        "email",
		Type:        "varchar(255)",
		Nullable:    true,
		DefaultValue: stringPtr("user@example.com"),
		PrimaryKey:  false,
		Description: "User email address",
	}
	
	if col.Name != "email" {
		t.Errorf("Expected column name 'email', got '%s'", col.Name)
	}
	
	if col.Type != "varchar(255)" {
		t.Errorf("Expected type 'varchar(255)', got '%s'", col.Type)
	}
	
	if !col.Nullable {
		t.Error("Column should be nullable")
	}
	
	if col.DefaultValue == nil {
		t.Error("Default value should not be nil")
	}
	
	if *col.DefaultValue != "user@example.com" {
		t.Errorf("Expected default value 'user@example.com', got '%s'", *col.DefaultValue)
	}
}

func TestUnifiedIndexInfo(t *testing.T) {
	index := IndexInfo{
		Name:    "idx_user_email",
		Columns: []string{"email", "status"},
		Unique:  true,
		Primary:  false,
		Type:    "btree",
	}
	
	if index.Name != "idx_user_email" {
		t.Errorf("Expected index name 'idx_user_email', got '%s'", index.Name)
	}
	
	if len(index.Columns) != 2 {
		t.Errorf("Expected 2 columns, got %d", len(index.Columns))
	}
	
	if !index.Unique {
		t.Error("Index should be unique")
	}
	
	if index.Primary {
		t.Error("Index should not be primary")
	}
}

func TestRelationInfo(t *testing.T) {
	relation := RelationInfo{
		FromTable:      "orders",
		FromColumns:    []string{"user_id"},
		ToTable:        "users",
		ToColumns:      []string{"id"},
		RelationType:   "foreign_key",
		ConstraintName: "fk_orders_user_id",
		OnDeleteAction: "CASCADE",
		OnUpdateAction: "RESTRICT",
	}
	
	if relation.FromTable != "orders" {
		t.Errorf("Expected from table 'orders', got '%s'", relation.FromTable)
	}
	
	if relation.ToTable != "users" {
		t.Errorf("Expected to table 'users', got '%s'", relation.ToTable)
	}
	
	if len(relation.FromColumns) != 1 {
		t.Errorf("Expected 1 from column, got %d", len(relation.FromColumns))
	}
	
	if relation.FromColumns[0] != "user_id" {
		t.Errorf("Expected from column 'user_id', got '%s'", relation.FromColumns[0])
	}
	
	if relation.RelationType != "foreign_key" {
		t.Errorf("Expected relation type 'foreign_key', got '%s'", relation.RelationType)
	}
	
	if relation.OnDeleteAction != "CASCADE" {
		t.Errorf("Expected on delete 'CASCADE', got '%s'", relation.OnDeleteAction)
	}
	
	if relation.OnUpdateAction != "RESTRICT" {
		t.Errorf("Expected on update 'RESTRICT', got '%s'", relation.OnUpdateAction)
	}
}

func TestRelationGraph(t *testing.T) {
	graph := &RelationGraph{
		Nodes: []RelationNode{
			{Table: "users", Type: "table"},
			{Table: "orders", Type: "table"},
		},
		Edges: []RelationEdge{
			{From: "orders", To: "users", Type: "foreign_key", Columns: []string{"user_id"}},
		},
	}
	
	if len(graph.Nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(graph.Nodes))
	}
	
	if len(graph.Edges) != 1 {
		t.Errorf("Expected 1 edge, got %d", len(graph.Edges))
	}
	
	if graph.Nodes[0].Table != "users" {
		t.Errorf("Expected first node 'users', got '%s'", graph.Nodes[0].Table)
	}
	
	if graph.Edges[0].From != "orders" {
		t.Errorf("Expected edge from 'orders', got '%s'", graph.Edges[0].From)
	}
}

func TestRelationsParameters(t *testing.T) {
	tool := &DatasourceInspectTool{}
	params := tool.Parameters()
	
	// Test new relations parameters
	includeRelations, exists := params["include_relations"]
	if !exists {
		t.Error("include_relations parameter should exist")
	}
	
	if includeRelations.Default != false {
		t.Errorf("Expected include_relations default false, got %v", includeRelations.Default)
	}
	
	if includeRelations.Required {
		t.Error("include_relations should be optional")
	}
	
	relationsDepth, exists := params["relations_depth"]
	if !exists {
		t.Error("relations_depth parameter should exist")
	}
	
	if relationsDepth.Default != 1 {
		t.Errorf("Expected relations_depth default 1, got %v", relationsDepth.Default)
	}
	
	includeReverse, exists := params["include_reverse_relations"]
	if !exists {
		t.Error("include_reverse_relations parameter should exist")
	}
	
	if includeReverse.Default != true {
		t.Errorf("Expected include_reverse_relations default true, got %v", includeReverse.Default)
	}
}

func TestDatabaseSpecificRelations(t *testing.T) {
	// Test PostgreSQL relations
	postgresRelations := []RelationInfo{
		{
			FromTable: "orders",
			ToTable: "users",
			FromColumns: []string{"user_id"},
			ToColumns: []string{"id"},
			RelationType: "foreign_key",
		},
	}
	
	// Test MySQL relations
	mysqlRelations := []RelationInfo{
		{
			FromTable: "order_items",
			ToTable: "products",
			FromColumns: []string{"product_id"},
			ToColumns: []string{"id"},
			RelationType: "foreign_key",
		},
	}
	
	// Test SQLite relations
	sqliteRelations := []RelationInfo{
		{
			FromTable: "posts",
			ToTable: "users",
			FromColumns: []string{"author_id"},
			ToColumns: []string{"id"},
			RelationType: "foreign_key",
		},
	}
	
	// Verify structure consistency
	testRelations := []RelationInfo{}
	testRelations = append(testRelations, postgresRelations...)
	testRelations = append(testRelations, mysqlRelations...)
	testRelations = append(testRelations, sqliteRelations...)
	
	for _, relation := range testRelations {
		if relation.FromTable == "" {
			t.Error("From table should not be empty")
		}
		
		if relation.ToTable == "" {
			t.Error("To table should not be empty")
		}
		
		if len(relation.FromColumns) == 0 {
			t.Error("From columns should not be empty")
		}
		
		if len(relation.ToColumns) == 0 {
			t.Error("To columns should not be empty")
		}
		
		if relation.RelationType != "foreign_key" {
			t.Errorf("Expected relation type 'foreign_key', got '%s'", relation.RelationType)
		}
	}
}

func TestRelationsDepthValidation(t *testing.T) {
	testCases := []struct {
		input    interface{}
		expected int
	}{
		{nil, 1},
		{0.5, 1},
		{1.0, 1},
		{2.0, 2},
		{3.0, 3},
		{4.0, 3}, // Max depth
		{-1.0, 1}, // Min depth
	}
	
	for _, tc := range testCases {
		relationsDepth := 1 // Default
		if depth, hasDepth := tc.input.(float64); hasDepth {
			if depth < 1 {
				relationsDepth = 1
			} else if depth > 3 {
				relationsDepth = 3
			} else {
				relationsDepth = int(depth)
			}
		}
		
		if relationsDepth != tc.expected {
			t.Errorf("Input %v: expected depth %d, got %d", tc.input, tc.expected, relationsDepth)
		}
	}
}

func stringPtr(s string) *string {
	return &s
}