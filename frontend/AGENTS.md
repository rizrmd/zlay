# Agent Guidelines

this project uses shadcn-vue

## Commands

We use bun

- Frontend dev: `bun run dev`
- Frontend build: `bun run build`
- Frontend test: `bun run test:unit`
- Frontend single test: `npx vitest run <test-file>`
- Frontend lint: `bun run lint`
- Frontend format: `bun run format`
- Backend build: `cd ../backend && zig build`
- Backend run: `cd ../backend && ./webserver`

## Code Style

- Use 2-space indentation, LF line endings
- Max line length: 100 characters
- No semicolons, single quotes for strings
- TypeScript strict mode enabled
- Vue 3 Composition API with `<script setup lang="ts">`
- Pinia for state management
- ESLint + Prettier for formatting
- Import order: external libs, internal modules, relative imports
- Error handling: use try/catch with proper typing
- Naming: camelCase for variables, PascalCase for components
