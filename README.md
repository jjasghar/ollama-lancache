# Ollama LanCache

A comprehensive model distribution system for efficiently sharing Ollama models across a local network. Features real-time session monitoring, multi-client support, file downloads server, and cross-platform client scripts.

## 🎯 Overview

Ollama LanCache is a simple yet powerful HTTP server that allows you to share Ollama models across your local network, reducing bandwidth usage by allowing clients to download models from a local server instead of the internet.

**Key Features:**
- **🚀 High-performance HTTP server** with real-time session tracking
- **📱 Cross-platform client scripts** (Windows PowerShell, Linux/macOS Bash)
- **📊 Real-time monitoring** with web interface and REST API
- **📁 File downloads server** for sharing additional resources
- **🌐 Multi-client support** with concurrent download tracking
- **🔒 Security features** with path traversal protection

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
- ✅ Creates `downloads/` directory with helpful README
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
- **📝 Copy-paste commands** for all platforms with real server URLs
- **📊 Real-time session monitoring** at `/api/sessions`
- **📁 File downloads browser** at `/downloads/`
- **🎨 Clean, responsive design** with proper UTF-8 emoji support

### 📊 Session Tracking & Monitoring

Real-time tracking of client downloads with detailed progress information:

```bash
# Check active sessions
curl http://your-server:8080/api/sessions

# Example response:
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

**Server logs provide detailed tracking:**
```bash
🚀 [192.168.1.50] Started downloading model: granite3.3:8b (estimated 5 files)
📄 [192.168.1.50] Manifest served: granite3.3:8b (expecting 5 files)
🗃️  [192.168.1.50] Blob served: sha256:77bce... (4713.89 MB) - granite3.3:8b
✅ [192.168.1.50] Completed downloading model: granite3.3:8b
   📊 Duration: 2m15s | Files: 5/5 | Data: 4.98 GB | Avg Speed: 37.8 MB/s
```

### 📁 File Downloads Server

Share additional files alongside models with automatic setup:

**Auto-created on first run:**
- Creates `downloads/` directory automatically
- Generates helpful `README.txt` with usage instructions
- Web interface for browsing and downloading files

**Perfect for sharing:**
- **📦 Executable files** (.exe, .msi, .deb, .rpm, .dmg)
- **🗜️ Archive files** (.zip, .tar.gz, .7z, .rar)
- **📄 Documentation** (.pdf, .txt, .md, .docx)
- **⚙️ Configuration files** (.json, .yaml, .conf, .ini)
- **📝 Scripts** (.ps1, .sh, .bat, .py)

**Usage:**
```bash
# Add files to the downloads directory
cp my-app.exe downloads/
cp documentation.pdf downloads/

# Files are available at:
# http://your-server:8080/downloads/           (browse all files)
# http://your-server:8080/downloads/my-app.exe (direct download)
```

## 📋 API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/` | GET | Web interface with usage instructions and model catalog |
| `/api/models` | GET | List available models (JSON) |
| `/api/info` | GET | Server information and statistics |
| `/api/sessions` | GET | Active download sessions with real-time progress |
| `/install.ps1` | GET | PowerShell client script (Windows) |
| `/install.sh` | GET | Bash client script (Linux/macOS) |
| `/downloads/` | GET | File downloads server and browser |
| `/downloads/{file}` | GET | Direct file download |
| `/manifests/{model}` | GET | Model manifest files |
| `/blobs/{digest}` | GET | Model blob files |
| `/health` | GET | Health check endpoint |

## 🛠️ Installation Options

### Using Make

```bash
make build          # Build binary for current platform
make build-all      # Cross-compile for Linux, macOS, Windows
make test           # Run tests
make run            # Build and run server
make install        # Install to system PATH
make clean          # Clean build artifacts
```

### Manual Build

```bash
go build -o ollama-lancache .
```

### Docker

```bash
# Using Docker Compose (recommended)
docker-compose up -d

# Manual Docker build and run
docker build -t ollama-lancache .
docker run -p 8080:8080 -v ~/.ollama/models:/models:ro -v ./downloads:/app/downloads ollama-lancache
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
```

## 🌐 Multi-Client Support

The system supports unlimited concurrent clients with individual tracking:

- **🔄 Independent sessions** per client IP and model
- **📊 Real-time progress** for each download
- **⚡ Parallel downloads** without interference
- **🧹 Automatic cleanup** of stale sessions (30-minute timeout)

## 🔍 Monitoring & Troubleshooting

### Health Check
```bash
curl http://your-server:8080/health
# Returns: OK
```

### Check Available Models
```bash
curl http://your-server:8080/api/models | jq .
```

### Monitor Active Downloads
```bash
curl http://your-server:8080/api/sessions | jq .
```

### Common Issues

**Models not appearing:**
```bash
# Verify models directory exists and has content
ls ~/.ollama/models/manifests/
ls ~/.ollama/models/blobs/
```

**Client connection issues:**
```bash
# Test server connectivity
curl http://your-server:8080/health

# Check firewall (if needed)
sudo ufw allow 8080
```

**Client downloads not working:**
```bash
# Verify client has Ollama installed
ollama --version

# Check target directory permissions
ls -la ~/.ollama/models/
```

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

### Custom Configuration

```bash
# Use custom models directory
./ollama-lancache serve --models-dir /path/to/models --port 8080

# Bind to specific interface
./ollama-lancache serve --bind 192.168.1.100 --port 8080
```

### Production Deployment

```bash
# Build optimized binary
make build

# Run with production settings
./ollama-lancache serve \
  --port 8080 \
  --bind 0.0.0.0 \
  --models-dir /opt/ollama/models
```

## 🔄 CI/CD & Automation

The project includes comprehensive GitHub Actions workflows:

- **✅ Continuous Integration** - Automated testing and linting
- **🏗️ Cross-platform builds** - Linux, macOS, Windows binaries
- **🐳 Docker automation** - Image building and publishing
- **🔒 Security scanning** - CodeQL and vulnerability checks
- **📦 Automated releases** - GitHub Releases with cross-platform assets

## 🎯 How It Works

1. **Server Setup**: Run `ollama-lancache serve` on a machine with Ollama models
2. **Client Discovery**: Clients visit the web interface for copy-paste commands
3. **Model Download**: Client scripts download models directly to local Ollama installation
4. **Real-time Tracking**: Server monitors all downloads with progress and timing
5. **File Sharing**: Additional files available via `/downloads/` endpoint

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

- **Formatting**: `make fmt`
- **Linting**: `make lint` 
- **Testing**: `make test`
- **Security**: Built-in path traversal protection

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Built for the [Ollama](https://ollama.ai) ecosystem
- Inspired by the need for efficient local AI model distribution
- Thanks to all contributors and users

## 📞 Support

- **Issues**: [GitHub Issues](https://github.com/jjasghar/ollama-lancache/issues)
- **Discussions**: [GitHub Discussions](https://github.com/jjasghar/ollama-lancache/discussions)
- **Documentation**: Check the auto-generated `downloads/README.txt` for file sharing instructions

---

**Made with ❤️ for the AI community**