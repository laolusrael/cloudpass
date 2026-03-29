#!/bin/bash
set -e

VERSION="${VERSION:-0.1.0}"
BUILD_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$BUILD_DIR"

echo "=== Building CloudPass v$VERSION ==="

echo ">>> Building UI..."
cd ui
npm ci --silent
npm run build
cd ..

echo ">>> Copying UI to embed directory..."
mkdir -p api/internal/web/build
cp -r ui/build/* api/internal/web/build/

echo ">>> Building Go API..."
cd api

echo ">>> Building binaries..."

build() {
    local os=$1
    local arch=$2
    local ext=""
    [ "$os" = "windows" ] && ext=".exe"

    echo "Building for $os/$arch..."
    GOOS=$os GOARCH=$arch CGO_ENABLED=0 go build -ldflags="-s -w" -o "../cloudpass-$os-$arch$ext" ./cmd/server
}

build linux arm64
build linux amd64
build darwin amd64
build darwin arm64
build windows amd64

cd ..

echo ">>> Creating archive..."
tar -czvf "cloudpass-$VERSION-linux-arm64.tar.gz" cloudpass-linux-arm64
tar -czvf "cloudpass-$VERSION-linux-amd64.tar.gz" cloudpass-linux-amd64
tar -czvf "cloudpass-$VERSION-darwin-amd64.tar.gz" cloudpass-darwin-amd64
tar -czvf "cloudpass-$VERSION-darwin-arm64.tar.gz" cloudpass-darwin-arm64
zip -q "cloudpass-$VERSION-windows-amd64.zip" cloudpass-windows-amd64.exe

echo ">>> Setup local binary..."
cp cloudpass-linux-amd64 cloudpass
chmod +x cloudpass

echo "=== Build Complete ==="
echo "Binaries created:"
ls -la cloudpass-*
