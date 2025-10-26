# Zlay Backend

A high-performance web server built with Zig, featuring multi-tenant authentication and WebSocket support.

## Project Structure

```
backend/
├── src/
│   ├── auth/              # Authentication modules
│   │   ├── types.zig     # Core types and error definitions
│   │   ├── password.zig  # Password hashing/verification
│   │   ├── token.zig     # Token generation/hashing
│   │   ├── database.zig  # Database operations
│   │   └── auth.zig     # High-level auth functions
│   ├── auth.zig          # Main auth module (re-exports)
│   ├── main.zig          # Main server application
│   ├── hotreload.zig     # Hot reload functionality
│   └── openai.zig        # OpenAI integration
├── test/                 # Test files
├── old/                  # Backup of old files
├── schema.sql            # Database schema
├── migration.sql         # Database migrations
├── build.zig             # Build configuration
├── build.zig.zon         # Dependencies
├── openapi.yaml          # API documentation
└── .gitignore            # Git ignore patterns
```

## Features

- **Multi-tenant Authentication**: Client-isolated user management
- **Secure**: SHA-256 salted password hashing, secure random tokens
- **Modular Architecture**: Clean separation of concerns
- **WebSocket Support**: Real-time communication
- **Hot Reload**: Development-friendly auto-restart
- **OpenAPI Documentation**: Complete API specification

## Quick Start

### Prerequisites

- Zig 0.15.1+
- PostgreSQL 12+
- libpq (PostgreSQL client library)

### Installation

1. Clone the repository:

```bash
git clone <repository-url>
cd backend
```

2. Install dependencies:

```bash
zig fetch --save https://github.com/karlseguin/pg.zig
```

3. Setup database:

```bash
createdb zlay
psql -d zlay -f schema.sql
```

4. Build and run:

```bash
# Development mode with hot reload
zig build dev

# Production mode
zig build run

# Or build executable
zig build
./zig-out/bin/zlay-backend
```

## API Endpoints

### Authentication

- `POST /api/auth/register` - User registration
- `POST /api/auth/login` - User authentication
- `POST /api/auth/logout` - User logout

### Headers

- `X-Client-ID` - Required for multi-tenant requests

## Development

### Build Commands

```bash
# Development build
zig build

# Production build
zig build prod

# Run with hot reload
zig build dev

# Clean build artifacts
zig build clean
```

### Testing

```bash
# Run tests
zig test src/

# Run specific test
zig test src/auth.zig
```

## Configuration

The server uses environment variables for configuration:

- `DATABASE_URL` - PostgreSQL connection string
- `HOST` - Server host (default: 127.0.0.1)
- `PORT` - Server port (default: 8080)

## Architecture

### Authentication System

The authentication system is modularized into several components:

1. **Types** (`src/auth/types.zig`): Core data structures and error types
2. **Password** (`src/auth/password.zig`): SHA-256 salted password hashing
3. **Token** (`src/auth/token.zig`): Secure token generation and validation
4. **Database** (`src/auth/database.zig`): Database operations for users/sessions
5. **Auth** (`src/auth/auth.zig`): High-level authentication functions

### Multi-Tenancy

Each client has isolated user management:

- Users are scoped to specific clients
- Client ID is required for all auth operations
- Database constraints prevent cross-client data access

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

[Your License Here]
