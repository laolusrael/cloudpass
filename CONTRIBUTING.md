# Contributing to CloudPass

Thank you for your interest in contributing to CloudPass!

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/YOUR_USERNAME/cloudpass.git`
3. Create a feature branch: `git checkout -b feature/your-feature`

## Development Setup

### Backend (Go)

```bash
cd api
go build -o cloudpass ./cmd/server
./cloudpass
```

### Frontend (Svelte)

```bash
cd ui
npm install
npm run dev
```

## Code Style

### Go

- Run `gofmt` before committing
- Run `golangci-lint run ./...` to check for issues
- Write tests for new code

### Svelte/TypeScript

- Run `npm run lint` before committing
- Run `npm run format` to format code
- Write tests for new components

## Testing

```bash
# Backend tests
cd api && go test ./...

# Frontend tests
cd ui && npm run test

# E2E tests
cd ui && npm run test:e2e
```

## Git Workflow

1. Create branch from `develop`
2. Make changes with clear commit messages
3. Push and create PR to `develop`

## Commit Message Format

```
<type>(<scope>): <description>

[optional body]
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`

## Pull Request Process

1. Update documentation for any changes
2. Ensure all tests pass
3. Request review from maintainers

## Questions?

Open an issue for questions about contributing.
