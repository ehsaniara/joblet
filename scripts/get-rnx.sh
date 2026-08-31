#!/bin/bash

# Resolve bin/rnx: $RNX_BINARY, else a ../joblet-rnx checkout (built), else the
# joblet-rnx GitHub release for this host. rnx is consumed here, never built.

set -e

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEST="$ROOT/bin/rnx"
RNX_REPO="${RNX_REPO:-$ROOT/../joblet-rnx}"
RNX_RELEASE_URL="https://github.com/ehsaniara/joblet-rnx/releases"

mkdir -p "$ROOT/bin"

if [ -n "$RNX_BINARY" ] && [ -x "$RNX_BINARY" ] && [ "$RNX_BINARY" != "$DEST" ]; then
    cp "$RNX_BINARY" "$DEST"
    echo "✅ rnx from \$RNX_BINARY: $RNX_BINARY"
    exit 0
fi

if [ -f "$RNX_REPO/go.mod" ]; then
    echo "Building rnx from sibling checkout: $RNX_REPO"
    make -C "$RNX_REPO" build >/dev/null
    cp "$RNX_REPO/bin/rnx" "$DEST"
    echo "✅ rnx built from $RNX_REPO ($("$DEST" --version 2>/dev/null | head -1))"
    exit 0
fi

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$(uname -m)" in
    x86_64) ARCH=amd64 ;;
    aarch64|arm64) ARCH=arm64 ;;
    *) echo "❌ unsupported architecture: $(uname -m)"; exit 1 ;;
esac

echo "Downloading rnx from $RNX_RELEASE_URL/latest ($OS-$ARCH)..."
TAG=$(curl -fsSL "$RNX_RELEASE_URL/latest" -o /dev/null -w '%{url_effective}' | grep -oE 'v[0-9][^/]*$')
[ -n "$TAG" ] || { echo "❌ could not resolve latest joblet-rnx release"; exit 1; }
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
curl -fsSL "$RNX_RELEASE_URL/download/$TAG/rnx-$TAG-$OS-$ARCH.tar.gz" | tar -xz -C "$TMP"
install -m 0755 "$TMP/rnx-$OS-$ARCH" "$DEST"
echo "✅ rnx $TAG downloaded to $DEST"
