# Project Structure

This document describes the organization and structure of the Ollama LanCache project.

## Directory Structure

```
ollama-lancache/
├── .github/
│   └── workflows/          # GitHub Actions CI/CD workflows
│       ├── ci.yml         # Continuous integration pipeline
│       └── release.yml    # Release automation
├── cmd/                   # CLI command implementations
│   ├── root.go           # Root command and configuration
│   └── serve.go          # HTTP model distribution server
├── examples/             # Usage examples and deployment scenarios
│   ├── client-install-examples.md  # Client installation examples
│   ├── docker-compose-simple.yml   # Simple Docker deployment
│   └── systemd-service.sh          # Systemd service installer
├── scripts/             # Client installation scripts
│   ├── install.ps1      # PowerShell client (Windows)
│   └── install.sh       # Bash client (Linux/macOS)
├── downloads/           # File downloads directory (created at runtime)
│   └── README.txt       # Usage instructions for downloads
├── CHANGELOG.md         # Version history and changes
├── CONTRIBUTING.md      # Contribution guidelines
├── Dockerfile          # Multi-stage Docker build
├── docker-compose.yml  # Docker Compose orchestration
├── go.mod              # Go module definition
├── go.sum              # Go module checksums
├── LICENSE             # MIT license
├── main.go             # Application entry point
├── Makefile            # Build automation and tasks
├── PROJECT_STRUCTURE.md # This file
└── README.md           # Main project documentation
```

## Core Components

### 1. HTTP Server (`cmd/serve.go`)

The main application component that provides:
- **Model serving**: Serves Ollama models from `~/.ollama/models`
- **Web interface**: Clean HTML interface with model catalog and usage instructions
- **Session tracking**: Real-time monitoring of client downloads with progress tracking
- **File downloads**: Additional file server at `/downloads/` endpoint
- **REST API**: JSON endpoints for programmatic access
- **Client scripts**: Dynamic generation of platform-specific installation scripts

### 2. Client Scripts (`scripts/`)

Cross-platform installation scripts that handle model downloads:

#### PowerShell Script (`install.ps1`)
- **Platform**: Windows
- **Features**: Environment variable support, smart defaults, auto-server detection
- **Usage**: `powershell -c "irm http://server:8080/install.ps1 | iex"`

#### Bash Script (`install.sh`)
- **Platform**: Linux/macOS
- **Features**: Command-line arguments, JSON parsing with jq fallback
- **Usage**: `curl -fsSL http://server:8080/install.sh | bash -s -- --server server:8080 --model model:tag`

### 3. Web Interface

Responsive HTML interface providing:
- **Model catalog** with sizes and modification dates
- **Copy-paste commands** for all platforms with real server URLs
- **Session monitoring** via `/api/sessions` endpoint
- **File downloads browser** at `/downloads/`
- **API documentation** with clickable endpoints

### 4. REST API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/` | GET | Main web interface |
| `/api/models` | GET | List available models (JSON) |
| `/api/info` | GET | Server information and statistics |
| `/api/sessions` | GET | Active download sessions with progress |
| `/install.ps1` | GET | PowerShell client script |
| `/install.sh` | GET | Bash client script |
| `/downloads/` | GET | File downloads browser |
| `/downloads/{file}` | GET | Direct file download |
| `/manifests/{model}` | GET | Model manifest files |
| `/blobs/{digest}` | GET | Model blob files |
| `/health` | GET | Health check endpoint |

## Key Features

### Session Tracking
- **Real-time monitoring** of client downloads
- **Progress tracking** with bytes served and completion percentage
- **Multi-client support** with individual session management
- **Automatic cleanup** of stale sessions (30-minute timeout)

### Security
- **Path traversal protection** prevents `../` attacks in downloads
- **Input validation** on all endpoints
- **Safe file serving** with proper content types and headers

### Cross-Platform Support
- **Windows**: PowerShell script with environment variable support
- **Linux/macOS**: Bash script with command-line arguments
- **Docker**: Multi-stage builds for containerized deployment

### File Downloads Server
- **Additional file sharing** alongside models
- **Web browser interface** for easy file access
- **Direct download URLs** for programmatic access
- **Automatic directory creation** with proper permissions

## Build System

### Makefile Targets
- `make build` - Build binary for current platform
- `make build-all` - Cross-compile for all platforms
- `make test` - Run tests with coverage
- `make run` - Build and run server
- `make docker` - Build Docker image
- `make clean` - Clean build artifacts

### Docker Support
- **Multi-stage build** for optimized image size
- **Non-root execution** for security
- **Volume mounts** for models and downloads
- **Docker Compose** for easy deployment

## Development

### Project Layout
The project follows Go standard layout:
- `cmd/` - CLI commands and application entry points
- `scripts/` - Client-side installation scripts
- `examples/` - Usage examples and deployment scenarios
- `.github/workflows/` - CI/CD automation

### Dependencies
- **Cobra** - CLI framework for command structure
- **Viper** - Configuration management
- **Standard library** - HTTP server, JSON handling, file operations

### Testing
- Unit tests for core functionality
- Integration tests for client scripts
- Docker-based testing for cross-platform compatibility

## Deployment Options

### 1. Binary Deployment
```bash
./ollama-lancache serve --port 8080
```

### 2. Docker Deployment
```bash
docker-compose up -d
```

### 3. Systemd Service
```bash
sudo ./examples/systemd-service.sh
```

## Configuration

### Command Line Flags
- `--port` - HTTP server port (default: 8080)
- `--bind` - IP address to bind to (default: 0.0.0.0)
- `--models-dir` - Models directory (default: ~/.ollama/models)

### Environment Variables
- `OLLAMA_MODELS` - Custom models directory
- `OLLAMA_LANCACHE_PORT` - Server port
- `OLLAMA_LANCACHE_BIND` - Bind address

## Architecture

The ollama-lancache follows a simple client-server architecture:

1. **Server**: HTTP server that serves models and provides web interface
2. **Clients**: Platform-specific scripts that download and install models
3. **Storage**: File system storage for models and additional downloads
4. **Monitoring**: Real-time session tracking and REST API

This architecture provides:
- **Simplicity**: No complex proxy or DNS configuration required
- **Reliability**: Direct HTTP file serving with standard protocols
- **Scalability**: Supports unlimited concurrent clients
- **Maintainability**: Clear separation of concerns and minimal dependencies