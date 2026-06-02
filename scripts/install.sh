#!/bin/bash

set -e

# Repository information
OWNER="AbdelrahmanDwedar"
REPO="mig"
CLOUDSMITH_REPO="AbdelrahmanDwedar/stable"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}Mig Installation Script${NC}"
echo "--------------------------"

# Detect OS
if [[ "$OSTYPE" != "linux-gnu"* ]]; then
    echo -e "${RED}Error: This script only supports Linux.${NC}"
    exit 1
fi

# Detect Package Manager
if command -v apt-get >/dev/null; then
    PM="apt"
elif command -v dnf >/dev/null; then
    PM="dnf"
elif command -v yum >/dev/null; then
    PM="yum"
else
    PM="manual"
fi

install_via_repo() {
    echo -e "${GREEN}Setting up Cloudsmith repository...${NC}"
    if [ "$PM" == "apt" ]; then
        curl -1sLf "https://dl.cloudsmith.io/public/${CLOUDSMITH_REPO}/setup.deb.sh" | sudo -E bash
        sudo apt-get install -y mig
    elif [ "$PM" == "dnf" ] || [ "$PM" == "yum" ]; then
        curl -1sLf "https://dl.cloudsmith.io/public/${CLOUDSMITH_REPO}/setup.rpm.sh" | sudo -E bash
        sudo $PM install -y mig
    fi
    echo -e "${GREEN}Mig installed successfully via $PM!${NC}"
}

install_manual() {
    echo -e "${BLUE}Installing binary directly from GitHub...${NC}"
    
    ARCH=$(uname -m)
    case $ARCH in
        x86_64) ARCH="amd64" ;;
        aarch64) ARCH="arm64" ;;
        *) echo -e "${RED}Unsupported architecture: $ARCH${NC}"; exit 1 ;;
    esac

    # Get latest version
    VERSION=$(curl -s https://api.github.com/repos/${OWNER}/${REPO}/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    
    if [ -z "$VERSION" ]; then
        echo -e "${RED}Error: Could not fetch latest version from GitHub.${NC}"
        exit 1
    fi

    echo "Downloading Mig $VERSION for $ARCH..."
    URL="https://github.com/${OWNER}/${REPO}/releases/download/${VERSION}/mig"
    
    sudo curl -L "$URL" -o /usr/local/bin/mig
    sudo chmod +x /usr/local/bin/mig
    
    echo -e "${GREEN}Mig installed successfully to /usr/local/bin/mig!${NC}"
}

# Main logic
if [ "$PM" != "manual" ]; then
    read -p "Would you like to install via your package manager ($PM)? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        install_via_repo || {
            echo -e "${RED}Repository setup failed. Falling back to manual installation...${NC}"
            install_manual
        }
    else
        install_manual
    fi
else
    install_manual
fi

mig --help
