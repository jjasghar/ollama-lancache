# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- File downloads server at `/downloads/` endpoint for sharing additional files
- Real-time session tracking with progress monitoring
- Copy-paste ready commands in web interface
- UTF-8 emoji support in web interface
- Smart defaults in client scripts
- Session timeout handling (30 minutes)
- Multi-client concurrent download support

### Removed
- Legacy DNS proxy server components (no longer needed)
- Registry proxy approach (simplified to HTTP-only)
- Unused internal packages and test files
- Complex DNS configuration requirements

### Changed
- Simplified architecture to HTTP-only model distribution
- Updated README with current features and capabilities
- Improved error handling and user feedback in client scripts
- Enhanced web interface with better styling and copy-paste commands
- Increased session timeout from 10 to 30 minutes for large downloads

### Fixed
- Windows file path compatibility issues with blob storage
- JSON parsing errors in client scripts
- PowerShell variable reference issues with model names containing colons

## [1.0.0] - 2025-01-XX

### Added
- Initial release of Ollama LanCache
- DNS server for intercepting registry.ollama.ai requests
- HTTP proxy for caching model blobs and manifests
- Cache management commands
- Basic client configuration support
- Docker registry v2 API compatibility

### Features
- **ollama-lancache Server**: Simple HTTP server for serving cached models
  - Web interface for browsing available models
  - API endpoints for listing models and server information
  - Cross-platform client scripts (PowerShell and Bash)
  - Automatic model discovery from Ollama cache directory
  - Progress tracking and download validation

- **Registry Proxy** (Advanced): Transparent proxy for Ollama registry requests
  - DNS interception for registry.ollama.ai
  - HTTP caching with full Docker Registry v2 API support
  - Blob and manifest caching
  - Cache statistics and management

- **Cross-Platform Support**:
  - Linux (AMD64, ARM64)
  - macOS (Intel, Apple Silicon)
  - Windows (AMD64)

- **Deployment Options**:
  - Single binary deployment
  - Docker containers
  - Systemd service integration
  - Docker Compose orchestration

### Technical Details
- Built with Go 1.21+
- Uses Cobra for CLI framework
- Viper for configuration management
- Docker Registry v2 API compatibility
- HTTP Range request support
- Chunked transfer encoding
- SHA256 blob verification

### Security
- Non-root Docker containers
- Minimal attack surface
- Input validation and sanitization
- Secure defaults
