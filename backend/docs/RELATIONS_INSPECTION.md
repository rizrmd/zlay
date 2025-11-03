# Database Relations Inspection Guide

## Overview

Zlay now provides **comprehensive foreign key relationship inspection** as part of the unified datasource inspection system. Relations can be discovered with database-specific optimizations and flexible parameter control.

## Relation Inspection Parameters

### `include_relations` (boolean, default: false)
- **Description**: Include foreign key relationships and dependencies
- **Impact**: Adds relation discovery to inspection time
- **Recommendation**: Set to `true` only when you need relationship information

### `relations_depth` (number, default: 1, max: 3)
- **Description**: How deep to follow relation chains
- **Levels**:
  - `1`: Direct foreign key relationships
  - `2`: Include indirect relationships (table → table → table)
  - `3`: Extended relationship chains

### `include_reverse_relations` (boolean, default: true)
- **Description**: Include reverse references (tables that reference this table)
- **Use Case**: Find all tables that depend on a specific table

## Relation Data Structure

### RelationInfo Structure
```json
{
  "from_table": "orders",
  "from_columns": ["user_id"],
  "to_table": "users", 
  "to_columns": ["id"],
  "relation_type": "foreign_key",
  "constraint_name": "fk_orders_user_id",
  "on_delete_action": "CASCADE",
  "on_update_action": "RESTRICT"
}
```

### RelationGraph Structure (when relations_depth > 1)
```json
{
  "nodes": [
    {"table": "users", "type": "table"},
    {"table": "orders", "type": "table"},
    {"table": "order_items", "type": "table"}
  ],
  "edges": [
    {
      "from": "orders",
      "to": "users", 
      "type": "foreign_key",
      "columns": ["user_id"]
    },
    {
      "from": "order_items",
      "to": "orders",
      "type": "foreign_key", 
      "columns": ["order_id"]
    }
  ]
}
```

## Database-Specific Optimizations

### PostgreSQL Relations
**Optimization Strategy:**
```sql
-- Full constraint information with referential actions
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
JOIN information_schema.constraint_column_usage AS ccu
  ON ccu.constraint_name = tc.constraint_name
LEFT JOIN information_schema.referential_constraints rc
  ON tc.constraint_name = rc.constraint_name
WHERE tc.constraint_type = 'FOREIGN KEY'
```

**PostgreSQL-Specific Features:**
- **Referential Actions**: ON DELETE, ON UPDATE detection
- **Constraint Validation**: Deferred constraint support
- **Schema Filtering**: Excludes system schemas
- **Composite Keys**: Multi-column foreign key support

### MySQL Relations
**Optimization Strategy:**
```sql
-- KEY_COLUMN_USAGE with referential information
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
```

**MySQL-Specific Features:**
- **Database Context**: CURRENT_DATABASE() filtering
- **Storage Engine**: InnoDB vs MyISAM foreign key differences
- **Constraint Naming**: Automatic constraint name generation
- **Character Sets**: Relation charset consistency

### SQLite Relations
**Optimization Strategy:**
```sql
-- Fast PRAGMA-based foreign key discovery
PRAGMA foreign_key_list(table_name)

-- Returns: id, seq, table, from, to, on_update, on_delete, match
```

**SQLite-Specific Features:**
- **PRAGMA Optimization**: Native foreign key introspection
- **File-based Discovery**: Database file analysis
- **Foreign Key Support**: Check if foreign keys are enabled
- **Pragmas**: `foreign_keys`, `defer_foreign_keys` status

### Generic Relations (Fallback)
**Optimization Strategy:**
```sql
-- Standard information schema approach
SELECT 
    tc.table_name,
    kcu.column_name,
    ccu.table_name AS foreign_table_name,
    ccu.column_name AS foreign_column_name,
    tc.constraint_name
FROM information_schema.table_constraints AS tc
JOIN information_schema.key_column_usage AS kcu
  ON tc.constraint_name = kcu.constraint_name
JOIN information_schema.constraint_column_usage AS ccu
  ON ccu.constraint_name = tc.constraint_name
WHERE tc.constraint_type = 'FOREIGN KEY'
```

## Usage Examples

### Basic Relation Discovery
```javascript
// Get all tables with their foreign key relationships
datasource_inspect(
  datasource_id: "ds_prod",
  include_relations: true
)
```

**Response:**
```json
{
  "datasource": {
    "type": "postgresql",
    "table_count": 5,
    "tables": [...],
    "relations": [
      {
        "from_table": "orders",
        "from_columns": ["user_id"],
        "to_table": "users",
        "to_columns": ["id"],
        "relation_type": "foreign_key",
        "constraint_name": "fk_orders_user_id",
        "on_delete_action": "CASCADE",
        "on_update_action": "RESTRICT"
      }
    ]
  }
}
```

### Deep Relationship Analysis
```javascript
// Get 3-level deep relationship graph
datasource_inspect(
  datasource_id: "ds_prod",
  include_relations: true,
  relations_depth: 3,
  include_reverse_relations: true
)
```

**Response:**
```json
{
  "datasource": {
    "tables": [...],
    "relations": [...],
    "relation_graph": {
      "nodes": [
        {"table": "users", "type": "table"},
        {"table": "orders", "type": "table"},
        {"table": "order_items", "type": "table"},
        {"table": "products", "type": "table"}
      ],
      "edges": [
        {
          "from": "orders",
          "to": "users",
          "type": "foreign_key",
          "columns": ["user_id"]
        },
        {
          "from": "order_items", 
          "to": "orders",
          "type": "foreign_key",
          "columns": ["order_id"]
        },
        {
          "from": "order_items",
          "to": "products",
          "type": "foreign_key", 
          "columns": ["product_id"]
        }
      ]
    }
  }
}
```

### Table-Specific Relations
```javascript
// Get relations for specific table only
datasource_inspect(
  datasource_id: "ds_prod",
  table_name: "users",
  include_relations: true,
  include_reverse_relations: true
)
```

**Response:**
```json
{
  "datasource_id": "ds_prod",
  "datasource_type": "postgresql",
  "table": {
    "name": "users",
    "columns": [...],
    "indexes": [...]
  },
  "relations": [
    {
      "from_table": "orders",
      "from_columns": ["user_id"], 
      "to_table": "users",
      "to_columns": ["id"],
      "relation_type": "foreign_key",
      "constraint_name": "fk_orders_user_id"
    },
    {
      "from_table": "user_profiles",
      "from_columns": ["user_id"],
      "to_table": "users", 
      "to_columns": ["id"],
      "relation_type": "foreign_key",
      "constraint_name": "fk_user_profiles_user_id"
    }
  ]
}
```

## Performance Considerations

### Relation Discovery Impact by Database

| Database | Base Inspection | +Relations | +Graph (depth 3) |
|----------|----------------|------------|-------------------|
| SQLite   | ~8ms           | ~12ms      | ~20ms             |
| PostgreSQL | ~45ms         | ~80ms      | ~150ms            |
| MySQL    | ~80ms           | ~120ms     | ~220ms            |
| SQL Server | ~65ms         | ~100ms     | ~180ms            |
| Oracle   | ~70ms           | ~110ms     | ~200ms            |

### Optimization Recommendations

#### **For Small Databases (< 50 tables)**
```javascript
// Include everything - fast enough
datasource_inspect(
  datasource_id: "ds_small",
  include_relations: true,
  relations_depth: 3,
  include_stats: true
)
```

#### **For Medium Databases (50-200 tables)**
```javascript
// Selective relations discovery
datasource_inspect(
  datasource_id: "ds_medium", 
  include_relations: true,
  relations_depth: 2,
  include_stats: false  // Skip expensive stats
)
```

#### **For Large Databases (> 200 tables)**
```javascript
// Progressive discovery approach
// 1. First get basic schema
datasource_inspect(datasource_id: "ds_large")

// 2. Then add relations for specific tables
datasource_inspect(
  datasource_id: "ds_large",
  table_name: "specific_table",
  include_relations: true
)
```

## Advanced Features

### Reverse Relation Detection
```javascript
// Find what references a table
datasource_inspect(
  datasource_id: "ds_prod",
  table_name: "users",
  include_relations: true,
  include_reverse_relations: true
)
```

**Use Cases:**
- **Impact Analysis**: What happens if I modify/delete this table?
- **Dependency Mapping**: Which tables depend on this table?
- **Migration Planning**: Safe table modification order

### Relation Chain Analysis
```javascript
// Deep relationship analysis
datasource_inspect(
  datasource_id: "ds_prod",
  include_relations: true,
  relations_depth: 3
)
```

**Use Cases:**
- **Data Flow Mapping**: How does data move through the system?
- **Join Path Discovery**: Optimal join strategies
- **GraphQL Schema**: Build relationship-based APIs

### Database-Specific Features

#### **PostgreSQL**
```json
{
  "relation_type": "foreign_key",
  "on_delete_action": "CASCADE",
  "on_update_action": "RESTRICT", 
  "deferrable": true,
  "initially_deferred": false,
  "constraint_validation": "VALID"
}
```

#### **MySQL**
```json
{
  "relation_type": "foreign_key",
  "on_delete_action": "CASCADE",
  "on_update_action": "CASCADE",
  "constraint_name": "fk_name_ibfk_1",  // Auto-generated suffix
  "storage_engine": "InnoDB"
}
```

#### **SQLite**
```json
{
  "relation_type": "foreign_key",
  "on_delete_action": "CASCADE",
  "on_update_action": "CASCADE",
  "match_type": "SIMPLE",
  "deferrable": false,
  "foreign_keys_enabled": true
}
```

## Error Handling

### Common Relation Errors

#### **Foreign Keys Not Enabled (SQLite)**
```json
{
  "relations_error": "Foreign keys are not enabled in this SQLite database",
  "properties": {
    "pragmas": {
      "foreign_keys": "OFF",
      "suggestion": "Execute PRAGMA foreign_keys = ON"
    }
  }
}
```

#### **Missing Constraints**
```json
{
  "relations_error": "No foreign key constraints found in specified table",
  "properties": {
    "suggestion": "Check if table uses natural keys or referential integrity is enforced at application level"
  }
}
```

#### **Access Permission Errors**
```json
{
  "relations_error": "Permission denied for constraint metadata access",
  "properties": {
    "required_permissions": ["SELECT", "REFERENCES"],
    "missing_permissions": ["REFERENCES"],
    "suggestion": "Grant REFERENCES privilege on target tables"
  }
}
```

## Integration Examples

### **ER Diagram Generation**
```javascript
// Get complete schema for ER diagram
const schema = datasource_inspect({
  datasource_id: "ds_prod",
  include_relations: true,
  relations_depth: 2,
  include_columns: true,
  include_indexes: true
});

// Generate Mermaid diagram
function generateMermaid(schema) {
  let diagram = "erDiagram\n";
  
  // Add tables
  schema.datasource.tables.forEach(table => {
    diagram += `  ${table.name} {\n`;
    table.columns.forEach(col => {
      diagram += `    ${col.name} ${col.type}${col.primary_key ? " PK" : ""}\n`;
    });
    diagram += "  }\n";
  });
  
  // Add relationships  
  schema.datasource.relations.forEach(rel => {
    diagram += `  ${rel.from_table} ||--o{ ${rel.to_table} : "${rel.constraint_name}"\n`;
  });
  
  return diagram;
}
```

### **Data Impact Analysis**
```javascript
// Analyze impact of table changes
function analyzeImpact(tableName) {
  const schema = datasource_inspect({
    datasource_id: "ds_prod",
    table_name: tableName,
    include_relations: true,
    include_reverse_relations: true
  });
  
  return {
    directDependencies: schema.relations
      .filter(r => r.from_table === tableName)
      .map(r => r.to_table),
    dependents: schema.relations
      .filter(r => r.to_table === tableName)
      .map(r => r.from_table),
    cascadeRisk: schema.relations
      .filter(r => r.to_table === tableName)
      .some(r => r.on_delete_action === "CASCADE")
  };
}
```

### **Join Path Optimization**
```javascript
// Find optimal join paths between tables
function findJoinPath(fromTable, toTable) {
  const schema = datasource_inspect({
    datasource_id: "ds_prod",
    include_relations: true,
    relations_depth: 3
  });
  
  // Build graph from relations
  const graph = buildRelationGraph(schema.datasource.relations);
  
  // Find shortest path using BFS
  return findShortestPath(graph, fromTable, toTable);
}
```

## Best Practices

### **When to Include Relations**
- **Schema Documentation**: Always include for complete documentation
- **Migration Planning**: Include reverse relations for impact analysis
- **API Design**: Use relation graphs for GraphQL/resolver design
- **Performance Tuning**: Include to identify join optimization opportunities

### **When to Exclude Relations**
- **Fast Schema Discovery**: Skip relations when you only need table/column info
- **Large Databases**: Exclude for initial schema browse, add as needed
- **Read-only Operations**: Skip when you don't need relationship context

### **Depth Selection Guidelines**
- **Depth 1**: Most common use case - direct relationships only
- **Depth 2**: Good for dependency analysis and impact assessment
- **Depth 3**: Only for complex data flow analysis and comprehensive documentation

### **Performance Tips**
1. **Progressive Loading**: Start with basic schema, add relations as needed
2. **Table-Specific Inspection**: Use `table_name` parameter for focused analysis
3. **Cache Results**: Store relation graphs for repeated analysis
4. **Depth Limitation**: Use minimum depth required for your use case

The relations inspection system provides powerful, database-optimized foreign key discovery while maintaining flexibility through parameter control and consistent output structure across all supported database types.