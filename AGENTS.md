# Agent Guidelines

This is a full-stack project with Zig backend and Vue.js frontend using shadcn-vue.

## Commands

**Frontend (Vue.js):**
- Dev: `cd frontend && bun run dev`
- Build: `cd frontend && bun run build`
- Test: `cd frontend && bun run test:unit`
- Single test: `cd frontend && npx vitest run <test-file>`
- Lint: `cd frontend && bun run lint`
- Format: `cd frontend && bun run format`

**Backend (Zig):**
- Build: `cd backend && zig build`
- Run: `cd backend && zig run src/main.zig`
- Test: `cd backend/zlay-db && zig build test`
- Library check: `cd backend/zlay-db && zig build setup`

## Code Style

**Frontend:**
- 2-space indentation, LF line endings, max 100 chars
- No semicolons, single quotes, TypeScript strict mode
- Vue 3 Composition API with `<script setup lang="ts">`
- Import order: external libs, internal modules, relative imports
- Naming: camelCase for variables, PascalCase for components
- **ALWAYS** use shadcn-vue UI components for consistent design

**Backend (Go):**
- **CRITICAL**: **ALWAYS** use ZDB for any database interactions - never use DB or SQLDB directly
- ZDB is the single source of truth for database operations across the entire application
- All HTTP API endpoints, WebSocket authentication, and database queries must use ZDB
- Follow standard Go formatting with gofmt
- Use explicit error handling, prefer meaningful error returns
- Naming: camelCase for variables, PascalCase for types

**Backend (Zig):**
- Follow Zig style guide (4-space indentation)
- Use explicit error handling with `try` and `catch`
- Prefer `const` over `var`, use `defer` for cleanup
- Module imports with `@import`, clear error types
- Naming: snake_case for functions, PascalCase for types

## Performance Guidelines

**Database Operations:**
- **NEVER** use INSERT + SELECT pattern to get auto-generated IDs
- **ALWAYS** use `RETURNING` clause for PostgreSQL to get inserted values
- Minimize database round trips - combine operations when possible
- Use `ON CONFLICT DO NOTHING/UPDATE` instead of separate existence checks
- Batch multiple operations in single transactions when feasible

**Query Optimization:**
- Use single queries instead of multiple sequential queries
- Avoid unnecessary string allocations in hot paths
- Prefer parameter binding over string concatenation
- Use appropriate indexes for frequently queried columns
- **CRITICAL**: Keep SQL queries on single lines to avoid syntax errors with multiline strings

**Authentication Performance:**
- Session creation must use `RETURNING id` to eliminate extra query
- Password hashing is expensive but unavoidable - optimize other parts
- Token generation should be cached when possible
- Consider connection pooling for database operations

**Performance Testing:**
- Always measure before optimizing - use `curl -w "%{time_total}"` for API endpoints
- Test with realistic data sizes and concurrent load
- Profile database queries to identify bottlenecks
- Monitor memory allocations in request handlers

**Critical Anti-Patterns to Avoid:**
```zig
// BAD: Two round trips to database
db.exec("INSERT INTO sessions ...");
db.query("SELECT id FROM sessions WHERE ...");

// GOOD: Single round trip with RETURNING
db.query("INSERT INTO sessions ... RETURNING id");

// BAD: Multiline SQL queries that can cause syntax errors
db.query(
    \\SELECT * FROM users
    \\WHERE active = true
    \\  AND created_at > NOW()
);

// GOOD: Single line SQL queries
db.query("SELECT * FROM users WHERE active = true AND created_at > NOW()");
```

**Memory Safety:**
- Always handle `OutOfMemory` errors in switch statements
- Use `errdefer` for cleanup in allocation-heavy functions
- Fix `@memcpy` calls with proper slice boundaries: `@memcpy(dest[0..src.len], src)`

**Performance Monitoring:**
- Log slow operations (>100ms for database, >50ms for API calls)
- Monitor request/response times in production
- Set up alerts for performance degradation
- Regular performance regression testing

**Project Management Rules:**
- **NEVER** change DATABASE_URL environment variable
- **NEVER** run the project yourself - let the user manage it
- **NEVER** start the backend or frontend services by yourself
- **NEVER** remove node_modules or package-lock.json
- Always ask user to start/stop services when needed
- Test with provided curl commands only when user explicitly requests