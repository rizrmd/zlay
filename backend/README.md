# Zlay Backend (Go)

A high-performance REST API backend built with Go and Gin framework, replacing the previous Zig implementation.

## Features

- **Authentication & Authorization**: JWT-based sessions with bcrypt password hashing
- **Multi-tenant Architecture**: Client and domain-based isolation
- **RESTful API**: Full CRUD operations for clients, domains, users, projects, and datasources
- **PostgreSQL**: High-performance database with connection pooling
- **Security**: CORS, secure cookies, SQL injection prevention
- **Performance**: Optimized queries with single-round-trip operations

## Quick Start

### Prerequisites

- Go 1.21 or later
- PostgreSQL 12 or later
- Bun (for frontend)

### Installation

1. Install dependencies:
```bash
cd backend
go mod download
```

2. Set up environment:
```bash
cp .env.example .env
# Edit .env with your database configuration
```

3. Initialize database:
```bash
psql $DATABASE_URL -f schema.sql
```

4. Run the server:
```bash
# Development
go run .

# Production
go build -o zlay-backend .
./zlay-backend
```

## API Endpoints

### Authentication
- `POST /api/auth/register` - Register new user
- `POST /api/auth/login` - User login
- `POST /api/auth/logout` - User logout
- `GET /api/auth/profile` - Get user profile (authenticated)

### Projects
- `GET /api/projects` - List user projects
- `POST /api/projects` - Create project
- `GET /api/projects/:id` - Get project
- `PUT /api/projects/:id` - Update project
- `DELETE /api/projects/:id` - Delete project

### Datasources
- `GET /api/datasources` - List datasources
- `POST /api/datasources` - Create datasource
- `GET /api/datasources/:id` - Get datasource
- `PUT /api/datasources/:id` - Update datasource
- `DELETE /api/datasources/:id` - Delete datasource

### Admin (root user only)
- `GET /api/admin/clients` - List clients
- `POST /api/admin/clients` - Create client
- `PUT /api/admin/clients/:id` - Update client
- `DELETE /api/admin/clients/:id` - Delete client
- `GET /api/admin/domains` - List domains
- `POST /api/admin/domains` - Create domain
- `PUT /api/admin/domains/:id` - Update domain
- `DELETE /api/admin/domains/:id` - Delete domain

### Health
- `GET /api/health` - Health check

## Default Credentials

- **Root User**: username: `root`, password: `12345678`

## Performance Features

- Single database round-trips for operations using RETURNING clause
- Connection pooling with pgxpool
- Efficient CORS handling
- Memory-efficient JSON handling
- Prepared statement caching

## Development

The development server automatically serves the frontend from `../frontend/dist`. Run the full development stack with:

```bash
cd .. && bun run dev.ts
```

## Security

- Passwords hashed with bcrypt
- Session tokens stored as SHA-256 hashes
- SQL injection prevention with parameterized queries
- CORS configuration for cross-origin requests
- Secure cookie configuration
