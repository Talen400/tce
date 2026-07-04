#!/usr/bin/env bash
set -euo pipefail

REPO="talen400/tce"
VERSION="${1:-latest}"
BINDIR="${BINDIR:-$HOME/.local/bin}"

# ---- Detect OS & Arch ----
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
  x86_64 | amd64) ARCH="amd64" ;;
  aarch64 | arm64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

case "$OS" in
  linux | darwin) ;;
  mingw* | msys* | cygwin*) OS="windows" ;;
  *)
    echo "Unsupported OS: $OS"
    exit 1
    ;;
esac

# ---- Resolve version ----
if [ "$VERSION" = "latest" ]; then
  VERSION="$(curl -sSfL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)"
  echo "Latest release: $VERSION"
fi

# ---- Download ----
BINARY="tce_${OS}_${ARCH}"
[ "$OS" = "windows" ] && BINARY="${BINARY}.exe"

URL="https://github.com/$REPO/releases/download/$VERSION/$BINARY"
echo "Downloading $URL ..."

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

curl -sSfL "$URL" -o "$TMPDIR/tce"
chmod +x "$TMPDIR/tce"

# ---- Install ----
mkdir -p "$BINDIR"
cp "$TMPDIR/tce" "$BINDIR/tce"

echo "Installed tce $VERSION to $BINDIR/tce"
echo ""
echo "Make sure $BINDIR is in your PATH:"
echo "  export PATH=\"\$PATH:$BINDIR\""
