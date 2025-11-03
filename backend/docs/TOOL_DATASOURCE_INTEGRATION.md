# Tool-Datasource Integration Guide

## Overview

Zlay now supports dynamic tool-datasource integration where each project can have multiple datasources and tools automatically adapt to query specific datasources.

## Supported Datasource Types

### Database Datasources
- **PostgreSQL** - `postgres` or `postgresql`
- **MySQL** - `mysql`
- **SQLite** - `sqlite` or `sqlite3`
- **SQL Server** - `sqlserver` or `mssql`
- **Oracle** - `oracle`
- **Trino** - `trino` or `presto`
- **ClickHouse** - `clickhouse`

### API Datasources
- **REST APIs** - Any HTTP/REST endpoint
- **Authentication**: Bearer token, Basic auth, API key

## Available Tools

### 1. Database Query Tool (`database_query`)
Executes SQL queries on database datasources with security filtering.

**Parameters:**
- `datasource_id` (string, optional): Specific datasource ID to query
- `query` (string, required): SQL query to execute
- `timeout_seconds` (number, optional): Query timeout in seconds

**Security:**
- Blocks dangerous operations (DROP, TRUNCATE, ALTER DATABASE)
- Supports SELECT, INSERT, UPDATE, DELETE, CREATE operations
- Project-based access control

### 2. API Request Tool (`api_request`)
Executes HTTP requests to REST API endpoints.

**Parameters:**
- `datasource_id` (string, optional): API datasource ID for auth/base URL
- `method` (string, required): HTTP method (GET, POST, PUT, DELETE, PATCH)
- `url` (string, required): Full URL or endpoint path
- `headers` (object, optional): Additional HTTP headers
- `body` (string, optional): Request body (JSON string)
- `timeout_seconds` (number, optional): Request timeout in seconds

### 3. System Info Tool (`system_info`)
Provides system information for debugging and monitoring.

**Parameters:**
- `include_memory` (boolean, optional): Include memory usage information
- `include_disk` (boolean, optional): Include disk usage information

## Datasource Configuration Examples

### PostgreSQL Datasource
```json
{
  "name": "Production DB",
  "type": "postgresql",
  "config": {
    "host": "localhost",
    "port": 5432,
    "database": "production",
    "username": "app_user",
    "password": "secure_password",
    "ssl_mode": "require"
  }
}
```

### MySQL Datasource
```json
{
  "name": "Analytics DB",
  "type": "mysql",
  "config": {
    "host": "mysql.example.com",
    "port": 3306,
    "database": "analytics",
    "username": "readonly_user",
    "password": "mysql_password"
  }
}
```

### SQLite Datasource
```json
{
  "name": "Local Data",
  "type": "sqlite",
  "config": {
    "file_path": "/path/to/database.db"
  }
}
```

### API Datasource (Bearer Token)
```json
{
  "name": "Payment API",
  "type": "rest_api",
  "config": {
    "base_url": "https://api.stripe.com/v1",
    "auth": {
      "type": "bearer",
      "token": "sk_live_your_token_here"
    },
    "headers": {
      "Content-Type": "application/json"
    }
  }
}
```

### API Datasource (API Key)
```json
{
  "name": "Weather API",
  "type": "rest_api",
  "config": {
    "base_url": "https://api.openweathermap.org/data/2.5",
    "auth": {
      "type": "api_key",
      "api_key": "your_api_key_here",
      "key_header": "X-API-Key"
    }
  }
}
```

## Usage Examples

### Database Query Example
```
User: "How many users do we have in the production database?"
AI: I'll query the production database for you.
[Tool Execution: database_query(datasource_id: "ds_123", query: "SELECT COUNT(*) as user_count FROM users")]
Result: {"user_count": 1542}
```

### API Request Example
```
User: "Get the current weather in New York"
AI: I'll fetch the weather information for you.
[Tool Execution: api_request(method: "GET", url: "/weather?q=New York&units=metric")]
Result: {"temp": 22, "description": "partly cloudy", "humidity": 65}
```

## Tool Execution Flow

1. **User Request** → AI analyzes request
2. **Tool Selection** → AI selects appropriate tool(s)
3. **Access Control** → `ValidateAccess(userID, projectID)` checks permissions
4. **Datasource Validation** → Tool validates datasource belongs to project
5. **Connection Creation** → ZDB creates connection using datasource config
6. **Execution** → Tool executes with provided parameters
7. **Result Streaming** → Results streamed via WebSocket in real-time
8. **Security Logging** → All tool executions logged

## Security Features

- **Project Isolation**: Tools only access datasources within the same project
- **User Authentication**: All tool executions require valid user session
- **Connection Validation**: Datasource connections validated before use
- **Query Filtering**: Dangerous SQL operations blocked
- **Timeout Protection**: All operations have configurable timeouts

## WebSocket Integration

Tool execution status is broadcast in real-time:

- `tool_execution_started` - Tool begins execution
- `tool_execution_completed` - Tool completes successfully  
- `tool_execution_failed` - Tool encounters an error

Example message:
```json
{
  "type": "tool_execution_completed",
  "data": {
    "tool_name": "database_query",
    "result": {"status": "completed", "data": {"rows": 10}, "time_ms": 245},
    "timestamp": 1640995200000
  }
}
```

## Implementation Details

### Dynamic Tool Registration
Tools are registered per WebSocket server instance and can access any datasource in any project through proper scoping.

### ZDB Integration
All database connections use the ZDB abstraction layer for consistent connection management, pooling, and error handling.

### Error Handling
Tools provide detailed error information with structured error codes and human-readable messages.

### Performance Considerations
- Connections created on-demand with connection pooling
- Configurable timeouts for all operations
- Efficient result serialization for WebSocket streaming

## Future Enhancements

- **Caching**: Intelligent query result caching
- **Bulk Operations**: Batch API requests and database operations
- **Monitoring**: Tool usage analytics and performance metrics
- **File Operations**: S3 and local filesystem tools
- **Webhook Support**: HTTP endpoint creation tools