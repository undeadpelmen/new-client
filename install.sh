#!/bin/bash
# install.sh – Automated installer for Terrarium Go application

set -e  # Exit on any error

# Configuration
APP_NAME="terarrium-app"
INSTALL_DIR="/opt/terarrium"
SERVICE_NAME="terrarium.service"
SERVICE_FILE="./terrarium.service"   # adjust if the file is elsewhere
USER="undead"                         # service will run as this user

# Colours for pretty output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Colour

echo -e "${GREEN}Starting installation of Terrarium...${NC}"

# Check if running as root (or with sudo)
if [ "$EUID" -ne 0 ]; then
    echo -e "${YELLOW}Not running as root. Some commands will use sudo.${NC}"
    SUDO="sudo"
else
    SUDO=""
fi

# 1. Verify required tools
command -v go >/dev/null 2>&1 || { echo -e "${RED}Go is not installed. Please install Go first.${NC}" >&2; exit 1; }
command -v systemctl >/dev/null 2>&1 || { echo -e "${RED}systemctl not found. This script requires systemd.${NC}" >&2; exit 1; }

# 2. Create installation directory
echo -e "${GREEN}Creating installation directory at $INSTALL_DIR...${NC}"
$SUDO mkdir -p "$INSTALL_DIR"

# 3. Build the Go binary
echo -e "${GREEN}Building $APP_NAME...${NC}"
go mod tidy
go build -o "$APP_NAME" .

# 4. Move binary to installation directory
echo -e "${GREEN}Moving binary to $INSTALL_DIR...${NC}"
$SUDO mv "$APP_NAME" "$INSTALL_DIR/"
$SUDO chmod +x "$INSTALL_DIR/$APP_NAME"

# 5. Copy systemd service file
echo -e "${GREEN}Installing systemd service...${NC}"
if [ ! -f "$SERVICE_FILE" ]; then
    echo -e "${YELLOW}Service file $SERVICE_FILE not found. Creating default...${NC}"
    # Create a default service file (adjust ExecStart if needed)
    cat << EOF | $SUDO tee "$SERVICE_FILE" >/dev/null
[Unit]
Description=Terrarium Go Application
After=network.target

[Service]
Type=simple
User=$USER
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/$APP_NAME
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF
else
    echo -e "${GREEN}Using existing $SERVICE_FILE.${NC}"
fi

$SUDO cp "$SERVICE_FILE" /etc/systemd/system/

# 6. Reload systemd and enable/start service
echo -e "${GREEN}Reloading systemd daemon...${NC}"
$SUDO systemctl daemon-reload

echo -e "${GREEN}Enabling $SERVICE_NAME to start on boot...${NC}"
$SUDO systemctl enable "$SERVICE_NAME"

echo -e "${GREEN}Starting $SERVICE_NAME...${NC}"
$SUDO systemctl start "$SERVICE_NAME"

# 7. Check service status
sleep 2  # give it a moment
if $SUDO systemctl is-active --quiet "$SERVICE_NAME"; then
    echo -e "${GREEN}Service $SERVICE_NAME is running.${NC}"
    $SUDO systemctl status "$SERVICE_NAME" --no-pager
else
    echo -e "${RED}Service $SERVICE_NAME failed to start. Showing logs:${NC}"
    $SUDO journalctl -u "$SERVICE_NAME" -n 20 --no-pager
    exit 1
fi

echo -e "${GREEN}Installation completed successfully.${NC}"