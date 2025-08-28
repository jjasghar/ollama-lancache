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
│   ├── cache.go          # Cache management commands
│   ├── root.go           # Root command and configuration
│   ├── serve.go          # Model distribution server
│   └── server.go         # Registry proxy server
├── examples/             # Usage examples and deployment scenarios
│   ├── client-install-examples.md  # Client installation examples
│   ├── docker-compose-simple.yml   # Simple Docker deployment
│   └── systemd-service.sh          # Systemd service installer
├── internal/            # Private application packages
│   ├── cache/           # Cache management logic
│   │   ├── cache.go     # Cache implementation
│   │   └── cache_test.go # Cache tests
│   ├── dns/             # DNS server implementation
│   │   ├── server.go    # DNS server logic
│   │   └── server_test.go # DNS tests
│   └── proxy/           # HTTP proxy implementation
│       ├── server.go    # Proxy server logic
│       └── server_test.go # Proxy tests
├── scripts/             # Client installation scripts
│   ├── install.ps1      # PowerShell client (Windows)
│   └── install.sh       # Bash client (Linux/macOS)
├── test/                # Integration and end-to-end tests
│   ├── cache_miss_test.go       # Cache miss testing
│   ├── integration_test.go      # Integration tests
│   └── live_cache_miss_demo.go  # Live demo tests
├── CHANGELOG.md         # Version history and changes
├── CONTRIBUTING.md      # Contribution guidelines
├── Dockerfile          # Multi-stage Docker build
├── docker-compose.yml  # Docker Compose orchestration
├── go.mod              # Go module definition
├── go.sum              # Go module checksums
├── LICENSE             # MIT license
├── main.go             # Application entry point
├── Makefile            # Build automation and tasks
└── README.md           # Main project documentation
```

## Core Components

### 1. ollama-lancache Server (`cmd/serve.go`)

**Purpose**: Simple HTTP server for distributing cached Ollama models

**Features**:
- Web interface for browsing models
- API endpoints for programmatic access
- Cross-platform client scripts
- Automatic model discovery
- Progress tracking

**Key Files**:
- `cmd/serve.go` - Server implementation
- `scripts/install.ps1` - Windows PowerShell client
- `scripts/install.sh` - Linux/macOS Bash client

### 2. Registry Proxy Server (`cmd/server.go`)

**Purpose**: Transparent proxy that intercepts Ollama registry requests

**Features**:
- DNS interception for registry.ollama.ai
- Docker Registry v2 API compatibility
- HTTP caching with range request support
- Blob and manifest caching

**Key Files**:
- `cmd/server.go` - Proxy server implementation
- `internal/proxy/server.go` - Core proxy logic
- `internal/dns/server.go` - DNS server implementation
- `internal/cache/cache.go` - Cache management

### 3. Client Scripts (`scripts/`)

**Purpose**: Platform-specific installation scripts for easy model deployment

**PowerShell Script (`install.ps1`)**:
- Windows-compatible installation
- Automatic server detection
- Progress indicators and validation
- Error handling and recovery

**Bash Script (`install.sh`)**:
- Linux/macOS compatible installation
- JSON parsing with jq fallback
- Flexible parameter handling
- Comprehensive error reporting

## Development Workflow

### Building

```bash
# Build for current platform
make build

# Build for all platforms
make build-all

# Build with custom version
VERSION=1.0.0 make build
```

### Testing

```bash
# Run unit tests
make test

# Run tests with coverage
make test-coverage

# Run integration tests
make test-integration

# Run benchmarks
make bench
```

### Quality Assurance

```bash
# Format code
make fmt

# Run linter
make lint

# Security scan
make security

# All quality checks
make quality
```

### Development Server

```bash
# Start model distribution server
make run

# Start registry proxy server
make run-server

# Development mode with auto-reload
make dev
```

## Deployment Options

### 1. Single Binary

```bash
# Download and install
curl -L https://github.com/jjasghar/ollama-lancache/releases/latest/download/ollama-lancache-linux-amd64 -o ollama-lancache
chmod +x ollama-lancache
./ollama-lancache serve --port 8080
```

### 2. Docker

```bash
# Simple deployment
docker run -p 8080:8080 -v ~/.ollama/models:/models ghcr.io/jjasghar/ollama-lancache:latest

# Using Docker Compose
docker-compose up -d
```

### 3. Systemd Service

```bash
# Install as system service
sudo ./examples/systemd-service.sh
```

## Configuration

### Environment Variables

- `OLLAMA_MODELS` - Override default models directory
- `OLLAMA_LANCACHE_PORT` - Default server port
- `OLLAMA_LANCACHE_HOST` - Server bind address

### Configuration File

Location: `~/.ollama-lancache.yaml`

```yaml
# Model distribution server
serve:
  port: 8080
  host: "0.0.0.0"
  models-dir: "~/.ollama/models"

# Registry proxy server
server:
  cache-dir: "~/.ollama/models"
  listen-addr: "0.0.0.0"
  http-port: 80
  dns-port: 53
  dns-enabled: true
```

## API Endpoints

### ollama-lancache Server

- `GET /` - Web interface
- `GET /api/models` - List available models (JSON)
- `GET /api/info` - Server information (JSON)
- `GET /install.ps1` - PowerShell client script
- `GET /install.sh` - Bash client script
- `GET /models/{model}:{tag}` - Download model
- `GET /manifests/{model}:{tag}` - Download manifest
- `GET /blobs/{digest}` - Download blob

### Registry Proxy Server

- `GET /health` - Health check
- `GET /v2/` - Docker Registry v2 API root
- `GET /v2/{name}/manifests/{reference}` - Model manifests
- `GET /v2/{name}/blobs/{digest}` - Model blobs
- `GET /cache/stats` - Cache statistics

## Testing Strategy

### Unit Tests
- Individual component testing
- Mock external dependencies
- High coverage targets (>80%)

### Integration Tests
- End-to-end workflow testing
- Client-server interaction validation
- Error scenario testing

### Performance Tests
- Load testing with multiple clients
- Large model transfer benchmarks
- Memory and CPU profiling

## Security Considerations

### Network Security
- Non-privileged port defaults
- Input validation and sanitization
- Rate limiting (future enhancement)

### Container Security
- Non-root user execution
- Minimal base images
- Security scanning in CI

### Access Control
- File system permissions
- Network access controls
- Audit logging (future enhancement)

## Contributing

1. **Setup**: Follow development setup in `CONTRIBUTING.md`
2. **Branching**: Use feature branches from `main`
3. **Testing**: Ensure all tests pass and coverage is maintained
4. **Documentation**: Update relevant documentation
5. **Review**: Submit pull request for review

## Release Process

1. **Version Bump**: Update version in appropriate files
2. **Changelog**: Update `CHANGELOG.md` with changes
3. **Tag**: Create git tag with version (e.g., `v1.0.0`)
4. **CI/CD**: GitHub Actions builds and publishes releases
5. **Docker**: Container images published to GitHub Container Registry

## Support and Maintenance

- **Issues**: GitHub Issues for bug reports
- **Discussions**: GitHub Discussions for questions
- **Security**: Private disclosure for security issues
- **Documentation**: GitHub Wiki for additional docs
