# Relations Inspection - Complete Example

## Real-World Scenario

Let's inspect a **complete e-commerce database** with relations to see how the system handles complex relationship discovery.

## Database Schema Example

### Tables
- `users` - User accounts
- `user_profiles` - Extended user information  
- `orders` - Customer orders
- `order_items` - Order line items
- `products` - Product catalog
- `categories` - Product categories
- `addresses` - Shipping addresses

### Foreign Key Relationships
- `user_profiles.user_id` → `users.id`
- `orders.user_id` → `users.id`
- `orders.shipping_address_id` → `addresses.id`
- `order_items.order_id` → `orders.id`
- `order_items.product_id` → `products.id`
- `products.category_id` → `categories.id`

## Complete Inspection Example

### 1. Full Schema with Relations
```javascript
// Request: Get complete schema with deep relationship analysis
const result = datasource_inspect({
  datasource_id: "ds_ecommerce",
  include_stats: true,
  include_relations: true,
  relations_depth: 3,
  include_reverse_relations: true
});
```

### 2. Response Structure
```json
{
  "datasource_id": "ds_ecommerce",
  "datasource": {
    "type": "postgresql",
    "database_name": "ecommerce",
    "version": "13.7",
    "status": "connected",
    "connection_time_ms": 67,
    "table_count": 7,
    "tables": [
      {
        "name": "users",
        "type": "table",
        "row_count": 15420,
        "size_bytes": 892736,
        "columns": [
          {
            "name": "id",
            "type": "integer",
            "nullable": false,
            "primary_key": true,
            "default_value": null
          },
          {
            "name": "email",
            "type": "varchar(255)",
            "nullable": false,
            "primary_key": false,
            "default_value": null
          },
          {
            "name": "created_at",
            "type": "timestamp",
            "nullable": false,
            "primary_key": false,
            "default_value": "now()"
          }
        ],
        "indexes": [
          {
            "name": "users_pkey",
            "columns": ["id"],
            "unique": true,
            "primary": true,
            "type": "btree"
          },
          {
            "name": "idx_users_email",
            "columns": ["email"],
            "unique": true,
            "primary": false,
            "type": "btree"
          }
        ],
        "properties": {}
      },
      {
        "name": "orders",
        "type": "table",
        "row_count": 89750,
        "size_bytes": 5623400,
        "columns": [
          {
            "name": "id",
            "type": "integer",
            "nullable": false,
            "primary_key": true,
            "default_value": null
          },
          {
            "name": "user_id",
            "type": "integer",
            "nullable": false,
            "primary_key": false,
            "default_value": null
          },
          {
            "name": "total_amount",
            "type": "decimal(10,2)",
            "nullable": false,
            "primary_key": false,
            "default_value": null
          }
        ],
        "indexes": [
          {
            "name": "orders_pkey",
            "columns": ["id"],
            "unique": true,
            "primary": true,
            "type": "btree"
          },
          {
            "name": "idx_orders_user_id",
            "columns": ["user_id"],
            "unique": false,
            "primary": false,
            "type": "btree"
          }
        ],
        "properties": {}
      }
    ],
    "relations": [
      {
        "from_table": "user_profiles",
        "from_columns": ["user_id"],
        "to_table": "users",
        "to_columns": ["id"],
        "relation_type": "foreign_key",
        "constraint_name": "fk_user_profiles_user_id",
        "on_delete_action": "CASCADE",
        "on_update_action": "CASCADE"
      },
      {
        "from_table": "orders",
        "from_columns": ["user_id"],
        "to_table": "users",
        "to_columns": ["id"],
        "relation_type": "foreign_key",
        "constraint_name": "fk_orders_user_id",
        "on_delete_action": "RESTRICT",
        "on_update_action": "CASCADE"
      },
      {
        "from_table": "order_items",
        "from_columns": ["order_id"],
        "to_table": "orders",
        "to_columns": ["id"],
        "relation_type": "foreign_key",
        "constraint_name": "fk_order_items_order_id",
        "on_delete_action": "CASCADE",
        "on_update_action": "CASCADE"
      },
      {
        "from_table": "order_items",
        "from_columns": ["product_id"],
        "to_table": "products",
        "to_columns": ["id"],
        "relation_type": "foreign_key",
        "constraint_name": "fk_order_items_product_id",
        "on_delete_action": "RESTRICT",
        "on_update_action": "CASCADE"
      },
      {
        "from_table": "products",
        "from_columns": ["category_id"],
        "to_table": "categories",
        "to_columns": ["id"],
        "relation_type": "foreign_key",
        "constraint_name": "fk_products_category_id",
        "on_delete_action": "RESTRICT",
        "on_update_action": "CASCADE"
      }
    ],
    "relation_graph": {
      "nodes": [
        {"table": "users", "type": "table"},
        {"table": "user_profiles", "type": "table"},
        {"table": "orders", "type": "table"},
        {"table": "order_items", "type": "table"},
        {"table": "products", "type": "table"},
        {"table": "categories", "type": "table"},
        {"table": "addresses", "type": "table"}
      ],
      "edges": [
        {
          "from": "user_profiles",
          "to": "users",
          "type": "foreign_key",
          "columns": ["user_id"]
        },
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
        },
        {
          "from": "products",
          "to": "categories",
          "type": "foreign_key",
          "columns": ["category_id"]
        }
      ]
    },
    "properties": {
      "relation_discovery_time_ms": 145,
      "relation_count": 5,
      "max_chain_length": 3
    }
  }
}
```

## Table-Specific Relations

### Get User Table with All Dependencies
```javascript
// Request: Analyze users table and everything that references it
const userRelations = datasource_inspect({
  datasource_id: "ds_ecommerce",
  table_name: "users",
  include_relations: true,
  include_reverse_relations: true
});
```

**Response:**
```json
{
  "datasource_id": "ds_ecommerce",
  "datasource_type": "postgresql",
  "table": {
    "name": "users",
    "type": "table",
    "row_count": 15420,
    "size_bytes": 892736,
    "columns": [...],
    "indexes": [...]
  },
  "relations": [
    // Tables that reference users (reverse relations)
    {
      "from_table": "user_profiles",
      "from_columns": ["user_id"],
      "to_table": "users",
      "to_columns": ["id"],
      "relation_type": "foreign_key",
      "constraint_name": "fk_user_profiles_user_id",
      "on_delete_action": "CASCADE",
      "on_update_action": "CASCADE"
    },
    {
      "from_table": "orders",
      "from_columns": ["user_id"],
      "to_table": "users",
      "to_columns": ["id"],
      "relation_type": "foreign_key",
      "constraint_name": "fk_orders_user_id",
      "on_delete_action": "RESTRICT",
      "on_update_action": "CASCADE"
    }
  ]
}
```

## Performance Analysis

### Database-Specific Performance
```
PostgreSQL 13.7:
┌───────────────────┬──────────────┬─────────────────┐
│ Operation        │ Time (ms)   │ Records         │
├───────────────────┼──────────────┼─────────────────┤
│ Basic Discovery  │ 67           │ 7 tables        │
│ + Relations      │ 145 (+78)    │ 5 relations    │
│ + Relation Graph │ 210 (+65)    │ 5 edges        │
│ + Statistics     │ 280 (+70)    │ Row counts     │
└───────────────────┴──────────────┴─────────────────┘

MySQL 8.0:
┌───────────────────┬──────────────┬─────────────────┐
│ Operation        │ Time (ms)   │ Records         │
├───────────────────┼──────────────┼─────────────────┤
│ Basic Discovery  │ 112          │ 7 tables        │
│ + Relations      │ 195 (+83)    │ 5 relations    │
│ + Relation Graph │ 265 (+70)    │ 5 edges        │
│ + Statistics     │ 325 (+60)    │ Row counts     │
└───────────────────┴──────────────┴─────────────────┘

SQLite 3.36:
┌───────────────────┬──────────────┬─────────────────┐
│ Operation        │ Time (ms)   │ Records         │
├───────────────────┼──────────────┼─────────────────┤
│ Basic Discovery  │ 12           │ 7 tables        │
│ + Relations      │ 18 (+6)      │ 5 relations    │
│ + Relation Graph │ 25 (+7)      │ 5 edges        │
│ + Statistics     │ 35 (+10)     │ Row counts     │
└───────────────────┴──────────────┴─────────────────┘
```

## Practical Use Cases

### 1. **Impact Analysis**
```javascript
// Find impact of modifying users table
function analyzeUserTableImpact() {
  const userAnalysis = datasource_inspect({
    datasource_id: "ds_ecommerce",
    table_name: "users",
    include_relations: true,
    include_reverse_relations: true
  });
  
  return {
    directDependents: userAnalysis.relations
      .filter(r => r.to_table === "users")
      .map(r => ({
        table: r.from_table,
        impact: r.on_delete_action === "CASCADE" ? "HIGH" : "MEDIUM",
        columns: r.from_columns
      })),
    indirectDependents: findIndependentDependents("users", userAnalysis.relations),
    modificationRisk: calculateModificationRisk(userAnalysis.relations)
  };
}
```

### 2. **GraphQL Schema Generation**
```javascript
// Generate GraphQL schema from database relations
function generateGraphQLSchema() {
  const schema = datasource_inspect({
    datasource_id: "ds_ecommerce",
    include_relations: true,
    relations_depth: 2
  });
  
  let typeDefs = "";
  
  // Generate types
  schema.datasource.tables.forEach(table => {
    typeDefs += `type ${table.name} {\n`;
    
    table.columns.forEach(col => {
      typeDefs += `  ${col.name}: ${mapToGraphQLType(col.type)}\n`;
    });
    
    // Add relation fields
    const outgoingRelations = schema.datasource.relations
      .filter(r => r.from_table === table.name);
    
    outgoingRelations.forEach(rel => {
      typeDefs += `  ${rel.to_table.toLowerCase()}: ${rel.to_table}\n`;
    });
    
    typeDefs += "}\n\n";
  });
  
  return typeDefs;
}
```

### 3. **Data Flow Mapping**
```javascript
// Trace data flow through the system
function traceDataFlow(sourceTable, targetTable) {
  const schema = datasource_inspect({
    datasource_id: "ds_ecommerce",
    include_relations: true,
    relations_depth: 3
  });
  
  return findDataPath(schema.relation_graph, sourceTable, targetTable);
}

// Example: Trace from user to order
const flow = traceDataFlow("users", "orders");
// Returns: users → orders (direct relationship)
```

### 4. **Migration Planning**
```javascript
// Plan safe migration order
function planMigrationOrder() {
  const schema = datasource_inspect({
    datasource_id: "ds_ecommerce",
    include_relations: true,
    include_reverse_relations: true
  });
  
  // Build dependency graph
  const dependencies = buildDependencyGraph(schema.relations);
  
  // Topological sort for migration order
  return topologicalSort(dependencies);
}

// Result for e-commerce DB:
// 1. categories, addresses, users (no dependencies)
// 2. products (depends on categories)
// 3. orders (depends on users, addresses)  
// 4. order_items (depends on orders, products)
// 5. user_profiles (depends on users)
```

## Database-Specific Optimizations in Action

### **PostgreSQL Performance**
- Uses `referential_constraints` for ON DELETE/UPDATE actions
- Efficient `pg_catalog` filtering
- Parallel constraint discovery
- Connection pooling via ZDB

### **MySQL Optimizations**
- `DATABASE()` function for current database context
- Optimized `KEY_COLUMN_USAGE` joins
- Storage engine aware queries (InnoDB vs MyISAM)
- Prepared statement caching

### **SQLite Performance**
- Fast `PRAGMA foreign_key_list()` queries
- Native foreign key introspection
- Minimal overhead for small databases
- Transaction safety checks

## Error Handling Examples

### **Complex Relation Chains**
```javascript
// Deep relationship chains (depth > 3)
const deepResult = datasource_inspect({
  datasource_id: "ds_complex",
  include_relations: true,
  relations_depth: 5
});

// System automatically limits to depth 3 and includes warning:
{
  "datasource": {...},
  "properties": {
    "depth_limit_applied": true,
    "max_depth_reached": 5,
    "limited_depth": 3,
    "warning": "Relation chains longer than 3 levels were truncated to prevent excessive complexity"
  }
}
```

### **Circular Dependencies**
```javascript
// Detecting circular dependencies
const circularResult = datasource_inspect({
  datasource_id: "ds_circular",
  include_relations: true,
  relations_depth: 3
});

// Response includes circular dependency analysis:
{
  "datasource": {...},
  "properties": {
    "circular_dependencies": [
      {
        "cycle": ["table_a", "table_b", "table_c", "table_a"],
        "length": 4,
        "tables_involved": ["table_a", "table_b", "table_c"]
      }
    ],
    "warning": "Circular foreign key dependencies detected"
  }
}
```

## Best Practices Demonstrated

### **1. Progressive Discovery**
```javascript
// Step 1: Basic schema (fast)
const basicSchema = datasource_inspect({datasource_id: "ds_prod"});

// Step 2: Add relations for key tables
const keyTables = ["users", "orders", "products"];
keyTables.forEach(table => {
  const tableWithRelations = datasource_inspect({
    datasource_id: "ds_prod",
    table_name: table,
    include_relations: true
  });
});
```

### **2. Performance-Aware Inspection**
```javascript
// Lightweight for schema browsing
const lightweight = datasource_inspect({
  datasource_id: "ds_prod",
  include_stats: false,
  include_relations: false
});

// Heavyweight for documentation
const documentation = datasource_inspect({
  datasource_id: "ds_prod", 
  include_stats: true,
  include_relations: true,
  relations_depth: 3
});
```

### **3. Database-Specific Handling**
```javascript
// Adapt based on database type
function adaptQueryStrategy(dbType) {
  switch (dbType) {
    case "postgresql":
      return {
        include_relations: true,
        relations_depth: 3,
        include_reverse_relations: true
      };
    case "sqlite":
      return {
        include_relations: true,
        relations_depth: 2,
        include_reverse_relations: true
      };
    case "mysql":
      return {
        include_relations: true, 
        relations_depth: 2,
        include_stats: true  // MySQL has good stats performance
      };
  }
}
```

This complete example demonstrates how the relations inspection system handles real-world scenarios with multiple tables, complex relationships, and database-specific optimizations while providing consistent, actionable output for various use cases.