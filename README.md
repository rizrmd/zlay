# Zlay Platform

A full-stack platform providing AI-powered chat capabilities with multi-tenant architecture, real-time communication, and extensible tool integration.

## Architecture

- **Backend**: Go with PostgreSQL database
- **Frontend**: Vue.js 3 with TypeScript
- **Real-time**: WebSocket-based chat system
- **Multi-tenant**: Client and domain-based isolation

## Documentation

### API Documentation

#### REST API
- **OpenAPI Specification**: `api/openapi.yaml`
- Describes all HTTP endpoints for authentication, projects, datasources, and admin operations
- Interactive documentation available at `/api/docs` when running the server

#### WebSocket API  
- **AsyncAPI Specification**: `api/asyncapi.yaml`
- Documents real-time chat functionality with message types and communication patterns
- WebSocket endpoint: `ws://localhost:6070/ws/chat`

#### Integration Examples

The platform uses a hybrid API approach:
- REST endpoints for CRUD operations and session management
- WebSocket connections for real-time chat and streaming responses

```javascript
// Example: Combined usage
const session = await fetch('/api/auth/login', {
  method: 'POST',
  body: JSON.stringify({ username: 'user', password: 'pass' })
});

const { token } = await session.json();

// Connect to WebSocket with auth token
const ws = new WebSocket(`ws://localhost:6070/ws/chat?token=${token}&project=proj-123`);
```

### Development Documentation

- **Frontend Development**: `docs/FRONTEND_PIVOT_TABLE.md`
- **WebSocket Implementation**: `docs/WEBSOCKET_CHAT_IMPLEMENTATION.md`

## Quick Start

### Prerequisites
- Go 1.19+
- Node.js 18+
- PostgreSQL 14+
- Bun (for frontend)

### Backend Setup
```bash
cd backend
cp .env.example .env
# Edit .env with your database configuration
go run main.go
```

### Frontend Setup
```bash
cd frontend
bun install
bun run dev
```

### Environment Variables

#### Backend (.env)
```
DATABASE_URL=postgresql://postgres:password@localhost:5432/zlay
PORT=8080
WS_PORT=6070
```

#### Frontend
Environment variables are configured in `frontend/.env`

## Key Features

### Multi-Tenant Architecture
- Client-based isolation with domain routing
- User-scoped projects and conversations
- Admin management for client provisioning

### Real-time Chat System
- WebSocket-based bidirectional communication
- Project-based room isolation
- Streaming AI responses
- Tool execution framework

### AI Integration
- OpenAI-compatible API integration
- Configurable models and endpoints per project
- Function calling for tool integration

### Tool System
- Database query tools
- HTTP API integration
- File system operations
- Extensible tool registry

## Project Structure

```
zlay/
├── api/                    # API specifications
│   ├── openapi.yaml       # REST API documentation
│   └── asyncapi.yaml      # WebSocket API documentation
├── backend/
│   ├── internal/          # Internal packages
│   │   ├── websocket/     # WebSocket implementation
│   │   ├── chat/          # Chat service
│   │   └── ...           # Other services
│   ├── main.go           # Application entry point
│   └── migrations/       # Database migrations
├── frontend/
│   ├── src/              # Vue.js source
│   └── dist/             # Build output
└── docs/                 # Documentation
```

## Development Commands

### Frontend
- Development: `bun run dev`
- Build: `bun run build`
- Test: `bun run test:unit`
- Lint: `bun run lint`

### Backend
- Run: `go run main.go`
- Build: `go build`
- Test: `go test ./...`

## API Overview

### Authentication
- Cookie-based session management
- 24-hour session duration
- Secure cookie configuration

### WebSocket Message Types
- `user_message`: User sends chat message
- `assistant_response`: AI streaming response
- `create_conversation`: Start new conversation
- `get_conversations`: List conversations
- `join_project`: Join project room

### REST Endpoints
- `/api/auth/*`: Authentication endpoints
- `/api/projects/*`: Project management
- `/api/datasources/*`: Datasource configuration
- `/api/admin/*`: Admin-only operations

## Security Considerations

- Session tokens are hashed in database
- Projects are isolated by user and client
- WebSocket connections require authentication
- CORS configuration for cross-origin requests

## Performance Guidelines

- Use single-line SQL queries to avoid syntax errors
- Implement RETURNING clauses for database operations
- Connection pooling for WebSocket and database operations
- Rate limiting and monitoring in production

## Contributing

1. Follow the existing code style
2. Add tests for new features
3. Update documentation as needed
4. Use semantic commit messages

## License

Private project - all rights reserved.
