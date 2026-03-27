# AGENTS.md - Contribution Guidelines

> All agents must read this file before working on the project. Strict adherence required.

---

## Quick Reference

| Topic | Location |
|-------|----------|
| Project Plan | [PROJECT_PLAN.md](PROJECT_PLAN.md) |
| Architecture | [ARCHITECTURE.md](ARCHITECTURE.md) |
| API Spec | [PROJECT_PLAN.md#4-api-specification](PROJECT_PLAN.md#4-api-specification) |
| Tech Stack | [PROJECT_PLAN.md#2-technology-stack](PROJECT_PLAN.md#2-technology-stack) |
| Multipass CLI Ref | [docs/MULTIPASS_API_REFERENCE.md](docs/MULTIPASS_API_REFERENCE.md) |

---

## 1. Git Workflow

### Branch Strategy

```
main      ← Production-ready only
develop   ← Integration branch (PR target)
feature/* ← New features
fix/*     ← Bug fixes
docs/*    ← Documentation
refactor/* ← Code refactoring
```

### Rules

1. All PRs to `develop` branch
2. Delete branch after merge
3. Use strict prefixes: `feature/`, `fix/`, `docs/`, `refactor/`
4. Never push directly to `main` or `develop`

### Process

```bash
# 1. Create branch from develop
git checkout develop
git pull origin develop
git checkout -b feature/your-feature

# 2. Work and commit
# Follow commit format below

# 3. Push and create PR
git push -u origin feature/your-feature
# Create PR via GitHub UI → base: develop

# 4. After approval and merge
git checkout develop
git pull origin develop
git branch -d feature/your-feature
```

---

## 2. Commit Message Format

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

### Types

| Type | Description |
|------|-------------|
| `feat` | New feature |
| `fix` | Bug fix |
| `docs` | Documentation |
| `style` | Formatting |
| `refactor` | Code refactoring |
| `test` | Tests |
| `chore` | Maintenance |

### Examples

```
feat(instances): add instance create endpoint
fix(terminal): resolve websocket disconnect issue
docs(api): update endpoint documentation
```

---

## 3. Coding Standards

### Go (Backend)

- **Linter**: Run `golangci-lint run` before commit
- **Format**: Run `gofmt -w .` before commit
- **Tests**: All new code requires unit tests
- **Error Handling**: Never ignore errors with `_`

**Commands:**
```bash
golangci-lint run ./...
gofmt -w .
go test ./...
```

### Svelte/TypeScript (Frontend)

- **Linter**: ESLint must pass
- **Formatter**: Prettier must pass
- **Types**: TypeScript strict mode, no `any`
- **Tests**: Vitest for unit tests

**Commands:**
```bash
npm run lint
npm run format
npm run test
```

---

## 4. Testing Requirements

### Mandatory Before PR

| Check | Command |
|--------|---------|
| Go tests | `go test ./...` |
| Go lint | `golangci-lint run ./...` |
| Go format | `gofmt -d .` |
| TS tests | `npm run test` |
| ESLint | `npm run lint` |
| Build | `npm run build` |

### Test Coverage

- Minimum 70% for new code (Go + TS)
- E2E tests for critical paths (Phase 3+)

---

## 5. PR Process

### Requirements

1. **Tests must pass** - All checks in section 4
2. **Review required** - Code review by at least 1 maintainer
3. **No merge conflicts** - Keep branch updated with develop
4. **Documentation** - Update docs if changing API

### PR Checklist

- [ ] Branch is up to date with develop
- [ ] All tests passing
- [ ] Lint and format checks passing
- [ ] New tests added for new features
- [ ] Documentation updated
- [ ] PR description explains changes

---

## 6. Security Guidelines

### Never Do

- ❌ Commit secrets/keys/tokens
- ❌ Commit `.env` files
- ❌ Disable security checks
- ❌ Skip input validation
- ❌ Use `any` type in TypeScript

### Always Do

- ✅ Validate all inputs
- ✅ Sanitize user data
- ✅ Use parameterized queries (if DB added)
- ✅ Run security scans (`trivy`)

---

## 7. Code Review Checklist

### For Reviewer

- [ ] Code follows project conventions
- [ ] No security vulnerabilities
- [ ] Tests are adequate
- [ ] Error handling is proper
- [ ] Logging is appropriate
- [ ] No unnecessary dependencies

### For Author

- [ ] Address all review comments
- [ ] Re-run tests after changes
- [ ] Keep PR focused (one feature/fix)

---

## 8. File Naming

| Type | Convention | Example |
|------|------------|---------|
| Go files | snake_case | `instance_handler.go` |
| Go packages | lowercase | `handlers`, `models` |
| Svelte files | kebab-case | `instance-list.svelte` |
| TypeScript files | kebab-case | `api-client.ts` |
| Config files | lowercase | `docker-compose.yml` |
| Test files | `_test.go` / `.test.ts` | `client_test.go` |

---

## 9. Directory Structure

```
api/
├── cmd/server/main.go          # Entry point
├── internal/
│   ├── handlers/               # HTTP handlers
│   ├── middleware/             # Middleware
│   ├── multipass/              # CLI wrapper
│   ├── models/                 # Data models
│   └── config/                 # Configuration

ui/
├── src/
│   ├── lib/
│   │   ├── components/          # UI components
│   │   ├── services/            # API client
│   │   ├── stores/              # Svelte stores
│   │   └── types/               # TypeScript types
│   └── routes/                 # Pages
└── static/
```

---

## 10. Getting Help

| Issue | Resource |
|-------|----------|
| Project details | [PROJECT_PLAN.md](PROJECT_PLAN.md) |
| Architecture | [ARCHITECTURE.md](ARCHITECTURE.md) |
| API specification | [PROJECT_PLAN.md#4-api-specification](PROJECT_PLAN.md#4-api-specification) |
| Multipass CLI Ref | [docs/MULTIPASS_API_REFERENCE.md](docs/MULTIPASS_API_REFERENCE.md) |
| Multipass CLI | [Multipass Docs](https://documentation.ubuntu.com/multipass/) |
| Echo framework | [Echo Docs](https://echo.labstack.com/) |
| Svelte | [Svelte Docs](https://svelte.dev/) |

---

## 11. Testing Standards

### Frameworks

| Language | Framework | Purpose |
|----------|-----------|---------|
| Go | `testify` + `assert` | Unit tests, assertions |
| Svelte | `vitest` + `@testing-library/svelte` | Unit + component tests |

### Testing Approach

**Test like users would use it, then test edge cases:**

```go
// ✅ DO: Test complete user workflows
func TestCreateInstance(t *testing.T) {
    client := NewTestClient()
    
    instance, err := client.CreateInstance(CreateOptions{
        Name: "test-vm",
        CPUs: 2,
    })
    
    assert.NoError(t, err)
    assert.Equal(t, "test-vm", instance.Name)
}

// ✅ DO: Test edge cases
func TestCreateInstance_InvalidName(t *testing.T) {
    client := NewTestClient()
    
    _, err := client.CreateInstance(CreateOptions{
        Name: "123-invalid",
    })
    
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "invalid instance name")
}

// ✅ DO: Test error states
func TestCreateInstance_DuplicateName(t *testing.T) {
    client := NewTestClient()
    _, _ = client.CreateInstance(CreateOptions{Name: "test"})
    
    _, err := client.CreateInstance(CreateOptions{Name: "test"})
    
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "already exists")
}
```

---

## 12. Design Patterns

### Backend (Go)

| Pattern | Use For | Example |
|---------|---------|---------|
| Repository | CLI wrapper abstraction | `MultipassClient` interface |
| Handler | HTTP request processing | `InstanceHandler` |
| Middleware | Cross-cutting concerns | IP whitelist, logging |
| Config | Centralized configuration | `config.yaml` loading |

### Frontend (Svelte)

| Pattern | Use For | Example |
|---------|---------|---------|
| Component | Reusable UI | `Button.svelte`, `Card.svelte` |
| Store | Global state | `instances.ts` store |
| Service | API client | `api.ts` client |
| Type | Shared types | `instance.ts` types |

---

## 13. Negative Space Programming

### Principles

```go
// ✅ DO: Single responsibility functions
func (c *Client) ListInstances() ([]Instance, error)

// ✅ DO: Small, clear structs
type Instance struct {
    Name   string `json:"name"`
    State  string `json:"state"`
}

// ✅ DO: Explicit over implicit
GetRunningInstances()  // ✅ Clear
GetSome()             // ❌ Avoid

// ✅ DO: Early returns (reduce nesting)
func (h *Handler) GetInstance(c echo.Context) error {
    name := c.Param("name")
    if name == "" {
        return echo.NewHTTPError(400, "name required")
    }
    
    // Main logic
    instance, err := h.client.GetInstance(name)
    if err != nil {
        return echo.NewHTTPError(404, "not found")
    }
    
    return c.JSON(200, instance)
}
```

---

## 14. Frontend Design Standards

### Style Guidelines

| Aspect | Standard |
|--------|----------|
| Design | Simple, no unnecessary decoration |
| Layout | Well-lined grids, clear visual structure |
| Colors | Grayscale palette |
| Spacing | Consistent 4px/8px/16px/24px grid |
| Typography | Readable sizes, clear hierarchy |

### Grayscale Palette

| Element | Color |
|---------|-------|
| Background | `#FFFFFF` / `#F5F5F5` |
| Borders | `#E0E0E0` |
| Primary Text | `#212121` |
| Secondary Text | `#757575` |
| Accents | `#424242` (use sparingly) |

### Required Components

Build these reusable components for consistency:

- `Button.svelte` - Button variants (primary, secondary, danger)
- `Card.svelte` - Container component
- `Badge.svelte` - Status indicators
- `Modal.svelte` - Dialog component
- `Table.svelte` - Data display
- `Input.svelte` - Form inputs
- `Select.svelte` - Dropdown
- `Spinner.svelte` - Loading state
- `InstanceCard.svelte` - Instance display
- `InstanceForm.svelte` - Create form

---

## Summary

1. **Branch**: `feature/*` or `fix/*` from `develop`
2. **Commit**: Use conventional format
3. **Test**: All tests + lint must pass
4. **Review**: One approval required
5. **Security**: No secrets, validate inputs
6. **Docs**: Update when changing APIs

**Read [PROJECT_PLAN.md](PROJECT_PLAN.md) for full context before starting work.**
