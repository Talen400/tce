#!/bin/sh
set -e

BINARY="tce"
INSTALL_DIR="/usr/local/bin"
TARGET="$INSTALL_DIR/$BINARY"

if [ ! -f "$TARGET" ]; then
    echo "❌ $BINARY not found in $INSTALL_DIR"
    exit 1
fi

echo "🗑️  Removing $TARGET..."
if [ -w "$INSTALL_DIR" ]; then
    rm "$TARGET"
else
    sudo rm "$TARGET"
fi

echo "✅ Uninstalled!"
