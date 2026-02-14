#!/bin/sh
# install.sh – POSIX‑compliant installer for Terrarium Go application

set -e  # exit on any error

# Configuration
APP_NAME="terarrium-app"
INSTALL_DIR="/opt/terarrium"
SERVICE_NAME="terrarium.service"
SERVICE_FILE="./terrarium.service"   # adjust path if needed
USER="undead"

# Colour output (optional; some shells may not support \033)
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Use printf for portable coloured output
printf "${GREEN}Starting installation of Terrarium...${NC}\n"

# Check if running as root; if not, we'll use sudo
if [ "$(id -u)" -ne 0 ]; then
    printf "${YELLOW}Not running as root. Some commands will use sudo.${NC}\n"
    SUDO="sudo"
else
    SUDO=""
fi

# Verify required tools
if ! command -v go >/dev/null 2>&1; then
    printf "${RED}Go is not installed. Please install Go first.${NC}\n" >&2
    exit 1
fi
if ! command -v systemctl >/dev/null 2>&1; then
    printf "${RED}systemctl not found. This script requires systemd.${NC}\n" >&2
    exit 1
fi

# Create installation directory
printf "${GREEN}Creating installation directory at %s...${NC}\n" "$INSTALL_DIR"
$SUDO mkdir -p "$INSTALL_DIR"

# Build the Go binary
printf "${GREEN}Building %s...${NC}\n" "$APP_NAME"
go mod tidy
go build -o "$APP_NAME" .

# Move binary to installation directory
printf "${GREEN}Moving binary to %s...${NC}\n" "$INSTALL_DIR"
$SUDO mv "$APP_NAME" "$INSTALL_DIR/"
$SUDO chmod +x "$INSTALL_DIR/$APP_NAME"

# Copy (or create) systemd service file
printf "${GREEN}Installing systemd service...${NC}\n"
if [ ! -f "$SERVICE_FILE" ]; then
    printf "${YELLOW}Service file %s not found. Creating default...${NC}\n" "$SERVICE_FILE"
    # Write a default service file (using cat + heredoc)
    $SUDO tee "$SERVICE_FILE" >/dev/null <<EOF
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
    printf "${GREEN}Using existing %s.${NC}\n" "$SERVICE_FILE"
fi

$SUDO cp "$SERVICE_FILE" /etc/systemd/system/

# Reload systemd and enable/start service
printf "${GREEN}Reloading systemd daemon...${NC}\n"
$SUDO systemctl daemon-reload

printf "${GREEN}Enabling %s to start on boot...${NC}\n" "$SERVICE_NAME"
$SUDO systemctl enable "$SERVICE_NAME"

printf "${GREEN}Starting %s...${NC}\n" "$SERVICE_NAME"
$SUDO systemctl start "$SERVICE_NAME"

# Check service status
sleep 2
if $SUDO systemctl is-active --quiet "$SERVICE_NAME"; then
    printf "${GREEN}Service %s is running.${NC}\n" "$SERVICE_NAME"
    $SUDO systemctl status "$SERVICE_NAME" --no-pager
else
    printf "${RED}Service %s failed to start. Showing logs:${NC}\n" "$SERVICE_NAME"
    $SUDO journalctl -u "$SERVICE_NAME" -n 20 --no-pager
    exit 1
fi

printf "${GREEN}Installation completed successfully.${NC}\n"