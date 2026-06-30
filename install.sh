#!/bin/sh
set -e

BINARY="tce"
INSTALL_DIR="/usr/local/bin"

if [ ! -f "main.go" ]; then
    echo "❌ Run this script from the project root (where main.go is)."
    exit 1
fi

echo "🔧 Building $BINARY..."
go build -o "$BINARY" .

echo "📦 Installing to $INSTALL_DIR/$BINARY..."
if [ -w "$INSTALL_DIR" ]; then
    mv "$BINARY" "$INSTALL_DIR/$BINARY"
else
    sudo mv "$BINARY" "$INSTALL_DIR/$BINARY"
fi

echo "✅ Installed! Run: tce --help"
