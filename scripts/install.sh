#!/bin/bash
# Ollama Model Installer for Linux/macOS
# Downloads and installs models from the ollama-lancache server

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
GRAY='\033[0;37m'
NC='\033[0m' # No Color

# Default values
SERVER=""
MODEL=""
LIST_MODELS=false
SHOW_HELP=false

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --server)
            SERVER="$2"
            shift 2
            ;;
        --model)
            MODEL="$2"
            shift 2
            ;;
        --list)
            LIST_MODELS=true
            shift
            ;;
        --help|-h)
            SHOW_HELP=true
            shift
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            exit 1
            ;;
    esac
done

show_help() {
    echo -e "${CYAN}🚀 Ollama Model Installer for Linux/macOS${NC}"
    echo ""
    echo -e "${YELLOW}USAGE:${NC}"
    echo "  curl -fsSL http://SERVER:PORT/install.sh | bash"
    echo "  curl -fsSL http://SERVER:PORT/install.sh | bash -s -- --server SERVER:PORT --model MODEL:TAG"
    echo ""
    echo -e "${YELLOW}PARAMETERS:${NC}"
    echo "  --server   ollama-lancache server address (e.g., 192.168.1.100:8080)"
    echo "  --model    Model to install (e.g., granite3.3:8b)"
    echo "  --list     List available models"
    echo "  --help     Show this help"
    echo ""
    echo -e "${YELLOW}EXAMPLES:${NC}"
    echo "  # List available models"
    echo "  curl -fsSL http://192.168.1.100:8080/install.sh | bash -s -- --list"
    echo ""
    echo "  # Install specific model"
    echo "  curl -fsSL http://192.168.1.100:8080/install.sh | bash -s -- --model granite3.3:8b"
}

get_ollama_models_dir() {
    if [[ -n "$OLLAMA_MODELS" ]]; then
        echo "$OLLAMA_MODELS"
    elif [[ -n "$HOME" ]]; then
        echo "$HOME/.ollama/models"
    else
        echo "/tmp/.ollama/models"
    fi
}

test_ollama_installed() {
    command -v ollama >/dev/null 2>&1
}

get_server_from_request() {
    local server_result=""
    
    # Try to extract server from environment or detect from curl
    if [[ -z "$SERVER" ]]; then
        # Try to detect from the URL this script was downloaded from
        if [[ -n "$HTTP_REFERER" ]]; then
            server_result="$HTTP_REFERER"
        elif [[ -n "$1" ]]; then
            server_result="$1"
        else
            # Provide default server address with option to customize
            local default_server="http://192.168.1.100:8080"
            echo -e "${CYAN}🌐 Default server address: $default_server${NC}" >&2
            echo -e "${YELLOW}Press Enter to accept default, or type a custom server address:${NC}" >&2
            read -p "Server address: " server_result
            
            # Use default if user just pressed Enter
            if [[ -z "$server_result" ]]; then
                server_result="$default_server"
                echo -e "${GREEN}✅ Using default server: $server_result${NC}" >&2
            else
                echo -e "${GREEN}✅ Using custom server: $server_result${NC}" >&2
            fi
        fi
    else
        server_result="$SERVER"
    fi
    
    # Ensure http:// prefix
    if [[ -n "$server_result" && ! "$server_result" =~ ^https?:// ]]; then
        server_result="http://$server_result"
    fi
    
    echo "$server_result"
}

get_available_models() {
    local server_url="$1"
    
    echo -e "${BLUE}📋 Fetching available models from $server_url...${NC}" >&2
    
    local response
    if ! response=$(curl -fsSL "$server_url/api/models" 2>/dev/null); then
        echo -e "${RED}Failed to fetch models from server${NC}" >&2
        return 1
    fi
    
    # Validate JSON response
    if [[ -z "$response" ]]; then
        echo -e "${RED}Empty response from server${NC}" >&2
        return 1
    fi
    
    # Test if it's valid JSON
    if command -v jq >/dev/null 2>&1; then
        if ! echo "$response" | jq . >/dev/null 2>&1; then
            echo -e "${RED}Invalid JSON response from server${NC}" >&2
            echo "Response: $response" >&2
            return 1
        fi
    fi
    
    echo "$response"
}

show_available_models() {
    local models_json="$1"
    
    echo ""
    echo -e "${CYAN}📦 Available Models:${NC}"
    echo "============================================================"
    
    # Parse JSON and display models (requires jq, fallback to basic parsing)
    if command -v jq >/dev/null 2>&1; then
        echo "$models_json" | jq -r '.[] | "🔹 \(.name):\(.tag)\n   Size: \((.size / 1024 / 1024 / 1024 * 100 | floor) / 100) GB | Modified: \(.modified[:19])\n"'
    else
        # Basic parsing without jq
        echo "$models_json" | sed 's/},{/}\n{/g' | while read -r line; do
            if [[ "$line" =~ \"name\":\"([^\"]+)\" ]]; then
                name="${BASH_REMATCH[1]}"
            fi
            if [[ "$line" =~ \"tag\":\"([^\"]+)\" ]]; then
                tag="${BASH_REMATCH[1]}"
            fi
            if [[ "$line" =~ \"size\":([0-9]+) ]]; then
                size="${BASH_REMATCH[1]}"
                size_gb=$(echo "scale=2; $size / 1073741824" | bc 2>/dev/null || echo "N/A")
            fi
            if [[ "$line" =~ \"modified\":\"([^\"]+)\" ]]; then
                modified="${BASH_REMATCH[1]}"
                echo -e "${GREEN}🔹 $name:$tag${NC}"
                echo "   Size: $size_gb GB | Modified: ${modified:0:19}"
                echo ""
            fi
        done
    fi
    
    echo -e "${YELLOW}Use --model MODEL:TAG to install a specific model${NC}"
}

install_model() {
    local server_url="$1"
    local model_name="$2"
    local model_tag="$3"
    
    local ollama_dir
    ollama_dir=$(get_ollama_models_dir)
    local manifests_dir="$ollama_dir/manifests/registry.ollama.ai/$model_name"
    local blobs_dir="$ollama_dir/blobs"
    
    echo -e "${CYAN}🚀 Installing model: $model_name:$model_tag${NC}"
    echo -e "${BLUE}📁 Target directory: $ollama_dir${NC}"
    
    # Create directories
    mkdir -p "$manifests_dir" "$blobs_dir"
    
    # Download manifest
    echo -e "${BLUE}📄 Downloading manifest...${NC}"
    local manifest_url="$server_url/manifests/$model_name:$model_tag"
    local manifest_path="$manifests_dir/$model_tag.json"
    
    if ! curl -fsSL "$manifest_url" -o "$manifest_path"; then
        echo -e "${RED}Failed to download manifest${NC}" >&2
        return 1
    fi
    
    # Parse manifest to get blobs
    echo -e "${BLUE}📦 Downloading model blobs...${NC}"
    
    local blob_count=0
    local total_blobs
    
    if command -v jq >/dev/null 2>&1; then
        total_blobs=$(jq '.layers | length' "$manifest_path")
        
        jq -r '.layers[] | "\(.digest) \(.size)"' "$manifest_path" | while read -r digest size; do
            blob_count=$((blob_count + 1))
            local size_gb
            size_gb=$(echo "scale=2; $size / 1073741824" | bc 2>/dev/null || echo "N/A")
            local digest_short="${digest:7:12}"
            
            local blob_path="$blobs_dir/$digest"
            
            echo -e "${GRAY}  [$blob_count/$total_blobs] Downloading blob: $digest_short... ($size_gb GB)${NC}"
            
            if [[ -f "$blob_path" ]]; then
                echo -e "${GREEN}    ✅ Already exists, skipping${NC}"
                continue
            fi
            
            local blob_url="$server_url/blobs/$digest"
            local temp_path="$blob_path.tmp"
            
            if curl -fsSL "$blob_url" -o "$temp_path" && mv "$temp_path" "$blob_path"; then
                echo -e "${GREEN}    ✅ Downloaded successfully${NC}"
            else
                echo -e "${RED}    ❌ Failed to download blob $digest${NC}" >&2
                [[ -f "$temp_path" ]] && rm -f "$temp_path"
                return 1
            fi
        done
    else
        echo -e "${YELLOW}Warning: jq not found, using basic parsing${NC}"
        # Basic parsing without jq
        grep -o '"digest":"[^"]*"' "$manifest_path" | sed 's/"digest":"//;s/"//' | while read -r digest; do
            blob_count=$((blob_count + 1))
            local digest_short="${digest:7:12}"
            local blob_path="$blobs_dir/$digest"
            
            echo -e "${GRAY}  [$blob_count/?] Downloading blob: $digest_short...${NC}"
            
            if [[ -f "$blob_path" ]]; then
                echo -e "${GREEN}    ✅ Already exists, skipping${NC}"
                continue
            fi
            
            local blob_url="$server_url/blobs/$digest"
            local temp_path="$blob_path.tmp"
            
            if curl -fsSL "$blob_url" -o "$temp_path" && mv "$temp_path" "$blob_path"; then
                echo -e "${GREEN}    ✅ Downloaded successfully${NC}"
            else
                echo -e "${RED}    ❌ Failed to download blob $digest${NC}" >&2
                [[ -f "$temp_path" ]] && rm -f "$temp_path"
                return 1
            fi
        done
    fi
    
    echo ""
    echo -e "${GREEN}✅ Model $model_name:$model_tag installed successfully!${NC}"
    
    if test_ollama_installed; then
        echo -e "${CYAN}🎯 You can now use: ollama run $model_name:$model_tag${NC}"
    else
        echo -e "${YELLOW}⚠️  Ollama not found in PATH. Please install Ollama first.${NC}"
    fi
}

# Main execution
if [[ "$SHOW_HELP" == true ]]; then
    show_help
    exit 0
fi

# Get server URL
SERVER_URL=$(get_server_from_request "$1")

if [[ -z "$SERVER_URL" ]]; then
    echo -e "${RED}Server address is required. Use --help for usage information.${NC}" >&2
    exit 1
fi

# Test server connectivity
echo -e "${BLUE}🔍 Testing connection to $SERVER_URL...${NC}"
if curl -fsSL "$SERVER_URL/health" -o /dev/null; then
    echo -e "${GREEN}✅ Server is reachable${NC}"
else
    echo -e "${RED}Cannot connect to server $SERVER_URL${NC}" >&2
    exit 1
fi

# Get available models
models_json=$(get_available_models "$SERVER_URL")
if [[ $? -ne 0 ]]; then
    echo -e "${RED}Failed to retrieve models from server${NC}" >&2
    exit 1
fi

if [[ "$LIST_MODELS" == true ]]; then
    show_available_models "$models_json"
    exit 0
fi

if [[ -n "$MODEL" ]]; then
    if [[ "$MODEL" =~ ^(.+):(.+)$ ]]; then
        model_name="${BASH_REMATCH[1]}"
        model_tag="${BASH_REMATCH[2]}"
        
        # Check if model exists
        if command -v jq >/dev/null 2>&1; then
            if ! echo "$models_json" | jq -e ".[] | select(.name == \"$model_name\" and .tag == \"$model_tag\")" >/dev/null; then
                echo -e "${RED}Model $MODEL not found on server. Use --list to see available models.${NC}" >&2
                exit 1
            fi
        else
            echo -e "${YELLOW}Warning: jq not found, skipping model existence check${NC}"
        fi
        
        install_model "$SERVER_URL" "$model_name" "$model_tag"
    else
        echo -e "${RED}Invalid model format. Use format: name:tag (e.g., granite3.3:8b)${NC}" >&2
        exit 1
    fi
else
    echo -e "${CYAN}🚀 ollama-lancache Client${NC}"
    echo ""
    show_available_models "$models_json"
    echo ""
    echo -e "${YELLOW}To install a model, run:${NC}"
    echo "curl -fsSL $SERVER_URL/install.sh | bash -s -- --model MODEL:TAG"
    echo ""
    echo -e "${GRAY}Use --help for more options.${NC}"
fi
