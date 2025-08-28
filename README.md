# Ollama LanCache

A comprehensive model distribution system for efficiently sharing Ollama models across a local network. Features real-time session monitoring, multi-client support, file downloads server, and cross-platform client scripts.

## 🎯 Overview

Ollama LanCache is a complete solution for local AI model distribution that reduces bandwidth usage by allowing clients to download models from a local server instead of the internet. The system provides:

- **🚀 High-performance HTTP server** with session tracking and monitoring
- **📱 Cross-platform client scripts** (Windows PowerShell, Linux/macOS Bash)
- **📊 Real-time monitoring** with web interface and REST API
- **📁 File downloads server** for additional resources
- **🔒 Security features** with path traversal protection
- **🌐 Multi-client support** with concurrent download tracking

## ⚡ Quick Start

### 1. Prerequisites

- Go 1.21 or higher
- Existing Ollama installation with cached models in `~/.ollama/models`

### 2. Build and Run

```bash
# Clone the repository
git clone https://github.com/jjasghar/ollama-lancache.git
cd ollama-lancache

# Build the application
make build
# or: go build -o ollama-lancache .

# Start the server
./ollama-lancache serve --port 8080
```

The server automatically:
- ✅ Discovers available models in `~/.ollama/models`
- ✅ Creates `downloads/` directory for additional files
- ✅ Displays server IP addresses and usage instructions
- ✅ Serves web interface at `http://your-ip:8080`

### 3. Install Models on Clients

Visit the web interface at `http://your-server:8080` for copy-paste ready commands, or use:

#### Windows (PowerShell)

```powershell
# Install granite3.3:8b model (example)
$env:OLLAMA_MODEL='granite3.3:8b'; powershell -c "irm http://192.168.1.100:8080/install.ps1 | iex"

# List available models first
powershell -c "irm http://192.168.1.100:8080/install.ps1 | iex"
```

#### Linux/macOS (Bash)

```bash
# Install granite3.3:8b model (example)
curl -fsSL http://192.168.1.100:8080/install.sh | bash -s -- --server 192.168.1.100:8080 --model granite3.3:8b

# List available models
curl -fsSL http://192.168.1.100:8080/install.sh | bash -s -- --server 192.168.1.100:8080 --list
```

## 🌟 Features

### 🖥️ Web Interface

- **📋 Model catalog** with sizes and modification dates
- **📝 Copy-paste commands** for all platforms
- **📊 Real-time session monitoring** at `/api/sessions`
- **📁 File downloads browser** at `/downloads/`
- **🎨 Clean, responsive design** with proper UTF-8 emoji support

### 📊 Session Tracking & Monitoring

- **Real-time progress tracking** with bytes served and completion percentage
- **Multi-client support** with individual session management
- **Download timing** with start/stop logging and speed calculations
- **Session cleanup** with configurable timeout (30 minutes default)
- **REST API endpoints** for programmatic monitoring

### 📁 File Downloads Server

Share additional files alongside models:

```bash
# Add files to the downloads directory
cp my-app.exe downloads/
cp documentation.pdf downloads/

# Access via web browser or direct download
# http://your-server:8080/downloads/
# http://your-server:8080/downloads/my-app.exe
```

Perfect for:
- **Executable files** (.exe, .msi, .deb, .rpm)
- **Documentation** (.pdf, .txt, .md)
- **Archive files** (.zip, .tar.gz, .7z)
- **Configuration files** and scripts

### 🔒 Security Features

- **Path traversal protection** prevents `../` attacks
- **Directory restrictions** - only serves files, not subdirectories
- **File validation** with existence and type checking
- **Safe defaults** with comprehensive input validation

## 📋 API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/` | GET | Web interface with usage instructions |
| `/api/models` | GET | List available models (JSON) |
| `/api/info` | GET | Server information and statistics |
| `/api/sessions` | GET | Active download sessions with progress |
| `/install.ps1` | GET | PowerShell client script |
| `/install.sh` | GET | Bash client script |
| `/downloads/` | GET | File downloads server and browser |
| `/downloads/{file}` | GET | Direct file download |
| `/health` | GET | Health check endpoint |

## 🛠️ Installation Options

### Using Make

```bash
make build          # Build binary
make build-all      # Cross-compile for all platforms
make test           # Run tests
make run            # Build and run
make install        # Install to system
```

### Manual Build

```bash
go build -o ollama-lancache .
```

### Docker

```bash
# Using Docker Compose
docker-compose up -d

# Manual Docker build
docker build -t ollama-lancache .
docker run -p 8080:8080 -v ~/.ollama/models:/models ollama-lancache
```

## 🔧 Configuration

### Command Line Options

```bash
./ollama-lancache serve [flags]

Flags:
  -p, --port int           Port to serve on (default 8080)
  -b, --bind string        IP address to bind to (default "0.0.0.0")
  -d, --models-dir string  Models directory (default "~/.ollama/models")
  -h, --help              Help for serve
      --version           Show version information
```

### Environment Variables

```bash
export OLLAMA_MODELS="/custom/path/to/models"
export OLLAMA_LANCACHE_PORT=8080
export OLLAMA_LANCACHE_BIND="0.0.0.0"
```

## 📊 Monitoring & Logging

### Real-time Session Monitoring

```bash
# Check active sessions
curl http://localhost:8080/api/sessions | jq .

# Example response
{
  "active_sessions": [
    {
      "client_ip": "192.168.1.50",
      "model": "granite3.3:8b",
      "start_time": "2025-01-15T10:30:00Z",
      "duration": "2m15s",
      "bytes_served": 2147483648,
      "files_served": 3,
      "total_files": 5,
      "progress_percent": 60
    }
  ],
  "total_sessions": 1
}
```

### Server Logs

```bash
🚀 [192.168.1.50] Started downloading model: granite3.3:8b (estimated 5 files)
📄 [192.168.1.50] Manifest served: granite3.3:8b (expecting 5 files)
🗃️  [192.168.1.50] Blob served: sha256:77bce... (4713.89 MB) - granite3.3:8b
✅ [192.168.1.50] Completed downloading model: granite3.3:8b
   📊 Duration: 2m15s | Files: 5/5 | Data: 4.98 GB | Avg Speed: 37.8 MB/s
```

## 🌐 Multi-Client Support

The system supports unlimited concurrent clients:

- **Independent session tracking** per client IP and model
- **Parallel downloads** with individual progress monitoring
- **Bandwidth sharing** across multiple clients
- **Session isolation** prevents interference between clients

## 🐳 Docker Support

### Docker Compose (Recommended)

```yaml
version: '3.8'
services:
  ollama-lancache:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - ~/.ollama/models:/models:ro
      - ./downloads:/app/downloads
    environment:
      - OLLAMA_MODELS=/models
    restart: unless-stopped
```

### Standalone Docker

```bash
docker run -d \
  --name ollama-lancache \
  -p 8080:8080 \
  -v ~/.ollama/models:/models:ro \
  -v ./downloads:/app/downloads \
  -e OLLAMA_MODELS=/models \
  ollama-lancache:latest
```

## 🔄 CI/CD & Automation

The project includes comprehensive GitHub Actions workflows:

- **Continuous Integration** with automated testing and linting
- **Cross-platform builds** for Linux, macOS, and Windows
- **Docker image building** and publishing
- **Security scanning** with CodeQL and vulnerability checks
- **Automated releases** with GitHub Releases and assets

## 📚 Advanced Usage

### Systemd Service

```bash
# Install as systemd service
sudo ./examples/systemd-service.sh

# Control the service
sudo systemctl start ollama-lancache
sudo systemctl enable ollama-lancache
sudo systemctl status ollama-lancache
```

### Custom Model Directory

```bash
# Use custom models directory
./ollama-lancache serve --models-dir /path/to/models --port 8080
```

### Production Deployment

```bash
# Build optimized binary
make build-linux

# Run with production settings
./ollama-lancache serve \
  --port 8080 \
  --bind 0.0.0.0 \
  --models-dir /opt/ollama/models
```

## 🔍 Troubleshooting

### Common Issues

**Models not appearing:**
```bash
# Verify models directory
ls ~/.ollama/models/manifests/

# Check server logs for errors
./ollama-lancache serve --port 8080
```

**Client connection issues:**
```bash
# Test server connectivity
curl http://your-server:8080/health

# Check firewall settings
sudo ufw allow 8080
```

**Download failures:**
```bash
# Check client-side Ollama installation
ollama list

# Verify file permissions
chmod -R 755 ~/.ollama/models/
```

### Debug Mode

```bash
# Enable verbose logging
export OLLAMA_LANCACHE_DEBUG=true
./ollama-lancache serve --port 8080
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

# Run with live reload
make run
```

### Code Quality

- **Go formatting**: `make fmt`
- **Linting**: `make lint`
- **Security scanning**: `make security`
- **Test coverage**: `make test-coverage`

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Built for the [Ollama](https://ollama.ai) ecosystem
- Inspired by the need for efficient local AI model distribution
- Thanks to all contributors and users of the project

## 📞 Support

- **Issues**: [GitHub Issues](https://github.com/jjasghar/ollama-lancache/issues)
- **Discussions**: [GitHub Discussions](https://github.com/jjasghar/ollama-lancache/discussions)
- **Documentation**: [Project Wiki](https://github.com/jjasghar/ollama-lancache/wiki)

---

**Made with ❤️ for the AI community**