# Agent Guidelines for Zlay Project

## Build/Lint/Test Commands

### Frontend (Vue 3 + TypeScript)
- **Build**: `npm run build` (includes type-check)
- **Build only**: `npm run build-only`
- **Type check**: `npm run type-check`
- **Lint**: `npm run lint` (auto-fixes with ESLint)
- **Format**: `npm run format` (Prettier)
- **Unit tests**: `npm run test:unit` (Vitest)
- **E2E tests**: `npm run test:e2e` (Nightwatch)
- **Single test**: `vitest run src/path/to/test.spec.ts`

### Backend (Go)
- **Build**: `cd backend && go build`
- **Test all**: `cd backend && go test ./...`
- **Single test**: `cd backend && go test -run TestName ./path/to/package`
- **Lint**: `golangci-lint run` (if installed)

## Code Style Guidelines

### Frontend (Vue 3 + TypeScript)
- **Vue**: Composition API preferred, avoid Options API
- **TypeScript**: Strict mode enabled, full type safety
- **Imports**: Use path aliases `@/*` for `src/*`
- **Formatting**: Prettier (no semicolons, single quotes, 100 char width)
- **Components**: PascalCase naming, `.vue` extension
- **Props/Emits**: Typed interfaces, defineProps/defineEmits
- **Styling**: Tailwind CSS with utility classes
- **Error handling**: Try-catch in async operations, proper error states

### Backend (Go)
- **Formatting**: `gofmt` standard formatting
- **Error handling**: Wrap errors with context, no panics for business logic
- **Context**: Pass context.Context to all blocking operations
- **Interfaces**: Accept interfaces, return concrete types
- **Concurrency**: Channels for orchestration, mutexes for state
- **Testing**: Table-driven tests with subtests
- **Naming**: Standard Go conventions (camelCase private, PascalCase exported)

### General
- **Commits**: Run lint/type-check before committing
- **Security**: Never log/commit secrets, validate all inputs
- **Performance**: Profile before optimizing, use benchmarks