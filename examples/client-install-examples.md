# Client Installation Examples

This document provides various examples of how to install models using the Ollama LanCache distribution system.

## Basic Usage

### Windows PowerShell

```powershell
# List available models
powershell -c "irm http://192.168.1.100:8080/install.ps1 | iex"

# Install a specific model
powershell -c "`$env:OLLAMA_MODEL='granite3.3:8b'; irm http://192.168.1.100:8080/install.ps1 | iex"

# Install with custom server
powershell -c "`$env:OLLAMA_SERVER='http://192.168.1.100:8080'; `$env:OLLAMA_MODEL='llama3.2:1b'; irm http://192.168.1.100:8080/install.ps1 | iex"
```

### Linux/macOS Bash

```bash
# List available models
curl -fsSL http://192.168.1.100:8080/install.sh | bash

# Install a specific model
curl -fsSL http://192.168.1.100:8080/install.sh | bash -s -- --model granite3.3:8b --server http://192.168.1.100:8080

# Install with verbose output
curl -fsSL http://192.168.1.100:8080/install.sh | bash -s -- --model llama3.2:1b --server http://192.168.1.100:8080 --verbose
```

## Advanced Examples

### Batch Installation (Windows)

```powershell
# Create a PowerShell script to install multiple models
@"
`$models = @('granite3.3:8b', 'llama3.2:1b', 'granite-code:8b')
`$server = 'http://192.168.1.100:8080'

foreach (`$model in `$models) {
    Write-Host "Installing `$model..." -ForegroundColor Cyan
    `$env:OLLAMA_MODEL = `$model
    irm `$server/install.ps1 | iex
    
    if (`$LASTEXITCODE -eq 0) {
        Write-Host "✅ `$model installed successfully" -ForegroundColor Green
    } else {
        Write-Host "❌ Failed to install `$model" -ForegroundColor Red
    }
}
"@ | Out-File -FilePath install-models.ps1

# Run the batch installation
powershell -ExecutionPolicy Bypass -File install-models.ps1
```

### Batch Installation (Linux/macOS)

```bash
#!/bin/bash
# install-models.sh

models=("granite3.3:8b" "llama3.2:1b" "granite-code:8b")
server="http://192.168.1.100:8080"

for model in "${models[@]}"; do
    echo "Installing $model..."
    
    if curl -fsSL "$server/install.sh" | bash -s -- --model "$model" --server "$server"; then
        echo "✅ $model installed successfully"
    else
        echo "❌ Failed to install $model"
    fi
done
```

### Corporate Network (Behind Proxy)

```powershell
# Windows with corporate proxy
$env:http_proxy = "http://proxy.company.com:8080"
$env:https_proxy = "http://proxy.company.com:8080"
powershell -c "`$env:OLLAMA_MODEL='granite3.3:8b'; irm http://internal-ollama-cache:8080/install.ps1 | iex"
```

```bash
# Linux/macOS with corporate proxy
export http_proxy=http://proxy.company.com:8080
export https_proxy=http://proxy.company.com:8080
curl -fsSL http://internal-ollama-cache:8080/install.sh | bash -s -- --model granite3.3:8b
```

### Air-Gapped Network

For completely offline environments:

1. **Download scripts on connected machine:**
```bash
# Download the client scripts
curl -o install.ps1 http://192.168.1.100:8080/install.ps1
curl -o install.sh http://192.168.1.100:8080/install.sh

# Download models manually (get URLs from /api/models)
curl -o granite3.3-8b.tar http://192.168.1.100:8080/models/granite3.3:8b
```

2. **Transfer to air-gapped machine and install:**
```powershell
# Windows - Manual installation
.\install.ps1 -LocalFile granite3.3-8b.tar -Model granite3.3:8b
```

### Docker Container Installation

```bash
# Install models inside a Docker container
docker run --rm -v ~/.ollama:/root/.ollama alpine/curl:latest sh -c "
  apk add --no-cache bash &&
  curl -fsSL http://host.docker.internal:8080/install.sh | bash -s -- --model granite3.3:8b --server http://host.docker.internal:8080
"
```

### Automated CI/CD Pipeline

```yaml
# GitHub Actions example
name: Install Ollama Models
on: [push]

jobs:
  setup-models:
    runs-on: ubuntu-latest
    steps:
      - name: Install Ollama
        run: curl -fsSL https://ollama.ai/install.sh | sh
        
      - name: Install models from LanCache
        run: |
          curl -fsSL http://${{ secrets.OLLAMA_CACHE_SERVER }}:8080/install.sh | bash -s -- --model granite3.3:8b
          curl -fsSL http://${{ secrets.OLLAMA_CACHE_SERVER }}:8080/install.sh | bash -s -- --model llama3.2:1b
          
      - name: Verify installation
        run: ollama list
```

## Troubleshooting Examples

### Check Server Connectivity

```bash
# Test server is reachable
curl -f http://192.168.1.100:8080/api/info

# List available models
curl -s http://192.168.1.100:8080/api/models | jq .

# Check model exists
curl -I http://192.168.1.100:8080/models/granite3.3:8b
```

### Verify Installation

```bash
# Check if model was installed correctly
ollama list

# Test model works
ollama run granite3.3:8b "Hello, world!"

# Check model files exist
ls -la ~/.ollama/models/manifests/registry.ollama.ai/library/granite3.3/8b
ls -la ~/.ollama/models/blobs/
```

### Debug Connection Issues

```powershell
# Windows - Test network connectivity
Test-NetConnection -ComputerName 192.168.1.100 -Port 8080

# Check DNS resolution
nslookup your-cache-server

# Test with verbose curl
curl -v http://192.168.1.100:8080/api/info
```

```bash
# Linux/macOS - Test network connectivity
nc -zv 192.168.1.100 8080

# Check firewall
sudo iptables -L | grep 8080

# Test with verbose curl
curl -v http://192.168.1.100:8080/api/info
```

## Script Customization

### Custom Installation Directory

```bash
# Install to custom directory
export OLLAMA_MODELS=/custom/path/to/models
curl -fsSL http://192.168.1.100:8080/install.sh | bash -s -- --model granite3.3:8b
```

### Progress Monitoring

```powershell
# Windows - Monitor installation progress
$job = Start-Job -ScriptBlock {
    $env:OLLAMA_MODEL='granite3.3:8b'
    irm http://192.168.1.100:8080/install.ps1 | iex
}

while ($job.State -eq "Running") {
    Write-Host "Installation in progress..." -ForegroundColor Yellow
    Start-Sleep 5
}

Receive-Job $job
```

### Retry Logic

```bash
#!/bin/bash
# install-with-retry.sh

model="granite3.3:8b"
server="http://192.168.1.100:8080"
max_retries=3
retry_count=0

while [ $retry_count -lt $max_retries ]; do
    echo "Attempt $((retry_count + 1)) of $max_retries"
    
    if curl -fsSL "$server/install.sh" | bash -s -- --model "$model" --server "$server"; then
        echo "✅ Successfully installed $model"
        exit 0
    else
        echo "❌ Failed to install $model, retrying in 30 seconds..."
        sleep 30
        retry_count=$((retry_count + 1))
    fi
done

echo "❌ Failed to install $model after $max_retries attempts"
exit 1
```
