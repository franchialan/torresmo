#!/usr/bin/env bash
set -euo pipefail

REPO="torresmo-dev/torresmo"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Detect OS and architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Get latest version
VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/')"

if [ -z "$VERSION" ]; then
    echo "Error: Could not determine latest version"
    exit 1
fi

echo "Installing torresmo v${VERSION} (${OS}/${ARCH})..."

# Download
URL="https://github.com/${REPO}/releases/download/v${VERSION}/torresmo_${VERSION}_${OS}_${ARCH}.tar.gz"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

curl -fsSL "$URL" -o "${TMP_DIR}/torresmo.tar.gz"
tar -xzf "${TMP_DIR}/torresmo.tar.gz" -C "$TMP_DIR"

# Install
if [ -w "$INSTALL_DIR" ]; then
    mv "${TMP_DIR}/torresmo" "${INSTALL_DIR}/torresmo"
else
    sudo mv "${TMP_DIR}/torresmo" "${INSTALL_DIR}/torresmo"
fi

chmod +x "${INSTALL_DIR}/torresmo"

echo "✓ torresmo v${VERSION} installed to ${INSTALL_DIR}/torresmo"
echo "  Run 'torresmo doctor' to verify your setup."
