#!/bin/bash
#
# CacheStorm One-Click Setup Script for Linux/macOS
# Usage: curl -fsSL https://raw.githubusercontent.com/cachestorm/cachestorm/main/scripts/install.sh | bash
#        or: wget -qO- https://raw.githubusercontent.com/cachestorm/cachestorm/main/scripts/install.sh | bash
#

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
REPO="cachestorm/cachestorm"
INSTALL_DIR="/usr/local/bin"
DATA_DIR="$HOME/.cachestorm"
CONFIG_DIR="$HOME/.cachestorm/config"
DOCKER_COMPOSE_URL="https://raw.githubusercontent.com/${REPO}/main/docker/docker-compose.yml"

# Detect OS and Architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$ARCH" in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        arm64|aarch64)
            ARCH="arm64"
            ;;
        armv7l)
            ARCH="armv7"
            ;;
        *)
            echo -e "${RED}Unsupported architecture: $ARCH${NC}"
            exit 1
            ;;
    esac

    case "$OS" in
        linux)
            PLATFORM="linux"
            ;;
        darwin)
            PLATFORM="darwin"
            ;;
        *)
            echo -e "${RED}Unsupported operating system: $OS${NC}"
            exit 1
            ;;
    esac

    echo -e "${BLUE}Detected platform: ${PLATFORM}/${ARCH}${NC}"
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Print banner
print_banner() {
    echo -e "${BLUE}"
    echo "╔══════════════════════════════════════════════════════════════╗"
    echo "║                                                              ║"
    echo "║                    CacheStorm Installer                      ║"
    echo "║                                                              ║"
    echo "║          High-Performance Redis-Compatible Database          ║"
    echo "║                                                              ║"
    echo "╚══════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
}

# Check prerequisites
check_prerequisites() {
    echo -e "${YELLOW}Checking prerequisites...${NC}"

    if command_exists docker; then
        DOCKER_VERSION=$(docker --version | cut -d ' ' -f3 | tr -d ',')
        echo -e "${GREEN}✓ Docker found (v${DOCKER_VERSION})${NC}"
        HAS_DOCKER=true
    else
        echo -e "${YELLOW}✗ Docker not found${NC}"
        HAS_DOCKER=false
    fi

    if command_exists docker-compose || command_exists "docker compose"; then
        echo -e "${GREEN}✓ Docker Compose found${NC}"
        HAS_DOCKER_COMPOSE=true
    else
        echo -e "${YELLOW}✗ Docker Compose not found${NC}"
        HAS_DOCKER_COMPOSE=false
    fi

    if command_exists go; then
        GO_VERSION=$(go version | cut -d ' ' -f3)
        echo -e "${GREEN}✓ Go found (${GO_VERSION})${NC}"
        HAS_GO=true
    else
        echo -e "${YELLOW}✗ Go not found${NC}"
        HAS_GO=false
    fi

    echo ""
}

# Install using Docker (recommended)
install_docker() {
    echo -e "${YELLOW}Installing CacheStorm with Docker...${NC}"

    # Create directories
    mkdir -p "$DATA_DIR/data" "$CONFIG_DIR"

    # Create default config if not exists
    if [ ! -f "$CONFIG_DIR/cachestorm.yaml" ]; then
        cat > "$CONFIG_DIR/cachestorm.yaml" << 'EOF'
server:
  port: 6379
  http_port: 8080
  bind: 0.0.0.0

storage:
  max_memory: 1gb
  eviction_policy: allkeys-lru

persistence:
  enabled: true
  mode: aof
  aof_fsync: everysec

logging:
  level: info
  format: json
EOF
        echo -e "${GREEN}✓ Created default configuration${NC}"
    fi

    # Create docker-compose.yml
    cat > "$DATA_DIR/docker-compose.yml" << EOF
version: '3.8'

services:
  cachestorm:
    image: ${REPO}:latest
    container_name: cachestorm
    ports:
      - "6379:6379"
      - "8080:8080"
    volumes:
      - ${DATA_DIR}/data:/data
      - ${CONFIG_DIR}/cachestorm.yaml:/etc/cachestorm/cachestorm.yaml:ro
    environment:
      - CACHESTORM_CONFIG=/etc/cachestorm/cachestorm.yaml
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 3s
      retries: 3
      start_period: 10s

  # Optional: Redis Insight for GUI management
  redisinsight:
    image: redis/redisinsight:latest
    container_name: cachestorm-insight
    ports:
      - "5540:5540"
    volumes:
      - ${DATA_DIR}/redisinsight:/data
    restart: unless-stopped
    profiles:
      - gui
EOF

    echo -e "${GREEN}✓ Created docker-compose.yml${NC}"

    # Start CacheStorm
    cd "$DATA_DIR"
    docker-compose up -d

    echo ""
    echo -e "${GREEN}✓ CacheStorm is running!${NC}"
    echo ""
    echo -e "  ${BLUE}Redis Protocol:${NC} localhost:6379"
    echo -e "  ${BLUE}HTTP API:${NC}      http://localhost:8080"
    echo -e "  ${BLUE}Admin UI:${NC}      http://localhost:8080"
    echo -e "  ${BLUE}Data Directory:${NC} ${DATA_DIR}/data"
    echo ""
    echo -e "  ${YELLOW}Commands:${NC}"
    echo -e "    docker-compose -f ${DATA_DIR}/docker-compose.yml logs -f"
    echo -e "    docker-compose -f ${DATA_DIR}/docker-compose.yml stop"
    echo -e "    docker-compose -f ${DATA_DIR}/docker-compose.yml start"
    echo ""
    echo -e "  ${YELLOW}To add GUI (Redis Insight):${NC}"
    echo -e "    docker-compose -f ${DATA_DIR}/docker-compose.yml --profile gui up -d"
    echo ""
}

# Install binary directly
install_binary() {
    echo -e "${YELLOW}Installing CacheStorm binary...${NC}"

    # Get latest release URL
    LATEST_URL="https://api.github.com/repos/${REPO}/releases/latest"

    echo -e "${BLUE}Fetching latest release...${NC}"

    if command_exists curl; then
        DOWNLOAD_URL=$(curl -s "$LATEST_URL" | grep "browser_download_url.*${PLATFORM}_${ARCH}" | cut -d '"' -f 4)
    elif command_exists wget; then
        DOWNLOAD_URL=$(wget -qO- "$LATEST_URL" | grep "browser_download_url.*${PLATFORM}_${ARCH}" | cut -d '"' -f 4)
    else
        echo -e "${RED}Error: curl or wget is required${NC}"
        exit 1
    fi

    if [ -z "$DOWNLOAD_URL" ]; then
        echo -e "${RED}Error: Could not find release for ${PLATFORM}/${ARCH}${NC}"
        exit 1
    fi

    # Download
    TMP_DIR=$(mktemp -d)
    TMP_FILE="$TMP_DIR/cachestorm.tar.gz"

    echo -e "${BLUE}Downloading from: $DOWNLOAD_URL${NC}"

    if command_exists curl; then
        curl -fsSL -o "$TMP_FILE" "$DOWNLOAD_URL"
    else
        wget -qO "$TMP_FILE" "$DOWNLOAD_URL"
    fi

    # Extract
    echo -e "${BLUE}Extracting...${NC}"
    tar -xzf "$TMP_FILE" -C "$TMP_DIR"

    # Install
    if [ -w "$INSTALL_DIR" ]; then
        mv "$TMP_DIR/cachestorm" "$INSTALL_DIR/"
    else
        echo -e "${YELLOW}Requesting sudo access to install to $INSTALL_DIR${NC}"
        sudo mv "$TMP_DIR/cachestorm" "$INSTALL_DIR/"
    fi

    chmod +x "$INSTALL_DIR/cachestorm"

    # Cleanup
    rm -rf "$TMP_DIR"

    # Create directories
    mkdir -p "$DATA_DIR/data" "$CONFIG_DIR"

    # Create default config
    if [ ! -f "$CONFIG_DIR/cachestorm.yaml" ]; then
        cat > "$CONFIG_DIR/cachestorm.yaml" << 'EOF'
server:
  port: 6379
  http_port: 8080
  bind: 0.0.0.0

storage:
  max_memory: 1gb
  eviction_policy: allkeys-lru

persistence:
  enabled: true
  mode: aof
  aof_fsync: everysec

logging:
  level: info
  format: json
EOF
    fi

    # Create systemd service (Linux only)
    if [ "$PLATFORM" = "linux" ] && command_exists systemctl; then
        create_systemd_service
    fi

    echo ""
    echo -e "${GREEN}✓ CacheStorm binary installed to ${INSTALL_DIR}/cachestorm${NC}"
    echo ""
    echo -e "  ${YELLOW}To start CacheStorm:${NC}"
    echo -e "    cachestorm --config ${CONFIG_DIR}/cachestorm.yaml"
    echo ""
    echo -e "  ${YELLOW}Or create a systemd service:${NC}"
    echo -e "    sudo systemctl start cachestorm"
    echo ""
}

# Create systemd service
create_systemd_service() {
    echo -e "${BLUE}Creating systemd service...${NC}"

    SERVICE_FILE="/etc/systemd/system/cachestorm.service"

    if [ -w "/etc/systemd/system" ]; then
        cat > "$SERVICE_FILE" << EOF
[Unit]
Description=CacheStorm - High-Performance Redis-Compatible Database
After=network.target

[Service]
Type=simple
User=$USER
ExecStart=${INSTALL_DIR}/cachestorm --config ${CONFIG_DIR}/cachestorm.yaml
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=cachestorm

[Install]
WantedBy=multi-user.target
EOF
        systemctl daemon-reload
        echo -e "${GREEN}✓ Created systemd service${NC}"
        echo -e "  ${YELLOW}Run: sudo systemctl enable --now cachestorm${NC}"
    else
        echo -e "${YELLOW}Creating systemd service (requires sudo)...${NC}"
        sudo tee "$SERVICE_FILE" > /dev/null << EOF
[Unit]
Description=CacheStorm - High-Performance Redis-Compatible Database
After=network.target

[Service]
Type=simple
User=$USER
ExecStart=${INSTALL_DIR}/cachestorm --config ${CONFIG_DIR}/cachestorm.yaml
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=cachestorm

[Install]
WantedBy=multi-user.target
EOF
        sudo systemctl daemon-reload
        echo -e "${GREEN}✓ Created systemd service${NC}"
        echo -e "  ${YELLOW}Run: sudo systemctl enable --now cachestorm${NC}"
    fi
}

# Build from source
install_source() {
    echo -e "${YELLOW}Building CacheStorm from source...${NC}"

    if ! command_exists go; then
        echo -e "${RED}Error: Go is required to build from source${NC}"
        echo -e "  Install Go: https://golang.org/dl/"
        exit 1
    fi

    # Check Go version
    GO_VERSION=$(go version | grep -o 'go[0-9.]*' | head -1 | tr -d 'go')
    REQUIRED_VERSION="1.22"

    if [ "$(printf '%s\n' "$REQUIRED_VERSION" "$GO_VERSION" | sort -V | head -n1)" != "$REQUIRED_VERSION" ]; then
        echo -e "${RED}Error: Go ${REQUIRED_VERSION}+ required (found ${GO_VERSION})${NC}"
        exit 1
    fi

    # Clone and build
    TMP_DIR=$(mktemp -d)
    cd "$TMP_DIR"

    echo -e "${BLUE}Cloning repository...${NC}"
    git clone --depth 1 https://github.com/${REPO}.git

    cd cachestorm

    echo -e "${BLUE}Building...${NC}"
    go build -ldflags="-s -w" -o cachestorm ./cmd/cachestorm

    # Install
    if [ -w "$INSTALL_DIR" ]; then
        mv cachestorm "$INSTALL_DIR/"
    else
        echo -e "${YELLOW}Requesting sudo access to install to $INSTALL_DIR${NC}"
        sudo mv cachestorm "$INSTALL_DIR/"
    fi

    chmod +x "$INSTALL_DIR/cachestorm"

    # Cleanup
    cd "$HOME"
    rm -rf "$TMP_DIR"

    # Create directories and config
    mkdir -p "$DATA_DIR/data" "$CONFIG_DIR"

    if [ ! -f "$CONFIG_DIR/cachestorm.yaml" ]; then
        cp "$TMP_DIR/cachestorm/config/example.yaml" "$CONFIG_DIR/cachestorm.yaml" 2>/dev/null || cat > "$CONFIG_DIR/cachestorm.yaml" << 'EOF'
server:
  port: 6379
  http_port: 8080
  bind: 0.0.0.0

storage:
  max_memory: 1gb
  eviction_policy: allkeys-lru

persistence:
  enabled: true
  mode: aof
  aof_fsync: everysec

logging:
  level: info
  format: json
EOF
    fi

    echo ""
    echo -e "${GREEN}✓ CacheStorm built and installed from source${NC}"
    echo ""
    echo -e "  ${YELLOW}To start:${NC} cachestorm --config ${CONFIG_DIR}/cachestorm.yaml"
    echo ""
}

# Uninstall
uninstall() {
    echo -e "${YELLOW}Uninstalling CacheStorm...${NC}"

    # Stop and remove Docker containers
    if [ -f "$DATA_DIR/docker-compose.yml" ]; then
        echo -e "${BLUE}Stopping Docker containers...${NC}"
        docker-compose -f "$DATA_DIR/docker-compose.yml" down 2>/dev/null || true
    fi

    # Stop systemd service
    if command_exists systemctl && systemctl list-unit-files | grep -q cachestorm; then
        echo -e "${BLUE}Stopping systemd service...${NC}"
        sudo systemctl stop cachestorm 2>/dev/null || true
        sudo systemctl disable cachestorm 2>/dev/null || true
        sudo rm -f /etc/systemd/system/cachestorm.service
        sudo systemctl daemon-reload
    fi

    # Remove binary
    if [ -f "$INSTALL_DIR/cachestorm" ]; then
        echo -e "${BLUE}Removing binary...${NC}"
        if [ -w "$INSTALL_DIR" ]; then
            rm -f "$INSTALL_DIR/cachestorm"
        else
            sudo rm -f "$INSTALL_DIR/cachestorm"
        fi
    fi

    echo ""
    echo -e "${GREEN}✓ CacheStorm uninstalled${NC}"
    echo ""
    echo -e "  ${YELLOW}Data directory preserved at:${NC} $DATA_DIR"
    echo -e "  ${YELLOW}To remove data, run:${NC} rm -rf $DATA_DIR"
    echo ""
}

# Show help
show_help() {
    echo "CacheStorm Installer"
    echo ""
    echo "Usage:"
    echo "  curl -fsSL .../install.sh | bash              # Interactive install"
    echo "  curl -fsSL .../install.sh | bash -s -- docker # Force Docker install"
    echo "  curl -fsSL .../install.sh | bash -s -- binary # Force binary install"
    echo "  curl -fsSL .../install.sh | bash -s -- source # Build from source"
    echo "  curl -fsSL .../install.sh | bash -s -- uninstall"
    echo ""
    echo "Options:"
    echo "  docker     Install using Docker (recommended)"
    echo "  binary     Download and install pre-built binary"
    echo "  source     Build and install from source"
    echo "  uninstall  Remove CacheStorm"
    echo "  help       Show this help message"
    echo ""
}

# Main installation logic
main() {
    print_banner

    # Handle command line arguments
    METHOD="${1:-auto}"

    if [ "$METHOD" = "help" ] || [ "$METHOD" = "--help" ] || [ "$METHOD" = "-h" ]; then
        show_help
        exit 0
    fi

    if [ "$METHOD" = "uninstall" ]; then
        uninstall
        exit 0
    fi

    detect_platform
    check_prerequisites

    # Auto-detect best method or use specified
    if [ "$METHOD" = "auto" ]; then
        if [ "$HAS_DOCKER" = true ] && [ "$HAS_DOCKER_COMPOSE" = true ]; then
            METHOD="docker"
        elif [ "$HAS_GO" = true ]; then
            METHOD="source"
        else
            METHOD="binary"
        fi
        echo -e "${BLUE}Auto-selected installation method: $METHOD${NC}"
        echo ""
    fi

    # Confirm installation
    if [ "$METHOD" != "auto" ]; then
        echo -e "${YELLOW}This will install CacheStorm using: ${GREEN}$METHOD${NC}"
        echo -e "${YELLOW}Press Enter to continue or Ctrl+C to cancel...${NC}"
        read -r
    fi

    # Install based on method
    case "$METHOD" in
        docker)
            install_docker
            ;;
        binary)
            install_binary
            ;;
        source)
            install_source
            ;;
        *)
            echo -e "${RED}Unknown installation method: $METHOD${NC}"
            show_help
            exit 1
            ;;
    esac

    # Test installation
    echo -e "${BLUE}Testing installation...${NC}"

    if [ "$METHOD" = "docker" ]; then
        sleep 3
        if docker ps | grep -q cachestorm; then
            echo -e "${GREEN}✓ CacheStorm container is running${NC}"

            # Test connection
            if command_exists redis-cli; then
                if redis-cli -p 6379 PING 2>/dev/null | grep -q PONG; then
                    echo -e "${GREEN}✓ Redis protocol responding${NC}"
                fi
            fi

            if command_exists curl; then
                if curl -s http://localhost:8080/health 2>/dev/null | grep -q "ok"; then
                    echo -e "${GREEN}✓ HTTP API responding${NC}"
                fi
            fi
        fi
    elif command_exists cachestorm; then
        echo -e "${GREEN}✓ CacheStorm binary installed${NC}"
        cachestorm -version 2>/dev/null || true
    fi

    echo ""
    echo -e "${GREEN}══════════════════════════════════════════════════════════════${NC}"
    echo -e "${GREEN}  Installation Complete!${NC}"
    echo -e "${GREEN}══════════════════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "  ${BLUE}Documentation:${NC} https://github.com/cachestorm/cachestorm"
    echo -e "  ${BLUE}Issues:${NC}        https://github.com/cachestorm/cachestorm/issues"
    echo ""
}

# Run main function
main "$@"
