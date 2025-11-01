# CRUSH - Development Guide

This document provides comprehensive guidance for agents working with the Zlay platform - a multi-tenant AI chat platform with real-time WebSocket communication and extensible tool integration.

## Project Overview

**Zlay Platform** is a full-stack application consisting of:
- **Backend**: Go-based HTTP API + WebSocket server with PostgreSQL database
- **Frontend**: Vue.js 3 + TypeScript with shadcn-vue UI components
- **Architecture**: Multi-tenant with client/domain isolation, real-time chat, AI integration

## Essential Commands

### Frontend (Vue.js)
```bash
cd frontend
bun run dev          # Development server
bun run build        # Production build
bun run test:unit    # Unit tests with Vitest
npx vitest run <file> # Run specific test file
bun run lint         # ESLint + auto-fix
bun run format       # Prettier formatting
```

### Backend (Go)
```bash
cd backend
go run main.go       # Development server (uses main.go, not src/main.zig)
go build             # Build executable
go test ./...        # Run all tests
```

### Database Operations
- **Connection**: Uses internal ZDB abstraction layer - NEVER use database/sql directly
- **Migrations**: Located in backend/ directory (check schema.sql)
- **Seeding**: Contact user for database seeding operations

## Code Architecture

### Backend Structure
```
backend/
├── main/                    # HTTP handlers and application entry
│   ├── main.go             # Main application with server setup
│   ├── auth.go             # Authentication endpoints
│   ├── projects.go         # Project CRUD operations
│   ├── datasources.go      # Datasource management
│   └── admin.go            # Admin-only endpoints
├── internal/               # Internal packages
│   ├── websocket/          # WebSocket server and handlers
│   ├── chat/              # Chat service and AI integration
│   ├── db/                 # **CRITICAL**: ZDB database abstraction layer
│   ├── llm/               # LLM integration (OpenAI)
│   └── tools/             # Tool system registry
└── go.mod, go.sum         # Go dependencies
```

### Frontend Structure
```
frontend/
├── src/
│   ├── views/             # Main application views
│   ├── components/        # Vue components (shadcn-vue)
│   ├── stores/            # Pinia state management
│   ├── services/          # API and WebSocket services
│   ├── router/            # Vue Router configuration
│   └── lib/               # Utilities (shadcn-vue utils)
├── tests/                 # Test files (e2e and unit)
└── components.json         # shadcn-vue configuration
```

## Critical Rules & Gotchas

### 🔴 CRITICAL: Database Operations
- **NEVER** use `database/sql` or `sql.DB` directly
- **ALWAYS** use the ZDB abstraction in `internal/db/`
- All database operations must go through ZDB methods like `Query()`, `QueryRow()`, `Execute()`
- Use single-line SQL queries to avoid multiline string syntax errors
- **ALWAYS** use `RETURNING` clause for INSERT operations to get generated IDs

### 🔴 CRITICAL: UUID Type Handling
- PostgreSQL UUID fields are problematic with `lib/pq` driver
- UUIDs may return as `[]byte` instead of strings
- Current workaround in `internal/db/types.go` handles 16-byte arrays as UUIDs
- **Recommended**: Switch to `github.com/jackc/pgx/v5/stdlib` driver for proper UUID support

### WebSocket Architecture
- HTTP handles authentication, REST APIs
- WebSocket handles real-time chat with streaming responses
- Messages follow AsyncAPI specification in `api/asyncapi.yaml`
- Authentication required for WebSocket connections via token parameter

### Multi-Tenant Architecture
- Client-based isolation with domain routing
- Domain cache in main.go maps domains to client IDs
- All operations scoped to user + client + project

## Code Style & Patterns

### Backend (Go)
- Standard Go formatting with `gofmt`
- Explicit error handling with meaningful returns
- Naming: camelCase for variables, PascalCase for types
- Use context.Context for all database operations
- Middleware pattern for authentication and CORS

### Frontend (Vue.js)
- **2-space indentation**, LF line endings, max 100 chars
- **No semicolons**, single quotes for strings
- **ALWAYS** use shadcn-vue UI components for consistency
- Vue 3 Composition API with `<script setup lang="ts">`
- Pinia for state management
- TypeScript strict mode enabled

### Import Order (Frontend)
1. External libraries (Vue, third-party)
2. Internal modules (@/services, @/stores)
3. Relative imports (./components, ../lib)

## Performance Guidelines

### Database Operations
```go
// GOOD: Single query with RETURNING
row, err := db.QueryRow(ctx, "INSERT INTO users (name) VALUES ($1) RETURNING id", name)

// BAD: Two round trips
db.Execute(ctx, "INSERT INTO users (name) VALUES ($1)", name)
row, err := db.QueryRow(ctx, "SELECT id FROM users WHERE name = $1", name)
```

### Query Optimization
- Keep SQL queries on single lines
- Use parameter binding, never string concatenation
- Batch operations in transactions when possible
- Monitor slow operations (>100ms database, >50ms API)

## Testing Strategy

### Backend Testing
- Unit tests for database operations
- Integration tests for API endpoints
- Test database operations with actual PostgreSQL connection
- Mock external services (OpenAI, external APIs)

### Frontend Testing
- Vitest for unit tests
- Nightwatch for e2e tests
- Component testing with Vue Test Utils
- Test WebSocket connections with mock servers

## Authentication & Security

### Session Management
- Cookie-based sessions with 24-hour expiration
- Tokens stored as SHA-256 hash in database
- WebSocket authentication via query token parameter
- CORS configured for cross-origin requests

### Security Patterns
- Never log sensitive information (passwords, tokens)
- Validate all inputs at API boundaries
- Use parameterized queries to prevent SQL injection
- HTTPS in production (enforce secure cookies)

## WebSocket Message Types

### Core Messages
- `user_message`: User sends chat content
- `assistant_response`: AI streaming response chunks
- `create_conversation`: Start new chat session
- `get_conversations`: List user conversations
- `join_project`: Join project-based chat room
- `tool_execution_started/completed/failed`: Tool execution notifications

### Message Format
```json
{
  "type": "message_type",
  "data": { /* message payload */ },
  "timestamp": 1640995200000
}
```

## API Endpoints Overview

### Authentication (`/api/auth/`)
- `POST /register` - User registration
- `POST /login` - User login with session creation
- `POST /logout` - Session termination
- `GET /profile` - Current user profile

### Projects (`/api/projects/`)
- Standard CRUD operations with authentication middleware
- Scoped to user + client

### Datasources (`/api/datasources/`)
- Database and API endpoint configuration
- Used by chat tools for data access

### Admin (`/api/admin/`)
- Client and domain management
- Requires admin middleware

## Development Workflow

### Making Changes
1. **Backend**: Start with ZDB layer, then handlers, then main.go setup
2. **Frontend**: Update components, then stores/state, then views
3. **WebSocket**: Update message handlers, then AsyncAPI spec
4. **Database**: Update schema.sql, then ZDB types, then handlers

### Testing After Changes
1. Run relevant unit tests
2. Test API endpoints with curl or Postman
3. Test WebSocket connections
4. Verify frontend integration
5. Check browser console for errors

## Common Issues & Solutions

### UUID as Byte Array Issue
**Problem**: PostgreSQL UUIDs return as `[]byte` instead of strings
**Current Fix**: Handled in `internal/db/types.go` byte array conversion
**Better Fix**: Switch to `pgx/v5/stdlib` driver

### WebSocket Authentication Failures
**Check**: Token format, session expiration, database connection
**Debug**: Look at main.go:loadDomainCache() for client ID mapping

### CORS Issues
**Location**: main.go InitRouter() CORS configuration
**Common Fix**: Add required headers to `AllowHeaders` array

### Frontend Build Issues
**Check**: Node version compatibility, bun.lock up-to-date
**Common Fix**: `bun install` to refresh dependencies

## Environment Configuration

### Backend (.env)
```
DATABASE_URL=postgresql://postgres:password@localhost:5432/zlay
PORT=8080
WS_PORT=6070
```

### Frontend Development
- Vite dev server: http://localhost:5173
- Proxy configuration in vite.config.ts for API calls

## Documentation References

- **OpenAPI Spec**: `api/openapi.yaml` (REST API)
- **AsyncAPI Spec**: `api/asyncapi.yaml` (WebSocket API)
- **Database Schema**: `backend/schema.sql`
- **Frontend Pivot Table**: `docs/FRONTEND_PIVOT_TABLE.md`
- **WebSocket Implementation**: `docs/WEBSOCKET_CHAT_IMPLEMENTATION.md`

## Tool System Architecture

### Tool Registration
- Tools registered in `internal/tools/registry.go`
- Interface defined in `internal/tools/interface.go`
- Built-in tools: database queries, HTTP requests, system operations

### Tool Execution
- Triggered by AI via function calling
- Results streamed back through WebSocket
- Error handling with detailed error codes and messages

## Migration Guides

### When Adding New API Endpoints
1. Add route in `main.go` InitRouter()
2. Implement handler function
3. Add authentication middleware if needed
4. Update OpenAPI specification
5. Add corresponding frontend service methods

### When Adding New WebSocket Messages
1. Define message type in handler.go switch statement
2. Implement handler function
3. Update AsyncAPI specification
4. Add frontend WebSocket message handling
5. Update message types in stores

This document should serve as the definitive guide for understanding and working with the Zlay platform codebase.