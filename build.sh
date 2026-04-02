#!/bin/bash
set -e

VERSION="${VERSION:-0.1.0}"
BUILD_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$BUILD_DIR"

echo "=== Building CloudPass v$VERSION ==="

echo ">>> Cleaning up..."
rm -rf ui/build
rm -rf api/internal/web/build

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

echo ">>> Creating release archives..."

# Create release directory structure for each platform
create_release() {
    local target=$1
    local os=$2
    local arch=$3
    local ext=""
    [ "$os" = "windows" ] && ext=".exe"

    local release_dir="release-$target"
    mkdir -p "$release_dir/scripts"

    # Copy binary
    cp "cloudpass-$os-$arch$ext" "$release_dir/cloudpass$ext"

    # Copy config
    cp config.yaml "$release_dir/"

    # Copy README
    cp README.md "$release_dir/"

    # Copy scripts (only platform-specific)
    if [ "$os" = "windows" ]; then
        cp scripts/install.ps1 "$release_dir/scripts/"
        cp scripts/uninstall.ps1 "$release_dir/scripts/"
    else
        cp scripts/install.sh "$release_dir/scripts/"
        cp scripts/uninstall.sh "$release_dir/scripts/"
        chmod +x "$release_dir/scripts/"*.sh
    fi
}

# Create releases for each platform
create_release "linux-arm64" linux arm64
create_release "linux-amd64" linux amd64
create_release "darwin-amd64" darwin amd64
create_release "darwin-arm64" darwin arm64
create_release "windows-amd64" windows amd64

# Create archives
echo ">>> Creating tarballs..."
tar -czvf "cloudpass-$VERSION-linux-arm64.tar.gz" -C release-linux-arm64 .
tar -czvf "cloudpass-$VERSION-linux-amd64.tar.gz" -C release-linux-amd64 .
tar -czvf "cloudpass-$VERSION-darwin-amd64.tar.gz" -C release-darwin-amd64 .
tar -czvf "cloudpass-$VERSION-darwin-arm64.tar.gz" -C release-darwin-arm64 .

echo ">>> Creating zip..."
cd release-windows-amd64
zip -q -r "../cloudpass-$VERSION-windows-amd64.zip" .
cd ..

# Cleanup release directories
rm -rf release-linux-arm64 release-linux-amd64 release-darwin-amd64 release-darwin-arm64 release-windows-amd64

# Cleanup individual binaries
rm -f cloudpass-linux-*
rm -f cloudpass-darwin-*
rm -f cloudpass-windows-*

echo ">>> Setup local binary..."
cp cloudpass-linux-amd64 cloudpass
chmod +x cloudpass

echo "=== Build Complete ==="
echo "Archives created:"
ls -la cloudpass-*.tar.gz cloudpass-*.zip
