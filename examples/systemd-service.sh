#!/bin/bash
# Script to install Ollama LanCache as a systemd service

set -e

# Configuration
BINARY_PATH="/usr/local/bin/ollama-lancache"
SERVICE_NAME="ollama-lancache"
SERVICE_USER="ollama"
MODELS_DIR="/var/lib/ollama/models"
PORT="8080"

echo "🚀 Installing Ollama LanCache as systemd service..."

# Create user if it doesn't exist
if ! id "$SERVICE_USER" &>/dev/null; then
    echo "Creating user: $SERVICE_USER"
    sudo useradd --system --shell /bin/false --home-dir /var/lib/ollama --create-home "$SERVICE_USER"
fi

# Create models directory
echo "Creating models directory: $MODELS_DIR"
sudo mkdir -p "$MODELS_DIR"
sudo chown "$SERVICE_USER:$SERVICE_USER" "$MODELS_DIR"

# Copy binary (assumes ollama-lancache is in current directory)
if [[ -f "./ollama-lancache" ]]; then
    echo "Installing binary to: $BINARY_PATH"
    sudo cp "./ollama-lancache" "$BINARY_PATH"
    sudo chmod +x "$BINARY_PATH"
    sudo chown root:root "$BINARY_PATH"
else
    echo "❌ Binary ./ollama-lancache not found in current directory"
    echo "Please build the binary first: make build"
    exit 1
fi

# Create systemd service file
echo "Creating systemd service file..."
sudo tee "/etc/systemd/system/${SERVICE_NAME}.service" > /dev/null << EOF
[Unit]
Description=Ollama LanCache Model Distribution Server
Documentation=https://github.com/jjasghar/ollama-lancache
After=network.target
Wants=network.target

[Service]
Type=simple
User=$SERVICE_USER
Group=$SERVICE_USER
ExecStart=$BINARY_PATH serve --port $PORT --models-dir $MODELS_DIR
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=$SERVICE_NAME

# Security settings
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=$MODELS_DIR
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectControlGroups=yes

[Install]
WantedBy=multi-user.target
EOF

# Reload systemd and enable service
echo "Enabling and starting service..."
sudo systemctl daemon-reload
sudo systemctl enable "$SERVICE_NAME"
sudo systemctl start "$SERVICE_NAME"

# Show status
echo ""
echo "✅ Installation complete!"
echo ""
echo "Service status:"
sudo systemctl status "$SERVICE_NAME" --no-pager

echo ""
echo "🌐 Service should be available at: http://localhost:$PORT"
echo ""
echo "Useful commands:"
echo "  sudo systemctl status $SERVICE_NAME    # Check status"
echo "  sudo systemctl restart $SERVICE_NAME   # Restart service"
echo "  sudo systemctl stop $SERVICE_NAME      # Stop service"
echo "  sudo journalctl -u $SERVICE_NAME -f    # View logs"
echo ""
echo "📁 Models directory: $MODELS_DIR"
echo "📋 Copy your Ollama models to this directory for sharing"
