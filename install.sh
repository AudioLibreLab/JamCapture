#!/usr/bin/env bash
# JamCapture installer — downloads the latest release binary and installs it.
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/AudioLibreLab/JamCapture/main/install.sh | bash
#   INSTALL_DIR=~/.local/bin bash install.sh   # install without sudo
set -euo pipefail

REPO="AudioLibreLab/JamCapture"
BIN="jamcapture"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Detect OS
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  linux|darwin) ;;
  *) echo "Unsupported OS: $OS. Download manually from https://github.com/${REPO}/releases"; exit 1 ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)        ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

ASSET="${BIN}-${OS}-${ARCH}"

# Resolve latest release tag
VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' | head -1 | cut -d '"' -f4)"
[ -z "$VERSION" ] && { echo "Could not determine latest version"; exit 1; }

URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"

echo "Installing ${BIN} ${VERSION} (${OS}/${ARCH})…"

TMP="$(mktemp)"
trap 'rm -f "$TMP"' EXIT

curl -fsSL -o "$TMP" "$URL"
chmod +x "$TMP"

if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP" "${INSTALL_DIR}/${BIN}"
else
  echo "Installing to ${INSTALL_DIR} (sudo required)…"
  sudo mv "$TMP" "${INSTALL_DIR}/${BIN}"
fi

echo ""
echo "${BIN} ${VERSION} installed to ${INSTALL_DIR}/${BIN}"
echo ""
echo "Quick start:"
echo "  jamcapture serve        # auto-detects sources and starts the web server"
echo "  jamcapture config init  # re-generate config from detected sources"
echo "  jamcapture sources      # list available audio sources"
