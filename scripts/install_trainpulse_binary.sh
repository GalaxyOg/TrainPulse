#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <version-tag> [repo]"
  echo "example: $0 v0.2.1 GalaxyOg/TrainPulse"
  exit 1
fi

VERSION="$1"
REPO="${2:-GalaxyOg/TrainPulse}"
BIN_NAME="trainpulse"
INSTALL_DIR="${TRAINPULSE_PREFIX:-$HOME/.local/bin}"

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) GOARCH="amd64" ;;
  aarch64|arm64) GOARCH="arm64" ;;
  *)
    echo "unsupported arch: $ARCH"
    exit 1
    ;;
esac

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
if [[ "$OS" != "linux" ]]; then
  echo "this installer currently supports linux only"
  exit 1
fi

ASSET="${BIN_NAME}_${VERSION#v}_${OS}_${GOARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

echo "downloading: $URL"
curl -fL "$URL" -o "$TMP_DIR/$ASSET"

tar -xzf "$TMP_DIR/$ASSET" -C "$TMP_DIR"
mkdir -p "$INSTALL_DIR"

FOUND_BIN="$(find "$TMP_DIR" -type f -name "$BIN_NAME" | head -n 1)"
if [[ -z "$FOUND_BIN" ]]; then
  echo "binary not found in archive"
  exit 1
fi

install -m 0755 "$FOUND_BIN" "$INSTALL_DIR/$BIN_NAME"

echo "installed: $INSTALL_DIR/$BIN_NAME"
if ! command -v "$BIN_NAME" >/dev/null 2>&1; then
  echo "warning: $INSTALL_DIR is not in PATH"
fi

"$INSTALL_DIR/$BIN_NAME" version || true
