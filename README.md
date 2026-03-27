<p align="center">
  <img src="docs/logo.png" alt="CloudPass Logo" width="200">
</p>

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
- **Docker Support**: Easy deployment with Docker and Docker Compose
- **IP Whitelist**: Security middleware for LAN access control

---

## Quick Start

### Prerequisites

- [Multipass](https://multipass.run/) installed on the server
- Docker and Docker Compose (for containerized deployment)
- Go 1.24+ (for development)
- Node.js 20+ (for frontend development)

### Using Docker Compose

```bash
# Clone the repository
git clone https://github.com/laolusrael/cloudpass.git
cd cloudpass

# Start the services
docker-compose up -d

# Access the UI
# API: http://localhost:8080
# UI: http://localhost:3000
```

## Deployment

### Option 1: Docker Compose (Recommended)

Follow the Docker Compose instructions in the Quick Start section above.

### Option 2: Direct Server Deployment (Without Docker)

For deploying on a server without Docker:

1. **Build the API**
```bash
cd api
go build -o cloudpass ./cmd/server
```

2. **Build the Frontend**
```bash
cd ui
npm install
npm run build
```

3. **Set up Nginx as Reverse Proxy**
- Install nginx
- Configure nginx to serve static files from ui/build and proxy API requests to the Go server
- Example nginx config provided

4. **Create Systemd Service**
- Create a systemd service file for the API server
- Enable and start the service

5. **Security Configuration**
- Set allowed IPs in config.yaml for IP whitelist
- Use firewall rules

6. **Access the Application**
- Access via nginx (e.g., http://your-server)

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

---

## Development

See [AGENTS.md](AGENTS.md) for contribution guidelines.

### Project Structure

```
cloudpass/
├── api/                    # Go backend
│   ├── cmd/server/        # Entry point
│   ├── internal/
│   │   ├── handlers/      # HTTP handlers
│   │   ├── middleware/    # Middleware
│   │   ├── multipass/     # CLI wrapper
│   │   ├── models/        # Data models
│   │   └── config/        # Configuration
│   └── Dockerfile
├── ui/                     # Svelte frontend
│   ├── src/
│   │   ├── lib/
│   │   │   ├── components/  # UI components
│   │   │   ├── services/    # API client
│   │   │   ├── stores/      # Svelte stores
│   │   │   └── types/       # TypeScript types
│   │   └── routes/        # Pages
│   └── Dockerfile
├── docker-compose.yml
└── README.md
```

### Running Tests

```bash
# Backend tests
cd api
go test ./...

# Frontend tests
cd ui
npm run test

# Lint
cd api && golangci-lint run ./...
cd ui && npm run lint
```

---

## Tech Stack

| Layer | Technology |
|-------|------------|
| Backend | Go, Echo |
| Frontend | Svelte, TypeScript, Vite |
| Styling | Tailwind CSS |
| Terminal | xterm.js |
| Container | Docker, Docker Compose |

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
