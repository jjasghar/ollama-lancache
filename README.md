# Ollama LanCache

A model distribution system for efficiently sharing Ollama models across a local network. Reduces bandwidth usage by allowing clients to download models from a local server instead of the internet.

## 🎯 Overview

Ollama LanCache provides two approaches for local model distribution:

1. **Model Distribution Server** (Recommended): A simple HTTP server that serves cached models with client scripts for easy installation
2. **Registry Proxy** (Advanced): A transparent proxy that intercepts Ollama registry requests

## 🚀 Quick Start - Model Distribution Server

The model distribution server is the recommended approach. It's simple, reliable, and works with any Ollama version.

### 1. Prerequisites

- Go 1.21 or higher
- Existing Ollama installation with cached models in `~/.ollama/models`

### 2. Build and Run

```bash
# Clone the repository
git clone https://github.com/jjasghar/ollama-lancache.git
cd ollama-lancache

# Build the application
go build -o ollama-lancache .

# Start the model distribution server
./ollama-lancache serve --port 8080
```

The server will automatically:
- Discover available models in your `~/.ollama/models` directory
- Display server IP addresses for client configuration
- Serve a web interface at `http://your-ip:8080`

### 3. Install Models on Client Machines

The server provides platform-specific client scripts for easy model installation.

#### Windows (PowerShell)

```powershell
# List available models
powershell -c "irm http://192.168.1.100:8080/install.ps1 | iex"

# Install a specific model
powershell -c "`$env:OLLAMA_MODEL='granite3.3:8b'; irm http://192.168.1.100:8080/install.ps1 | iex"
```

#### Linux/macOS (Bash)

```bash
# List available models
curl -fsSL http://192.168.1.100:8080/install.sh | bash

# Install a specific model
curl -fsSL http://192.168.1.100:8080/install.sh | bash -s -- --model granite3.3:8b --server 192.168.1.100:8080
```

Replace `192.168.1.100` with your actual server IP address.

### 4. Verify Installation

After installation, verify the model is available:

```bash
ollama list
ollama run granite3.3:8b
```

## 📋 Features

### Model Distribution Server
- **Web Interface**: Browse available models through a web UI
- **API Endpoints**: JSON API for programmatic access
- **Cross-Platform**: PowerShell and Bash client scripts
- **Automatic Discovery**: Finds models in your Ollama cache directory
- **Progress Tracking**: Shows download progress and model information

### Registry Proxy (Advanced)
- **DNS Interception**: Redirects `registry.ollama.ai` requests to local cache
- **HTTP Caching**: Caches model blobs and manifests locally
- **Transparent Operation**: Works with existing Ollama installations
- **Cache Statistics**: Built-in cache management and statistics

## 🔧 Installation Methods

### Method 1: Binary Release

Download the latest release from [GitHub Releases](https://github.com/jjasghar/ollama-lancache/releases):

```bash
# Linux/macOS
curl -L https://github.com/jjasghar/ollama-lancache/releases/latest/download/ollama-lancache-linux-amd64 -o ollama-lancache
chmod +x ollama-lancache

# Windows (PowerShell)
Invoke-WebRequest -Uri "https://github.com/jjasghar/ollama-lancache/releases/latest/download/ollama-lancache-windows-amd64.exe" -OutFile "ollama-lancache.exe"
```

### Method 2: Build from Source

```bash
git clone https://github.com/jjasghar/ollama-lancache.git
cd ollama-lancache
make build
```

### Method 3: Go Install

```bash
go install github.com/jjasghar/ollama-lancache@latest
```

## 📖 Usage

### Model Distribution Server

```bash
# Start the server (default port 8080)
./ollama-lancache serve

# Custom port and models directory
./ollama-lancache serve --port 9000 --models-dir /path/to/models

# Get help
./ollama-lancache serve --help
```

#### Server Endpoints

- `GET /` - Web interface listing available models
- `GET /api/models` - JSON list of available models
- `GET /api/info` - Server information
- `GET /install.ps1` - PowerShell client script
- `GET /install.sh` - Bash client script
- `GET /models/{model}:{tag}` - Download complete model
- `GET /manifests/{model}:{tag}` - Download model manifest
- `GET /blobs/{digest}` - Download individual blob

### Registry Proxy Server

```bash
# Start the registry proxy (requires root for DNS)
sudo ./ollama-lancache server

# HTTP-only mode (recommended for testing)
./ollama-lancache server --dns-enabled=false --http-port 80

# Custom configuration
./ollama-lancache server --cache-dir /var/cache/ollama --http-port 8080 --dns-port 53
```

#### Configure Clients for Proxy Mode

Add to client machine's hosts file:
```bash
# Linux/macOS
echo "192.168.1.100 registry.ollama.ai" | sudo tee -a /etc/hosts

# Windows (as Administrator)
echo 192.168.1.100 registry.ollama.ai >> C:\Windows\System32\drivers\etc\hosts
```

### Cache Management

```bash
# View cache statistics
./ollama-lancache cache stats

# Clear cache
./ollama-lancache cache clear
```

## 🌐 API Reference

### GET /api/models

Returns a JSON array of available models:

```json
[
  {
    "name": "granite3.3",
    "tag": "8b",
    "size": 4942891236,
    "modified": "2025-08-08T10:48:44.820079316-05:00",
    "download_url": "/models/granite3.3:8b",
    "manifest_url": "/manifests/granite3.3:8b"
  }
]
```

### GET /api/info

Returns server information:

```json
{
  "server": "ollama-lancache",
  "version": "1.0.0",
  "models_directory": "/Users/user/.ollama/models",
  "model_count": 3,
  "server_ips": ["192.168.1.100", "10.0.0.5"]
}
```

## 🏗️ Architecture

### Model Distribution Server Architecture

```
┌─────────────────┐    HTTP     ┌──────────────────┐
│  Client Script  │ ──────────> │ Distribution     │
│ (install.ps1/.sh)│             │ Server (:8080)   │
└─────────────────┘             └──────────────────┘
                                          │
                                          │ Reads
                                          ▼
                                ┌──────────────────┐
                                │ ~/.ollama/models │
                                │  ├── manifests/  │
                                │  └── blobs/      │
                                └──────────────────┘
```

### Registry Proxy Architecture

```
┌─────────────────┐             ┌──────────────────┐             ┌─────────────────┐
│ Ollama Client   │             │ Ollama LanCache  │             │ registry.       │
│                 │             │                  │             │ ollama.ai       │
│ ollama pull     │ ──────────> │ DNS + HTTP Proxy │ ──────────> │                 │
│ granite3.3:8b   │             │                  │             │ (upstream)      │
└─────────────────┘             └──────────────────┘             └─────────────────┘
                                          │
                                          │ Caches
                                          ▼
                                ┌──────────────────┐
                                │ Local Cache      │
                                │  ├── manifests/  │
                                │  └── blobs/      │
                                └──────────────────┘
```

## 📁 Project Structure

```
ollama-lancache/
├── cmd/                    # CLI commands
│   ├── root.go            # Root command configuration
│   ├── serve.go           # Model distribution server
│   ├── server.go          # Registry proxy server
│   └── cache.go           # Cache management commands
├── internal/              # Internal packages
│   ├── cache/             # Cache management logic
│   ├── dns/               # DNS server implementation
│   └── proxy/             # HTTP proxy implementation
├── scripts/               # Client installation scripts
│   ├── install.ps1        # PowerShell client script
│   └── install.sh         # Bash client script
├── test/                  # Integration tests
├── Makefile              # Build and development tasks
├── go.mod                # Go module dependencies
└── README.md             # This file
```

## 🔧 Configuration

### Environment Variables

- `OLLAMA_MODELS` - Override default models directory
- `OLLAMA_LANCACHE_PORT` - Default server port
- `OLLAMA_LANCACHE_HOST` - Server bind address

### Configuration File

Create `~/.ollama-lancache.yaml`:

```yaml
# Model distribution server settings
serve:
  port: 8080
  host: "0.0.0.0"
  models-dir: "~/.ollama/models"

# Registry proxy server settings  
server:
  cache-dir: "~/.ollama/models"
  listen-addr: "0.0.0.0"
  http-port: 80
  dns-port: 53
  dns-enabled: true
  upstream-dns: "8.8.8.8:53"
```

## 🚀 Deployment

### Docker

```bash
# Build image
docker build -t ollama-lancache .

# Run model distribution server
docker run -p 8080:8080 -v ~/.ollama/models:/models ollama-lancache serve --models-dir /models

# Run registry proxy server
docker run -p 53:53/udp -p 80:80 -v ~/.ollama/models:/cache ollama-lancache server --cache-dir /cache
```

### Systemd Service

Create `/etc/systemd/system/ollama-lancache.service`:

```ini
[Unit]
Description=Ollama LanCache Model Distribution Server
After=network.target

[Service]
Type=simple
User=ollama
ExecStart=/usr/local/bin/ollama-lancache serve --port 8080
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl enable ollama-lancache
sudo systemctl start ollama-lancache
```

## 🔍 Troubleshooting

### Common Issues

#### Models not appearing
- Verify models exist in `~/.ollama/models/manifests/`
- Check file permissions on models directory
- Ensure manifest files are not corrupted

#### Client download failures
- Verify server is accessible: `curl http://server-ip:8080/api/models`
- Check firewall settings on server
- Ensure client has sufficient disk space

#### Permission errors
- Run server with appropriate user permissions
- For proxy mode with DNS, run as root or use capabilities:
  ```bash
  sudo setcap 'cap_net_bind_service=+ep' ./ollama-lancache
  ```

### Debug Mode

```bash
# Enable verbose logging
./ollama-lancache serve --verbose

# Enable debug logging  
./ollama-lancache serve --log-level debug
```

### Network Testing

```bash
# Test server connectivity
curl -v http://192.168.1.100:8080/api/info

# Test model download
curl -I http://192.168.1.100:8080/models/granite3.3:8b

# Test client script download
curl http://192.168.1.100:8080/install.sh
```

## 🤝 Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### Development Setup

```bash
# Clone and setup
git clone https://github.com/jjasghar/ollama-lancache.git
cd ollama-lancache

# Install dependencies
go mod download

# Run tests
make test

# Build
make build

# Run with development settings
go run . serve --port 8080
```

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [Ollama](https://ollama.ai/) for the excellent model platform
- [Cobra](https://github.com/spf13/cobra) for CLI framework
- [Viper](https://github.com/spf13/viper) for configuration management
- [miekg/dns](https://github.com/miekg/dns) for DNS server functionality

## 📞 Support

- 🐛 **Bug Reports**: [GitHub Issues](https://github.com/jjasghar/ollama-lancache/issues)
- 💡 **Feature Requests**: [GitHub Discussions](https://github.com/jjasghar/ollama-lancache/discussions)
- 📖 **Documentation**: [Wiki](https://github.com/jjasghar/ollama-lancache/wiki)

---

**Made with ❤️ for the Ollama community**