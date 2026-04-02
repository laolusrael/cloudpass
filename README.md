<h1 align="center">CloudPass</h1>

<p align="center">
  A web-based management UI for Canonical's Multipass virtualization tool.
  <br>
  <a href="https://github.com/laolusrael/cloudpass/issues">Report Bug</a>
  ·
  <a href="https://github.com/laolusrael/cloudpass/pulls">Request Feature</a>
</p>

---

## Features

- **Instance Management**: Create, start, stop, restart, and delete Ubuntu VMs
- **Real-time Status**: Auto-refreshing dashboard with live instance status
- **Web UI**: Modern Svelte-based interface with grayscale design
- **REST API**: Programmatic access via RESTful endpoints
- **SSH Terminal**: Web-based terminal access to instances
- **IP Whitelist**: Security middleware for LAN access control
- **Jobs API**: Async operations for long-running tasks

---

## Quick Start

### Prerequisites

- [Multipass](https://multipass.run/) installed on the server
- Go 1.25+ (for development)
- Node.js 20+ (for frontend development)

### Quick Start

#### Option 1: Pre-built Binary

```bash
# Clone the repository
git clone https://github.com/laolusrael/cloudpass.git
cd cloudpass

# Download the latest release for your platform
# Visit: https://github.com/laolusrael/cloudpass/releases

# Or build from source
./build.sh

# Start the server
./cloudpass

# Access the UI
# http://localhost:8080
```

#### Option 2: Build from Source

```bash
# Clone the repository
git clone https://github.com/laolusrael/cloudpass.git
cd cloudpass

# Build the API
cd api
go build -o cloudpass ./cmd/server
cd ..

# Build the Frontend
cd ui
npm install
npm run build
cd ..

# Copy frontend build to API
cp -r ui/build api/internal/web/build

# Start the server
./api/cloudpass
```

### Deployment

#### Option 1: Binary Release

1. Download the latest release from [GitHub Releases](https://github.com/laolusrael/cloudpass/releases)
2. Extract the archive
3. Configure `config.yaml` as needed
4. Run `./cloudpass`

#### Option 2: Build from Source

```bash
# Build API and UI
./build.sh

# Configure
# Edit config.yaml as needed

# Run the server
./cloudpass
```

#### Option 3: With Nginx Reverse Proxy

1. **Build and run CloudPass** (as above)
2. **Install nginx**
3. **Configure nginx** to proxy API and serve static files:

```nginx
server {
    listen 80;
    server_name your-server;

    # API proxy
    location /api/ {
        proxy_pass http://localhost:8080/;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
    }

    # Static files
    location / {
        root /path/to/cloudpass/api/internal/web/build;
        try_files $uri $uri/ /index.html;
    }
}
```

4. **Create systemd service** (optional):

### Manual Setup

#### Backend

```bash
cd api

# Build the server
go build -o cloudpass ./cmd/server

# Run the server
./cloudpass
```

#### Frontend

```bash
cd ui

# Install dependencies
npm install

# Start development server
npm run dev
```

---

## Configuration

### API Configuration (config.yaml)

```yaml
server:
  host: "0.0.0.0"
  port: 8080

security:
  allowed_ips: []
  websocket_idle_timeout_minutes: 30

multipass:
  socket_path: "/var/run/multipass_socket"
  default_timeout_seconds: 300

logging:
  level: "info"
  format: "json"
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `CLOUDPASS_CONFIG` | Config file path | `./config.yaml` |
| `CLOUDPASS_HOST` | Server host | `0.0.0.0` |
| `CLOUDPASS_PORT` | Server port | `8080` |
| `CLOUDPASS_LOG_LEVEL` | Log level | `info` |

---

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/health` | Health check |
| `GET` | `/api/instances` | List all instances |
| `POST` | `/api/instances` | Create new instance |
| `GET` | `/api/instances/:name` | Get instance details |
| `DELETE` | `/api/instances/:name` | Delete instance |
| `POST` | `/api/instances/:name/start` | Start instance |
| `POST` | `/api/instances/:name/stop` | Stop instance |
| `POST` | `/api/instances/:name/restart` | Restart instance |
| `GET` | `/api/images` | List available images |
| `GET` | `/api/networks` | List available networks |
| `GET` | `/api/jobs` | List all jobs |
| `GET` | `/api/jobs/:id` | Get job status |
| `WS` | `/api/instances/:name/shell` | WebSocket terminal |

---

## Development

See [AGENTS.md](AGENTS.md) for contribution guidelines.

### Project Structure

```
cloudpass/
├── api/                    # Go backend
│   ├── cmd/server/         # Entry point
│   ├── internal/
│   │   ├── handlers/       # HTTP handlers (instances, networks, images, jobs)
│   │   ├── middleware/    # Middleware (IP whitelist, logging)
│   │   ├── multipass/     # CLI wrapper (client, SSH)
│   │   ├── models/       # Data models
│   │   ├── config/       # Configuration
│   │   ├── logger/       # Logging
│   │   ├── web/          # Embedded UI
│   │   └── websocket/    # WebSocket terminal
│   └── internal/web/     # UI build output (embedded)
├── ui/                     # Svelte frontend
│   ├── src/
│   │   ├── lib/
│   │   │   ├── components/  # UI components
│   │   │   ├── services/    # API client
│   │   │   ├── stores/      # Svelte stores
│   │   │   └── types/       # TypeScript types
│   │   └── routes/         # Pages
│   └── static/
├── build.sh                # Build script
├── release.sh              # Release script
├── config.yaml             # Configuration
└── README.md
```

### Running Tests

```bash
# Backend tests (use -tags ci to skip web embed)
cd api
go test -tags ci ./...

# Frontend tests
cd ui
npm run test

# Lint
cd api && golangci-lint run --build-tags ci ./...
cd ui && npm run lint
```
cd api && golangci-lint run ./...
cd ui && npm run lint
```

---

## Tech Stack

| Layer | Technology |
|-------|------------|
| Backend | Go, Echo |
| Frontend | Svelte, TypeScript, Vite |
| Styling | Plain CSS (grayscale) |
| Terminal | xterm.js |
| Testing | Go testing, Vitest, Playwright (E2E) |

---

## Contributing

Contributions are welcome! Please read our [contributing guidelines](AGENTS.md) first.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'feat: add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

---

## License

Distributed under the MIT License. See [LICENSE](LICENSE) for more information.

---

## Acknowledgments

- [Multipass](https://multipass.run/) - Ubuntu VMs for everyone
- [Echo](https://echo.labstack.com/) - Fast and unfancy web framework for Go
- [Svelte](https://svelte.dev/) - Cybernetically enhanced web apps
