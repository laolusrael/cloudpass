# CloudPass Architecture

> Technical design document for CloudPass - Multipass Management UI

---

## Table of Contents

1. [System Overview](#1-system-overview)
2. [API Design](#2-api-design)
3. [WebSocket Protocol](#3-websocket-protocol)
4. [Component Design](#4-component-design)
5. [Data Flow](#5-data-flow)
6. [Security](#6-security)
7. [Error Handling](#7-error-handling)
8. [Configuration](#8-configuration)
9. [Testing Strategy](#9-testing-strategy)
10. [Deployment](#10-deployment)

---

## 1. System Overview

### 1.1 High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                              Browser                                 │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │                     Svelte Application                         │  │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────────────┐  │  │
│  │  │Dashboard│  │Instance │  │Settings │  │   Terminal      │  │  │
│  │  │  Page   │  │ Details │  │  Page   │  │ (xterm.js)     │  │  │
│  │  └────┬────┘  └────┬────┘  └────┬────┘  └────────┬────────┘  │  │
│  │       │            │            │                 │            │  │
│  │       └────────────┴───────────┴─────────────────┘            │  │
│  │                           │                                    │  │
│  │                    ┌──────┴──────┐                             │  │
│  │                    │ API Service │                             │  │
│  │                    └──────┬──────┘                             │  │
│  └───────────────────────────│───────────────────────────────────┘  │
└──────────────────────────────│─────────────────────────────────────┘
                               │ HTTP/WebSocket
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         Go API Server                                │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │                      Echo Framework                           │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │  │
│  │  │  Middleware │  │  Handlers   │  │   WebSocket Hub    │  │  │
│  │  │ - IP Filter │  │ - Instance  │  │   - Terminal      │  │  │
│  │  │ - Logging   │  │ - Network   │  │   - Sessions      │  │  │
│  │  │ - Recovery  │  │ - Health    │  │                    │  │  │
│  │  └──────┬──────┘  └──────┬──────┘  └─────────┬──────────┘  │  │
│  └─────────│─────────────────│──────────────────│──────────────┘  │
└────────────│─────────────────│──────────────────│──────────────────┘
             │                 │                  │
             ▼                 ▼                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      Internal Layers                                │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────┐  │
│  │    Config       │  │    Models       │  │  Multipass Client   │  │
│  │  - YAML loader  │  │  - Instance     │  │  - CLI Executor    │  │
│  │  - Env override │  │  - Network      │  │  - Output Parser   │  │
│  │                 │  │  - Image        │  │  - Stream Handler  │  │
│  └─────────────────┘  └─────────────────┘  └─────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    System Layer (multipass CLI)                      │
│                    exec/pipe/shell → multipass                       │
└─────────────────────────────────────────────────────────────────────┘
```

### 1.2 Component Responsibilities

| Component | Responsibility |
|-----------|----------------|
| **Svelte UI** | User interface, state management |
| **Echo Server** | HTTP routing, middleware chain |
| **IP Whitelist** | Network access control |
| **Handlers** | Request processing, response formatting |
| **Multipass Client** | CLI execution, output parsing |
| **WebSocket Hub** | Terminal session management |
| **Config** | Application configuration |

---

## 2. API Design

### 2.1 REST Endpoints

#### Base URL
```
http://localhost:8080/api
```

#### Instance Endpoints

| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| `GET` | `/instances` | List all instances | Yes |
| `POST` | `/instances` | Create new instance | Yes |
| `GET` | `/instances/:name` | Get instance details | Yes |
| `DELETE` | `/instances/:name` | Delete instance | Yes |
| `POST` | `/instances/:name/start` | Start instance | Yes |
| `POST` | `/instances/:name/stop` | Stop instance | Yes |
| `POST` | `/instances/:name/restart` | Restart instance | Yes |
| `POST` | `/instances/:name/suspend` | Suspend instance | Yes |
| `POST` | `/instances/:name/resume` | Resume instance | Yes |

#### Network Endpoints

| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| `GET` | `/networks` | List available networks | Yes |
| `POST` | `/networks` | Create network bridge | Yes |
| `DELETE` | `/networks/:name` | Delete network | Yes |

#### Other Endpoints

| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| `GET` | `/images` | List available images | Yes |
| `GET` | `/health` | Health check | No |
| `WS` | `/instances/:name/shell` | Terminal websocket | Yes |

### 2.2 Request/Response Formats

#### GET /instances

**Response:**
```json
{
  "instances": [
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
  ]
}
```

#### POST /instances

**Request:**
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

**Response (201 Created):**
```json
{
  "name": "my-vm",
  "state": "Running",
  "ipv4": ["192.168.64.3"],
  "cpu": 2,
  "memory": "2G",
  "disk": "10G",
  "image": "22.04",
  "created_at": "2024-01-15T11:00:00Z"
}
```

#### GET /instances/:name

**Response:**
```json
{
  "name": "primary",
  "state": "Running",
  "ipv4": ["192.168.64.2"],
  "ipv6": ["fdec:5598:6ba4:a940::1"],
  "cpu": 2,
  "memory": "2G",
  "disk": "10G",
  "image": "22.04",
  "release": "Ubuntu 22.04.3 LTS",
  "created_at": "2024-01-15T10:30:00Z",
  "mounts": [
    {
      "source": "/home/user/projects",
      "target": "/home/ubuntu/projects"
    }
  ],
  "disks": [
    {
      "name": "main",
      "total": "10G",
      "used": "2G"
    }
  ],
  "load": [0.5, 0.3, 0.2],
  "memory": {
    "total": "2G",
    "used": "500M"
  },
  "network": {
    "enp0s1": {
      "ipv4": "192.168.64.2",
      "ipv6": "fdec:5598:6ba4:a940::1"
    }
  }
}
```

#### POST /instances/:name/start, /stop, /restart

**Response (200 OK):**
```json
{
  "message": "Instance started"
}
```

#### DELETE /instances/:name

**Response (200 OK):**
```json
{
  "message": "Instance deleted"
}
```

### 2.3 Error Responses

**400 Bad Request:**
```json
{
  "error": "invalid_request",
  "message": "Instance name is required"
}
```

**404 Not Found:**
```json
{
  "error": "not_found",
  "message": "Instance 'vm-name' not found"
}
```

**500 Internal Server Error:**
```json
{
  "error": "internal_error",
  "message": "Failed to execute multipass command"
}
```

**503 Service Unavailable:**
```json
{
  "error": "service_unavailable",
  "message": "Multipass daemon is not running"
}
```

---

## 3. WebSocket Protocol

### 3.1 Connection Lifecycle

```
Client                                    Server
  │                                         │
  │──── WebSocket Connect (/:name/shell) ──▶│
  │                                         │
  │◀─── Connect ACK (with instance info) ───│
  │                                         │
  │──── { type: "start", cols, rows } ──────▶│
  │                                         │
  │◀─── { type: "output", data: "..." } ─────│
  │     { type: "output", data: "..." } ────▶│
  │                                         │
  │──── { type: "input", data: "ls\n" } ────▶│
  │◀─── { type: "output", data: "..." } ─────│
  │                                         │
  │──── { type: "resize", cols, rows } ──────▶│
  │                                         │
  │──── { type: "stop" } ───────────────────▶│
  │◀─── { type: "exit", code: 0 } ───────────│
  │                                         │
  │────────── Close Connection ─────────────▶│
```

### 3.2 Message Formats

#### Client Messages

```typescript
// Start terminal session
{
  "type": "start",
  "cols": 80,
  "rows": 24
}

// Send input to shell
{
  "type": "input",
  "data": "ls -la\n"
}

// Resize terminal
{
  "type": "resize",
  "cols": 120,
  "rows": 30
}

// Stop and disconnect
{
  "type": "stop"
}
```

#### Server Messages

```typescript
// Terminal output
{
  "type": "output",
  "data": "ubuntu@primary:~$ "
}

// Error output
{
  "type": "error",
  "data": "command not found"
}

// Session started confirmation
{
  "type": "started",
  "instance": "primary"
}

// Session ended
{
  "type": "exit",
  "code": 0
}

// Error occurred
{
  "type": "error",
  "message": "Instance not running"
}
```

### 3.3 Connection Management

| Feature | Implementation |
|---------|----------------|
| **Heartbeat** | Ping/pong every 30 seconds |
| **Idle Timeout** | Configurable (default: 30 min) |
| **Max Connections** | 1 per instance (prevent conflicts) |
| **Reconnection** | Client handles with exponential backoff |
| **Cleanup** | Goroutine cleanup on disconnect |

---

## 4. Component Design

### 4.1 Go Backend Packages

```
api/internal/
├── config/
│   └── config.go           # Configuration loading
│
├── models/
│   ├── instance.go        # Instance data structures
│   ├── network.go         # Network data structures
│   ├── image.go           # Image data structures
│   └── error.go           # Error types
│
├── multipass/
│   ├── client.go          # Client interface
│   ├── executor.go        # CLI command execution
│   ├── parser.go          # Output parsing
│   └── types.go           # Multipass-specific types
│
├── handlers/
│   ├── instance.go        # Instance CRUD handlers
│   ├── network.go         # Network handlers
│   ├── image.go           # Image handlers
│   ├── health.go          # Health check
│   └── terminal.go        # WebSocket terminal
│
├── middleware/
│   ├── ip_whitelist.go    # IP filter middleware
│   ├── logging.go         # Request logging
│   ├── recovery.go        # Panic recovery
│   └── cors.go            # CORS (if needed)
│
└── websocket/
    ├── hub.go             # Connection hub
    ├── client.go          # Client wrapper
    └── terminal.go        # Terminal handler
```

### 4.2 Svelte Frontend Structure

```
ui/src/
├── lib/
│   ├── components/
│   │   ├── Button.svelte          # Button variants
│   │   ├── Card.svelte            # Card container
│   │   ├── Badge.svelte           # Status badges
│   │   ├── Modal.svelte           # Modal dialog
│   │   ├── Table.svelte           # Data table
│   │   ├── Input.svelte           # Form input
│   │   ├── Select.svelte          # Dropdown select
│   │   ├── Spinner.svelte         # Loading spinner
│   │   ├── Terminal.svelte        # xterm.js wrapper
│   │   ├── InstanceCard.svelte    # Instance display card
│   │   └── InstanceForm.svelte    # Create instance form
│   │
│   ├── services/
│   │   ├── api.ts                 # API client
│   │   └── websocket.ts           # WebSocket client
│   │
│   ├── stores/
│   │   ├── instances.ts           # Instance state
│   │   ├── networks.ts            # Network state
│   │   └── notifications.ts       # User notifications
│   │
│   └── types/
│       ├── instance.ts            # Instance types
│       ├── network.ts              # Network types
│       └── api.ts                  # API response types
│
└── routes/
    ├── +layout.svelte             # App layout
    ├── +page.svelte               # Dashboard
    ├── instances/
    │   ├── +page.svelte           # Instance list
    │   ├── [name]/+page.svelte    # Instance details
    │   └── new/+page.svelte       # Create instance
    ├── networks/
    │   └── +page.svelte           # Network management
    └── settings/
        └── +page.svelte           # Settings page
```

### 4.3 Design Patterns

#### Repository Pattern (Go)

```go
type MultipassClient interface {
    ListInstances() ([]Instance, error)
    GetInstance(name string) (*Instance, error)
    CreateInstance(opts CreateOptions) (*Instance, error)
    StartInstance(name string) error
    StopInstance(name string) error
    DeleteInstance(name string) error
    ListImages() ([]Image, error)
    ListNetworks() ([]Network, error)
}

type multipassClient struct {
    executor Executor
    parser   Parser
}

func NewMultipassClient(exec Executor, parser Parser) MultipassClient {
    return &multipassClient{
        executor: exec,
        parser:   parser,
    }
}
```

#### Service Pattern (Svelte)

```typescript
// services/api.ts
class ApiService {
  private baseUrl: string;

  async getInstances(): Promise<Instance[]> {
    const response = await fetch(`${this.baseUrl}/instances`);
    return response.json();
  }

  async createInstance(data: CreateInstanceRequest): Promise<Instance> {
    const response = await fetch(`${this.baseUrl}/instances`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    return response.json();
  }
}

export const api = new ApiService();
```

#### Store Pattern (Svelte)

```typescript
// stores/instances.ts
import { writable } from 'svelte/store';
import type { Instance } from '$lib/types';

function createInstancesStore() {
  const { subscribe, set, update } = writable<Instance[]>([]);

  return {
    subscribe,
    set,
    async refresh() {
      const instances = await api.getInstances();
      set(instances);
    },
    add(instance: Instance) {
      update(instances => [...instances, instance]);
    },
    remove(name: string) {
      update(instances => instances.filter(i => i.name !== name));
    },
  };
}

export const instances = createInstancesStore();
```

---

## 5. Data Flow

### 5.1 Instance Creation Flow

```
User (Browser)                    API Server                    Multipass
    │                                 │                            │
    │── POST /instances ──────────────▶│                            │
    │   { name: "vm", cpu: 2, ... }   │                            │
    │                                 │                            │
    │                                 │── exec("multipass launch")─▶│
    │                                 │                            │
    │                                 │◀─ stdout: "Launched: vm" ──│
    │                                 │                            │
    │                                 │── exec("multipass info vm")─▶│
    │                                 │                            │
    │                                 │◀─ JSON output ─────────────│
    │                                 │                            │
    │◀── 201 Created ────────────────┤                            │
    │   { name: "vm", state: ... }   │                            │
    │                                 │                            │
```

### 5.2 State Synchronization

```
┌─────────────────────────────────────────────────────────────┐
│                     Instance State Flow                       │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  multipass list ──▶ Parse Output ──▶ In-Memory Store ──▶ API│
│                           │                    │              │
│                           │                    │              │
│                      Cache (TTL)              │              │
│                           │                    │              │
│                           ▼                    ▼              │
│                      Background              Polling          │
│                      Refresh                 (Svelte)         │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 5.3 Terminal Data Flow

```
┌─────────────────────────────────────────────────────────────┐
│                    Terminal Data Flow                         │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Browser ──▶ WebSocket ──▶ Goroutine ──▶ pty ──▶ multipass │
│     │                                    │              │
│     │◀── WebSocket ──▶ Data ────────────┤              │
│     │                                    │              │
│     │◀── WebSocket ──▶ Output ───────────┘              │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## 6. Security

### 6.1 IP Whitelist Middleware

```go
func IPWhitelist(allowedCIDRs []string) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            clientIP := c.RealIP()
            
            // Check against allowed CIDRs
            allowed := false
            for _, cidr := range allowedCIDRs {
                if _, ipnet, _ := net.ParseCIDR(cidr); ipnet != nil {
                    if ipnet.Contains(net.ParseIP(clientIP)) {
                        allowed = true
                        break
                    }
                }
            }
            
            if !allowed {
                return echo.NewHTTPError(http.StatusForbidden, 
                    "Access denied")
            }
            
            return next(c)
        }
    }
}
```

### 6.2 Input Validation

| Field | Validation |
|-------|------------|
| Instance Name | `^[a-z][a-z0-9-]*[a-z0-9]$`, max 63 chars |
| CPU Count | Integer, min 1, max hardware limit |
| Memory | Pattern: `^\d+[KMG]?$`, min 128M |
| Disk | Pattern: `^\d+[KMG]?$`, min 512M |
| Network Name | Alphanumeric, max 255 chars |

### 6.3 CLI Command Sanitization

```go
func sanitizeInstanceName(name string) error {
    // Only allow safe characters
    if !regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$`).MatchString(name) {
        return errors.New("invalid instance name")
    }
    if len(name) > 63 {
        return errors.New("instance name too long")
    }
    return nil
}

func (e *Executor) Execute(name string, args ...string) ([]byte, error) {
    // args must come from controlled sources, not user input
    // For user input, use command builders with validation
    return exec.Command("multipass", args...).Output()
}
```

---

## 7. Error Handling

### 7.1 Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `invalid_request` | 400 | Invalid request parameters |
| `unauthorized` | 401 | Authentication failed |
| `forbidden` | 403 | IP not whitelisted |
| `not_found` | 404 | Resource not found |
| `conflict` | 409 | Resource already exists |
| `instance_running` | 409 | Cannot perform action on running instance |
| `instance_stopped` | 409 | Instance not running |
| `multipass_error` | 500 | Multipass CLI error |
| `internal_error` | 500 | Internal server error |
| `service_unavailable` | 503 | Multipass daemon not running |

### 7.2 Logging Strategy

```go
// Request logging
log.Info().
    Str("method", c.Request().Method).
    Str("path", c.Request().URL.Path).
    Str("ip", c.RealIP()).
    Int("status", rec.Status).
    Dur("latency", time.Since(start)).
    Msg("request")

// Error logging
log.Error().
    Err(err).
    Str("instance", name).
    Msg("failed to start instance")
```

---

## 8. Configuration

### 8.1 Config Structure

```go
type Config struct {
    Server struct {
        Host string `yaml:"host"`
        Port int    `yaml:"port"`
    } `yaml:"server"`
    
    Security struct {
        AllowedIPs      []string `yaml:"allowed_ips"`
        WebsocketTimeout int    `yaml:"websocket_idle_timeout_minutes"`
    } `yaml:"security"`
    
    Multipass struct {
        SocketPath        string `yaml:"socket_path"`
        DefaultTimeout    int    `yaml:"default_timeout_seconds"`
    } `yaml:"multipass"`
    
    Logging struct {
        Level  string `yaml:"level"`
        Format string `yaml:"format"`
    } `yaml:"logging"`
}
```

### 8.2 Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `CLOUDPASS_CONFIG` | Config file path | `./config.yaml` |
| `CLOUDPASS_HOST` | Server host | `0.0.0.0` |
| `CLOUDPASS_PORT` | Server port | `8080` |
| `CLOUDPASS_LOG_LEVEL` | Log level | `info` |

---

## 9. Testing Strategy

### 9.1 Testing Framework

| Layer | Framework | Purpose |
|-------|-----------|---------|
| Go Unit | `testing` + `testify` | Unit tests |
| Go Integration | `testify` | CLI integration |
| Svelte Unit | `vitest` | Component tests |
| Svelte Component | `@testing-library/svelte` | Component testing |
| E2E | `Playwright` | Full flow testing |

### 9.2 Unit Test Patterns

```go
// Test like users would use it
func TestCreateInstance(t *testing.T) {
    // Setup
    client := NewTestClient()
    
    // Execute
    instance, err := client.CreateInstance(CreateOptions{
        Name: "test-vm",
        CPUs: 2,
    })
    
    // Assert
    assert.NoError(t, err)
    assert.Equal(t, "test-vm", instance.Name)
    assert.Equal(t, "Running", instance.State)
}

// Test edge cases
func TestCreateInstance_InvalidName(t *testing.T) {
    client := NewTestClient()
    
    _, err := client.CreateInstance(CreateOptions{
        Name: "123-invalid",
    })
    
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "invalid instance name")
}

func TestCreateInstance_DuplicateName(t *testing.T) {
    client := NewTestClient()
    
    // Create first instance
    _, _ = client.CreateInstance(CreateOptions{Name: "test"})
    
    // Try to create duplicate
    _, err := client.CreateInstance(CreateOptions{Name: "test"})
    
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "already exists")
}
```

### 9.3 Test Coverage Targets

| Component | Target |
|-----------|--------|
| Handlers | 70% |
| Multipass Client | 80% |
| Models | 90% |
| Svelte Components | 60% |

---

## 10. Deployment

### 10.1 Docker

```dockerfile
# api/Dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/server

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /server /server
COPY config.yaml /etc/cloudpass/config.yaml
EXPOSE 8080
ENTRYPOINT ["/server"]
```

```dockerfile
# ui/Dockerfile
FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/build /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
```

### 10.2 Kubernetes

```yaml
# kubernetes/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cloudpass
spec:
  replicas: 1
  selector:
    matchLabels:
      app: cloudpass
  template:
    spec:
      containers:
      - name: api
        image: cloudpass/api:latest
        ports:
        - containerPort: 8080
        volumeMounts:
        - name: config
          mountPath: /etc/cloudpass
        - name: multipass-socket
          mountPath: /var/run/multipass
      - name: ui
        image: cloudpass/ui:latest
        ports:
        - containerPort: 80
      volumes:
      - name: config
        configMap:
          name: cloudpass-config
      - name: multipass-socket
        hostPath:
          path: /var/run/multipass
```

---

## Appendix: API Quick Reference

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/health` | GET | Health check |
| `/api/instances` | GET | List instances |
| `/api/instances` | POST | Create instance |
| `/api/instances/:name` | GET | Get instance |
| `/api/instances/:name` | DELETE | Delete instance |
| `/api/instances/:name/start` | POST | Start instance |
| `/api/instances/:name/stop` | POST | Stop instance |
| `/api/instances/:name/restart` | POST | Restart instance |
| `/api/instances/:name/suspend` | POST | Suspend instance |
| `/api/instances/:name/shell` | WS | Terminal |
| `/api/images` | GET | List images |
| `/api/networks` | GET | List networks |
| `/api/networks` | POST | Create network |
| `/api/networks/:name` | DELETE | Delete network |

---

## Related Documentation

- [PROJECT_PLAN.md](PROJECT_PLAN.md) - Project roadmap
- [AGENTS.md](AGENTS.md) - Contribution guidelines
- [docs/MULTIPASS_API_REFERENCE.md](docs/MULTIPASS_API_REFERENCE.md) - Multipass CLI reference
