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

### 1. Download

Download the latest release from [GitHub Releases](https://github.com/laolusrael/cloudpass/releases)

### 2. Run

Extract the archive and run:

```bash
# Linux
tar -xzf cloudpass-VERSION-linux-amd64.tar.gz
cd cloudpass-VERSION-linux-amd64
./cloudpass
```

```powershell
# Windows
Expand-Archive cloudpass-VERSION-windows-amd64.zip -DestinationPath cloudpass
cd cloudpass
.\cloudpass.exe
```

Access the UI at: **http://localhost:8080**

---

## Installation

### Linux (systemd)

Install CloudPass as a systemd service for automatic startup:

```bash
# Extract the archive
tar -xzf cloudpass-VERSION-linux-amd64.tar.gz
cd cloudpass-VERSION-linux-amd64

# Install with default port (8080)
sudo ./scripts/install.sh

# Or specify a custom port
sudo ./scripts/install.sh --port 9000
```

**Uninstall:**

```bash
sudo ./scripts/uninstall.sh
```

### Windows (Service)

Install CloudPass as a Windows service for automatic startup:

```powershell
# Extract the archive
Expand-Archive cloudpass-VERSION-windows-amd64.zip -DestinationPath cloudpass
cd cloudpass

# Install with default port (8080)
.\scripts\install.ps1

# Or specify a custom port
.\scripts\install.ps1 -Port 9000
```

**Uninstall:**

```powershell
.\scripts\uninstall.ps1
```

### Advanced

#### Nginx Reverse Proxy

```nginx
server {
    listen 80;
    server_name your-server;

    # API proxy (WebSocket support)
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

---

## SSH Key Setup

For terminal access to instances, CloudPass needs the Multipass SSH private key.

> **Note:** The install scripts (`install.sh` and `install.ps1`) automatically copy the SSH key during installation. You only need manual setup if running CloudPass directly without the install scripts.

### Linux

```bash
sudo cp /var/snap/multipass/common/data/multipassd/ssh-keys/id_rsa \
       ~/.cloudpass/multipass_id_rsa
sudo chmod 600 ~/.cloudpass/multipass_id_rsa
```

### Windows

```powershell
# Create .cloudpass directory in your profile
New-Item -ItemType Directory -Force -Path "$env:USERPROFILE\.cloudpass"

# Copy the SSH key
Copy-Item "C:\Windows\System32\config\systemprofile\AppData\Roaming\multipassd\ssh-keys\id_rsa" `
    "$env:USERPROFILE\.cloudpass\id_rsa"
```

---

## Upgrade

### Linux

```bash
sudo systemctl stop cloudpass
# Replace the binary in /opt/cloudpass/
sudo systemctl start cloudpass
```

### Windows

```powershell
Stop-Service CloudPass
# Replace cloudpass.exe in C:\Program Files\CloudPass\
Start-Service CloudPass
```

---

## Configuration

### config.yaml

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
| `CLOUDPASS_HOST` | Server host | `0.0.0.0` |
| `CLOUDPASS_PORT` | Server port | `8080` |
| `CLOUDPASS_LOG_LEVEL` | Log level | `info` |

---

## License

Distributed under the MIT License. See [LICENSE](LICENSE) for more information.

---

## Acknowledgments

- [Multipass](https://multipass.run/) - Ubuntu VMs for everyone
- [Echo](https://echo.labstack.com/) - Fast and unfancy web framework for Go
- [Svelte](https://svelte.dev/) - Cybernetically enhanced web apps
