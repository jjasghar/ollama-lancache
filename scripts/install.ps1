# Ollama Model Installer for Windows
# Downloads and installs models from the distribution server

param(
    [string]$Server = "",
    [string]$Model = "",
    [switch]$List,
    [switch]$Help
)

$ErrorActionPreference = "Stop"

function Show-Help {
    Write-Host "🚀 Ollama Model Installer for Windows" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "USAGE:" -ForegroundColor Yellow
    Write-Host "  # Auto-detect server from download URL (recommended)"
    Write-Host "  powershell -c `"irm http://SERVER:PORT/install.ps1 | iex`""
    Write-Host ""
    Write-Host "  # Download and run with parameters"
    Write-Host "  `$script = irm http://SERVER:PORT/install.ps1; Invoke-Expression `"`$script -Model MODEL:TAG`""
    Write-Host ""
    Write-Host "  # Using environment variables"
    Write-Host "  `$env:OLLAMA_MODEL='granite3.3:8b'; irm http://SERVER:PORT/install.ps1 | iex"
    Write-Host ""
    Write-Host "PARAMETERS:" -ForegroundColor Yellow
    Write-Host "  -Server   Distribution server address (e.g., 192.168.1.100:8080)"
    Write-Host "  -Model    Model to install (e.g., granite3.3:8b)"
    Write-Host "  -List     List available models"
    Write-Host "  -Help     Show this help"
    Write-Host ""
    Write-Host "ENVIRONMENT VARIABLES:" -ForegroundColor Yellow
    Write-Host "  OLLAMA_MODEL   Model to install (alternative to -Model parameter)"
    Write-Host ""
    Write-Host "EXAMPLES:" -ForegroundColor Yellow
    Write-Host "  # List available models (auto-detects server)"
    Write-Host "  powershell -c `"irm http://192.168.1.100:8080/install.ps1 | iex`""
    Write-Host ""
    Write-Host "  # Install specific model using environment variable"
    Write-Host "  powershell -c `"`$env:OLLAMA_MODEL='granite3.3:8b'; irm http://192.168.1.100:8080/install.ps1 | iex`""
    Write-Host ""
    Write-Host "  # Download script first, then run with parameters"
    Write-Host "  powershell -c `"`$s = irm http://192.168.1.100:8080/install.ps1; iex `"`$s -Model granite3.3:8b`"`""
}

function Get-OllamaModelsDir {
    $ollamaDir = if ($env:OLLAMA_MODELS) {
        $env:OLLAMA_MODELS
    } elseif ($env:USERPROFILE) {
        Join-Path $env:USERPROFILE ".ollama\models"
    } else {
        Join-Path $env:HOME ".ollama\models"
    }
    
    if (!(Test-Path $ollamaDir)) {
        Write-Host "📁 Creating Ollama models directory: $ollamaDir" -ForegroundColor Green
        New-Item -ItemType Directory -Path $ollamaDir -Force | Out-Null
        New-Item -ItemType Directory -Path (Join-Path $ollamaDir "manifests") -Force | Out-Null
        New-Item -ItemType Directory -Path (Join-Path $ollamaDir "blobs") -Force | Out-Null
    }
    
    return $ollamaDir
}

function Test-OllamaInstalled {
    try {
        $null = Get-Command ollama -ErrorAction Stop
        return $true
    } catch {
        return $false
    }
}

function Get-ServerFromRequest {
    # First check if Server parameter was provided
    if ($Server) {
        # Ensure http:// prefix
        if (!$Server.StartsWith("http")) {
            $Server = "http://$Server"
        }
        return $Server
    }
    
    # Try to extract server from auto-detection comment at top of script
    try {
        # Get the script content to look for AUTO_DETECTED_SERVER comment
        $scriptContent = $MyInvocation.MyCommand.ScriptContents
        if ($scriptContent) {
            $lines = $scriptContent.Split("`n")
            foreach ($line in $lines) {
                if ($line.StartsWith("# AUTO_DETECTED_SERVER=")) {
                    $detectedServer = $line.Substring("# AUTO_DETECTED_SERVER=".Length).Trim()
                    if ($detectedServer) {
                        Write-Host "🔍 Auto-detected server: $detectedServer" -ForegroundColor Green
                        return $detectedServer
                    }
                }
            }
        }
    } catch {
        # Ignore errors in auto-detection
    }
    
    # Try to extract server from the request context if available
    if ($PSScriptRoot -and (Test-Path (Join-Path $PSScriptRoot "server.txt"))) {
        return Get-Content (Join-Path $PSScriptRoot "server.txt") -Raw
    }
    
    # Prompt user for server address
    Write-Host "⚠️  Server address not provided" -ForegroundColor Yellow
    $inputServer = Read-Host "Enter distribution server address (e.g., 192.168.1.100:8080)"
    
    # Ensure http:// prefix
    if ($inputServer -and !$inputServer.StartsWith("http")) {
        $inputServer = "http://$inputServer"
    }
    
    return $inputServer
}

function Get-AvailableModels {
    param([string]$ServerUrl)
    
    try {
        Write-Host "📋 Fetching available models from $ServerUrl..." -ForegroundColor Blue
        $response = Invoke-RestMethod -Uri "$ServerUrl/api/models" -Method Get
        return $response
    } catch {
        Write-Error "Failed to fetch models from server: $_"
        return $null
    }
}

function Show-AvailableModels {
    param([array]$Models)
    
    Write-Host ""
    Write-Host "📦 Available Models:" -ForegroundColor Cyan
    Write-Host ("=" * 60)
    
    foreach ($model in $Models) {
        $sizeGB = [math]::Round($model.size / 1GB, 2)
        $modifiedDate = [DateTime]::Parse($model.modified).ToString("yyyy-MM-dd HH:mm")
        
        Write-Host "🔹 $($model.name):$($model.tag)" -ForegroundColor Green
        Write-Host "   Size: $sizeGB GB | Modified: $modifiedDate"
        Write-Host ""
    }
    
    Write-Host "Total Models: $($Models.Count)" -ForegroundColor Yellow
}

function Install-Model {
    param(
        [string]$ServerUrl,
        [string]$ModelName,
        [string]$ModelTag
    )
    
    $ollamaDir = Get-OllamaModelsDir
    $manifestsDir = Join-Path $ollamaDir "manifests\registry.ollama.ai\$ModelName"
    $blobsDir = Join-Path $ollamaDir "blobs"
    
    Write-Host "🚀 Installing model: ${ModelName}:${ModelTag}" -ForegroundColor Cyan
    Write-Host "📁 Target directory: $ollamaDir" -ForegroundColor Blue
    
    try {
        # Create directories
        if (!(Test-Path $manifestsDir)) {
            New-Item -ItemType Directory -Path $manifestsDir -Force | Out-Null
        }
        
        # Download manifest
        Write-Host "📄 Downloading manifest..." -ForegroundColor Blue
        $manifestUrl = "$ServerUrl/manifests/${ModelName}:${ModelTag}"
        $manifestPath = Join-Path $manifestsDir "$ModelTag.json"
        
        Invoke-WebRequest -Uri $manifestUrl -OutFile $manifestPath -UseBasicParsing
        
        # Parse manifest to get blobs
        $manifest = Get-Content $manifestPath | ConvertFrom-Json
        
        Write-Host "📦 Downloading model blobs..." -ForegroundColor Blue
        
        $totalBlobs = $manifest.layers.Count
        $currentBlob = 0
        
        foreach ($layer in $manifest.layers) {
            $currentBlob++
            $digest = $layer.digest
            $size = $layer.size
            $sizeGB = [math]::Round($size / 1GB, 2)
            
            # Convert colon to hyphen for Windows file system compatibility
            # Ollama stores blobs as sha256-abc123... but manifests reference them as sha256:abc123...
            $blobFileName = $digest -replace ":", "-"
            $blobPath = Join-Path $blobsDir $blobFileName
            
            Write-Host "  [$currentBlob/$totalBlobs] Downloading blob: $($digest.Substring(7, 12))... ($sizeGB GB)" -ForegroundColor Gray
            
            if (Test-Path $blobPath) {
                Write-Host "    ✅ Already exists, skipping" -ForegroundColor Green
                continue
            }
            
            # Use the correct blob URL for our model distribution server
            $blobUrl = "$ServerUrl/blobs/$digest"
            
            # Download with progress
            $tempPath = "$blobPath.tmp"
            try {
                # Try to download from the blob endpoint
                Invoke-WebRequest -Uri $blobUrl -OutFile $tempPath -UseBasicParsing
                Move-Item $tempPath $blobPath
                Write-Host "    ✅ Downloaded successfully" -ForegroundColor Green
            } catch {
                if (Test-Path $tempPath) {
                    Remove-Item $tempPath -Force
                }
                throw "Failed to download blob $digest`: $_"
            }
        }
        
        Write-Host ""
        Write-Host "✅ Model ${ModelName}:${ModelTag} installed successfully!" -ForegroundColor Green
        
        if (Test-OllamaInstalled) {
            Write-Host "🎯 You can now use: ollama run ${ModelName}:${ModelTag}" -ForegroundColor Cyan
        } else {
            Write-Host "⚠️  Ollama not found in PATH. Please install Ollama first." -ForegroundColor Yellow
        }
        
    } catch {
        Write-Error "Failed to install model: $_"
    }
}

# Main execution
if ($Help) {
    Show-Help
    return
}

# Check for environment variable model specification
if (!$Model -and $env:OLLAMA_MODEL) {
    $Model = $env:OLLAMA_MODEL
    Write-Host "📋 Using model from environment variable: $Model" -ForegroundColor Blue
}

# Get server URL
$ServerUrl = Get-ServerFromRequest

if (!$ServerUrl) {
    Write-Error "Server address is required. Use -Help for usage information."
    return
}

# Test server connectivity
try {
    Write-Host "🔍 Testing connection to $ServerUrl..." -ForegroundColor Blue
    $null = Invoke-RestMethod -Uri "$ServerUrl/health" -Method Get -TimeoutSec 10
    Write-Host "✅ Server is reachable" -ForegroundColor Green
} catch {
    Write-Error "Cannot connect to server $ServerUrl`: $_"
    return
}

# Get available models
$models = Get-AvailableModels -ServerUrl $ServerUrl
if (!$models) {
    Write-Error "Failed to retrieve models from server"
    return
}

if ($List) {
    Show-AvailableModels -Models $models
    return
}

if ($Model) {
    if ($Model -match "^(.+):(.+)$") {
        $modelName = $matches[1]
        $modelTag = $matches[2]
        
        # Check if model exists
        $foundModel = $models | Where-Object { $_.name -eq $modelName -and $_.tag -eq $modelTag }
        if (!$foundModel) {
            Write-Error "Model $Model not found on server. Use -List to see available models."
            return
        }
        
        Install-Model -ServerUrl $ServerUrl -ModelName $modelName -ModelTag $modelTag
    } else {
        Write-Error "Invalid model format. Use format: name:tag (e.g., granite3.3:8b)"
    }
} else {
    Write-Host "🚀 Ollama Model Distribution Client" -ForegroundColor Cyan
    Write-Host ""
    Show-AvailableModels -Models $models
    Write-Host ""
    Write-Host "To install a model, use one of these methods:" -ForegroundColor Yellow
    Write-Host "  # Method 1 (Environment Variable):"
    Write-Host "  powershell -c `"`$env:OLLAMA_MODEL='MODEL:TAG'; irm $ServerUrl/install.ps1 | iex`""
    Write-Host ""
    Write-Host "  # Method 2 (Download & Execute):"
    Write-Host "  powershell -c `"`$s = irm $ServerUrl/install.ps1; iex `"`$s -Model MODEL:TAG`"`""
    Write-Host ""
    Write-Host "Use -Help for more options." -ForegroundColor Gray
}
