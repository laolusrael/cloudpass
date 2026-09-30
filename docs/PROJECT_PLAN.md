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
| **Styling** | Tailwind CSS | UI styling (grayscale) |
| **Terminal** | xterm.js | Web shell access |
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
│   │   │   ├── image.go
│   │   │   ├── jobs.go
│   │   │   ├── job_storage.go
│   │   │   ├── job_events.go
│   │   │   ├── config.go
│   │   │   └── health.go
│   │   ├── middleware/
│   │   │   ├── ip_whitelist.go
│   │   │   └── logging.go
│   │   ├── multipass/
│   │   │   ├── client.go
│   │   │   └── ssh.go
│   │   ├── models/
│   │   │   ├── instance.go
│   │   │   └── job.go
│   │   ├── logger/
│   │   │   └── logger.go
│   │   ├── web/
│   │   │   ├── web.go           # UI embed (production)
│   │   │   └── web_ci.go        # CI stub
│   │   └── websocket/
│   │       └── terminal.go
│   ├── internal/web/build/       # Embedded UI (generated)
│   ├── go.mod
│   ├── go.sum
│   └── config.yaml
├── ui/                           # Svelte frontend
│   ├── src/
│   │   ├── lib/
│   │   │   ├── components/       # Reusable UI components
│   │   │   ├── services/         # API client
│   │   │   ├── stores/           # Svelte stores
│   │   │   └── types/            # TypeScript types
│   │   └── routes/               # SvelteKit pages
│   │       ├── +page.svelte
│   │       ├── instances/
│   │       ├── networks/
│   │       └── settings/
│   ├── static/
│   ├── package.json
│   ├── svelte.config.js
│   ├── vite.config.ts
│   ├── tsconfig.json
│   └── .eslintrc.cjs
├── build.sh                      # Binary build script
├── release.sh                    # Release automation
├── .github/
│   └── workflows/
│       ├── ci.yml
│       └── release.yml
├── README.md
├── CONTRIBUTING.md
├── AGENTS.md
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
| `GET` | `/jobs` | List all jobs | 1 |
| `GET` | `/jobs/:id` | Get job status | 1 |
| `POST` | `/instances/:name/export` | Export VM | 3 |
| `POST` | `/instances/:name/export/async` | Export VM (background job) | 3 |
| `POST` | `/instances/import` | Import VM | 3 |
| `POST` | `/instances/import/async` | Import VM (background job) | 3 |
| `POST` | `/instances/:name/snapshots` | Create snapshot | 3 |
| `POST` | `/instances/:name/snapshots/async` | Create snapshot (background job) | 3 |
| `GET` | `/instances/:name/snapshots` | List snapshots | 3 |
| `POST` | `/instances/:name/snapshots/:id/restore` | Restore snapshot | 3 |
| `POST` | `/instances/:name/snapshots/:id/restore/async` | Restore snapshot (background job) | 3 |
| `POST` | `/instances/:name/mounts` | Mount host directory | 3 |
| `POST` | `/instances/:name/mounts/async` | Mount host directory (background job) | 3 |
| `DELETE` | `/instances/:name/mounts` | Unmount directory | 3 |
| `PUT` | `/instances/:name/resources` | Update CPUs/memory/disk | 3 |

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

### 4.4 File Upload

`POST /instances/:name/upload` accepts `multipart/form-data` with fields:

| Field | Required | Description |
|-------|----------|-------------|
| `file` | Yes | File content. Empty files rejected; size capped by `upload.max_file_size_mb` (default 100, max 1024). |
| `target_path` | No | Guest destination. Empty → `upload.default_path/<filename>`. Trailing `/` → directory, filename appended. Otherwise must be an absolute guest file path. |

Directory components are stripped from the client file name. The instance must be
`Running` (`instance_not_running` otherwise). Parent directories are created on
the guest before transfer. Uploading to an existing guest path overwrites it
without confirmation (last-write-wins).

Success: `201 {"message": "File uploaded", "path": "<guest path>"}`.
Errors: `invalid_request` (400), `file_too_large` (400), `instance_not_running`
(400), `not_found` (404), `upload_error`/`multipass_error` (500), plus standard
`csrf_*` (403) and `rate_limited` (429).

Uploads are staged in `upload.staging_dir` when set, otherwise automatically:
snap installs of multipass run the transfer in confinement with a private
`/tmp`, so the server stages under `$HOME/cloudpass-staging`; native installs
use the platform temp dir. Explicit config always wins; the effective directory
is reported read-only as `environment.staging_dir_effective` in `GET /config`.

### 4.5 Host Directory Mounts

`POST /instances/:name/mounts` accepts JSON fields:

| Field | Required | Description |
|-------|----------|-------------|
| `source_path` | Yes | Existing absolute host directory. Relative paths and `~` are rejected. Anyone with UI access can expose any host path — only mount paths you trust. |
| `target_path` | Yes | Guest destination. Created if missing; existing contents are overlaid, not deleted. |
| `mount_type` | No | `classic` (SSHFS, default, works on all backends) or `native` (Hyper-V/QEMU only). |
| `uid_map` / `gid_map` | No | Optional `host:instance` numeric ID mappings. |

The instance must be `Running`. Mounts persist across restarts; re-mounting an
existing target returns `conflict` (409). When `mount_type` is omitted the
server auto-selects from the detected driver (native on QEMU/Hyper-V, classic
otherwise); with snap-confined multipass, sources under `/tmp`/`/var/tmp` are
rejected up front since the daemon cannot see them.

Success: `201 {"message": "Directory mounted", "source": ..., "target": ...}`.
Errors: `invalid_request` (400), `instance_not_running` (400), `not_found`
(404), `conflict` (409), `multipass_error` (500), plus standard `csrf_*` (403)
and `rate_limited` (429).

### 4.6 Long operations, jobs, and timeouts

Minute-scale operations (mounts, snapshot create/restore, export, import)
run as background jobs: the `/async` variants validate synchronously and
answer `202 {"job": {...}}` (replays with the same `Idempotency-Key` answer
200 with the original job). Clients track completion via `GET /jobs/:id` or
the `GET /jobs/stream` SSE feed; results land in `job.result`, failures in
`job.error`. Sync variants remain for scripts and fast cases.

CSRF tokens are stable per session (minted when absent/expiring, never
rotated by plain reads), so polling, SSE reconnects, and concurrent tabs
cannot invalidate in-flight requests; the client additionally shares one
token refresh across concurrent 403s.

Server `ReadTimeout` is 10 minutes (100 MB uploads on slow links);
`WriteTimeout` stays 30s because every slow response is a job. The UI bounds
JSON requests at 60s and uploads at 10 minutes so stalled connections fail
visibly instead of spinning forever.

### 4.7 WebSocket Protocol

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
  socket_path: "/var/run/multipass/socket"
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
