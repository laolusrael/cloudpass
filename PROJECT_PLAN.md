# CloudPass - Project Plan

> Production-grade web management UI for Multipass

---

## 1. Executive Summary

CloudPass is an open-source web-based management interface for Canonical's Multipass virtualization tool. It provides a REST API wrapper around the multipass CLI and a modern web UI for managing Ubuntu virtual machines.

**Mission**: Make Multipass accessible through a user-friendly interface while maintaining production-grade code quality.

---

## 2. Technology Stack

| Layer | Technology | Purpose |
|-------|------------|---------|
| **Backend** | Go + Echo | REST API server |
| **Frontend** | Svelte + TypeScript | Web UI |
| **Build** | Vite | Frontend bundling |
| **Styling** | Tailwind CSS | UI styling |
| **Terminal** | xterm.js | Web shell access |
| **Container** | Docker + Kubernetes | Deployment |
| **Testing** | Go testing + Vitest | Unit tests |
| **E2E** | Playwright | Integration tests |

---

## 3. Architecture

### 3.1 High-Level Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                        Browser (Svelte)                      │
└─────────────────────┬───────────────────────────────────────┘
                      │ REST API / WebSocket
                      ▼
┌─────────────────────────────────────────────────────────────┐
│                    Go + Echo API Server                      │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │   Middleware │  │  Handlers   │  │  Multipass Client   │ │
│  │ - IP Filter │  │ - Instance  │  │  - CLI Executor     │ │
│  │ - Logging   │  │ - Network   │  │  - Output Parser   │ │
│  │             │  │ - Terminal  │  │                     │ │
│  └─────────────┘  └─────────────┘  └─────────────────────┘ │
└─────────────────────┬───────────────────────────────────────┘
                      │ exec/pipe
                      ▼
┌─────────────────────────────────────────────────────────────┐
│                    multipass CLI (system)                    │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 Project Structure

```
cloudpass/
├── api/                          # Go backend
│   ├── cmd/server/
│   │   └── main.go               # Entry point
│   ├── internal/
│   │   ├── config/
│   │   │   └── config.go
│   │   ├── handlers/
│   │   │   ├── instance.go
│   │   │   ├── network.go
│   │   │   ├── terminal.go
│   │   │   └── health.go
│   │   ├── middleware/
│   │   │   ├── ip_whitelist.go
│   │   │   └── logging.go
│   │   ├── multipass/
│   │   │   ├── client.go
│   │   │   ├── executor.go
│   │   │   └── parser.go
│   │   ├── models/
│   │   │   └── instance.go
│   │   └── websocket/
│   │       ├── hub.go
│   │       └── terminal.go
│   ├── go.mod
│   ├── go.sum
│   ├── Dockerfile
│   └── config.yaml
├── ui/                           # Svelte frontend
│   ├── src/
│   │   ├── lib/
│   │   │   ├── components/       # Reusable UI components
│   │   │   ├── services/        # API client
│   │   │   ├── stores/          # Svelte stores
│   │   │   └── types/           # TypeScript types
│   │   ├── routes/              # SvelteKit pages
│   │   │   ├── +page.svelte
│   │   │   ├── instances/
│   │   │   └── settings/
│   │   └── app.html
│   ├── static/
│   ├── package.json
│   ├── svelte.config.js
│   ├── vite.config.ts
│   ├── tailwind.config.js
│   ├── tsconfig.json
│   ├── Dockerfile
│   └── .eslintrc.cjs
├── docker-compose.yml
├── kubernetes/
│   ├── deployment.yaml
│   ├── service.yaml
│   └── configmap.yaml
├── .github/
│   └── workflows/
│       ├── ci.yml
│       └── release.yml
├── README.md
├── CONTRIBUTING.md
├── AGENTS.md
├── LICENSE
└── PROJECT_PLAN.md
```

---

## 4. API Specification

### 4.1 Base URL

```
http://localhost:8080/api
```

### 4.2 Endpoints

| Method | Endpoint | Description | Phase |
|--------|----------|-------------|-------|
| `GET` | `/health` | Health check | 1 |
| `GET` | `/instances` | List all instances | 1 |
| `POST` | `/instances` | Create new instance | 1 |
| `GET` | `/instances/:name` | Get instance details | 1 |
| `DELETE` | `/instances/:name` | Delete instance | 1 |
| `POST` | `/instances/:name/start` | Start instance | 1 |
| `POST` | `/instances/:name/stop` | Stop instance | 1 |
| `POST` | `/instances/:name/restart` | Restart instance | 1 |
| `POST` | `/instances/:name/suspend` | Suspend instance | 2 |
| `WS` | `/instances/:name/shell` | WebSocket terminal | 2 |
| `GET` | `/images` | List available images | 1 |
| `GET` | `/networks` | List available networks | 1 |
| `POST` | `/networks` | Create network bridge | 2 |
| `DELETE` | `/networks/:name` | Delete network | 2 |
| `POST` | `/instances/:name/export` | Export VM | 3 |
| `POST` | `/instances/import` | Import VM | 3 |
| `POST` | `/instances/:name/snapshots` | Create snapshot | 3 |
| `GET` | `/instances/:name/snapshots` | List snapshots | 3 |
| `POST` | `/instances/:name/snapshots/:id/restore` | Restore snapshot | 3 |

### 4.3 Data Models

#### Instance

```json
{
  "name": "primary",
  "state": "Running",
  "ipv4": ["192.168.64.2"],
  "cpu": 2,
  "memory": "2G",
  "disk": "10G",
  "image": "22.04",
  "created_at": "2024-01-15T10:30:00Z"
}
```

#### Create Instance Request

```json
{
  "name": "my-vm",
  "image": "22.04",
  "cpus": 2,
  "memory": "2G",
  "disk": "10G",
  "network": "default",
  "cloud_init": "#cloud-config\n..."
}
```

#### Network

```json
{
  "name": "br0",
  "type": "bridge",
  "ipv4": "192.168.64.1/24",
  "description": "Bridge network"
}
```

### 4.4 WebSocket Protocol

**Connection**: `ws://localhost:8080/api/instances/:name/shell`

```typescript
// Client -> Server
{ "type": "start", "cols": 80, "rows": 24 }
{ "type": "input", "data": "ls\n" }
{ "type": "resize", "cols": 120, "rows": 30 }
{ "type": "stop" }

// Server -> Client
{ "type": "output", "data": "ubuntu@primary:~$ " }
{ "type": "error", "data": "command not found" }
{ "type": "exit", "code": 0 }
```

---

## 5. Feature Roadmap

### Phase 1: MVP (Weeks 1-2)

- [ ] Project initialization
- [ ] Go + Echo setup with logging
- [ ] IP whitelist middleware
- [ ] Multipass CLI wrapper
- [ ] Instance CRUD endpoints
- [ ] Start/Stop/Restart endpoints
- [ ] Image listing endpoint
- [ ] Network listing endpoint
- [ ] Svelte + Vite setup
- [ ] Dashboard page
- [ ] Instance creation form
- [ ] Instance details page
- [ ] Docker compose setup

### Phase 2: Advanced Lifecycle (Weeks 2-3)

- [ ] Suspend/Resume endpoints
- [ ] WebSocket terminal implementation
- [ ] Network creation API
- [ ] Terminal UI (xterm.js)
- [ ] Network management UI
- [ ] Real-time status updates
- [ ] Kubernetes manifests

### Phase 3: Advanced Features (Weeks 3-4)

- [ ] VM Export/Import
- [ ] Snapshot management
- [ ] Mount management
- [ ] Cloud-init editor
- [ ] Playwright E2E tests
- [ ] Release workflow

---

## 6. Configuration

### 6.1 Server Configuration (config.yaml)

```yaml
server:
  host: "0.0.0.0"
  port: 8080

security:
  allowed_ips:
    - "192.168.1.0/24"
    - "10.0.0.0/8"
  websocket:
    idle_timeout_minutes: 30

multipass:
  socket_path: "/var/run/multipass_socket"
  default_timeout_seconds: 300

logging:
  level: "info"
  format: "json"
```

### 6.2 Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `CLOUDPASS_CONFIG` | Config file path | `./config.yaml` |
| `CLOUDPASS_LOG_LEVEL` | Log level | `info` |
| `CLOUDPASS_PORT` | Server port | `8080` |

---

## 7. Deployment

### 7.1 Docker

```bash
# Build
docker build -t cloudpass/api ./api
docker build -t cloudpass/ui ./ui

# Run
docker-compose up -d
```

### 7.2 Kubernetes

```bash
kubectl apply -f kubernetes/
```

---

## 8. Development Workflow

See [AGENTS.md](AGENTS.md) for detailed contribution guidelines including:

- Git workflow
- Coding standards
- Testing requirements
- PR process

---

## 9. License

MIT License - See [LICENSE](LICENSE)

---

## 10. Resources

- [Multipass Documentation](https://documentation.ubuntu.com/multipass/)
- [Echo Framework](https://echo.labstack.com/)
- [Svelte](https://svelte.dev/)
- [xterm.js](https://xtermjs.org/)
