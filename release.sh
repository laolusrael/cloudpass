#!/bin/bash
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

error() { echo -e "${RED}Error: $1${NC}" >&2; exit 1; }
info() { echo -e "${GREEN}$1${NC}"; }
warn() { echo -e "${YELLOW}$1${NC}"; }
usage() { echo -e "${BLUE}$1${NC}"; }

if [ "$1" = "-h" ] || [ "$1" = "--help" ]; then
    usage "Usage: ./release.sh [version]"
    echo ""
    usage "Arguments:"
    echo "  version    Optional tag version (e.g., v1.0.0). If not provided, auto-generated."
    echo ""
    usage "Examples:"
    echo "  ./release.sh              # Auto-generate next version"
    echo "  ./release.sh v1.2.3       # Use specific version"
    echo ""
    exit 0
fi

BRANCH=$(git branch --show-current)
if [ "$BRANCH" != "main" ]; then
    error "Must be on main branch (current: $BRANCH)"
fi

if [ -n "$(git status --porcelain)" ]; then
    error "Uncommitted changes detected. Commit or stash them first."
fi

GIVEN_TAG="$1"

if [ -n "$GIVEN_TAG" ]; then
    if [[ ! "$GIVEN_TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        error "Invalid tag format. Use v0.0.0 (e.g., v1.2.3)"
    fi
    TAG="$GIVEN_TAG"
    VERSION="${TAG#v}"
    info "Using provided tag: $TAG"
else
    LAST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
    
    COMMITS_SINCE_TAG=$(git log "$LAST_TAG"..HEAD --oneline 2>/dev/null | wc -l)
    
    if [ "$COMMITS_SINCE_TAG" -eq 0 ]; then
        info "No changes since last tag ($LAST_TAG). Skipping release."
        exit 0
    fi
    
    warn "Last tag: $LAST_TAG"
    warn "Commits since last tag: $COMMITS_SINCE_TAG"
    
    CURRENT_VER=${LAST_TAG#v}
    MAJOR=$(echo "$CURRENT_VER" | cut -d. -f1)
    MINOR=$(echo "$CURRENT_VER" | cut -d. -f2)
    PATCH=$(echo "$CURRENT_VER" | cut -d. -f3)
    
    COMMIT_MSGS=$(git log "$LAST_TAG"..HEAD --format=%s 2>/dev/null)
    
    if echo "$COMMIT_MSGS" | grep -qE "^feat\!|^BREAKING"; then
        warn "Breaking change detected - bumping major version"
        MAJOR=$((MAJOR + 1)); MINOR=0; PATCH=0
    elif echo "$COMMIT_MSGS" | grep -qE "^feat[^\!]"; then
        warn "New feature detected - bumping minor version"
        MINOR=$((MINOR + 1)); PATCH=0
    elif echo "$COMMIT_MSGS" | grep -qE "^fix"; then
        warn "Bug fix detected - bumping patch version"
        PATCH=$((PATCH + 1))
    else
        warn "Regular changes detected - bumping patch version"
        PATCH=$((PATCH + 1))
    fi
    
    TAG="v$MAJOR.$MINOR.$PATCH"
    VERSION="$MAJOR.$MINOR.$PATCH"
    info "Auto-generated tag: $TAG"
fi

info "Releasing CloudPass $TAG..."

info "Building binaries..."
chmod +x ./build.sh
VERSION="$VERSION" ./build.sh

info "Creating source tarball..."
git archive --format=tar.gz -o "cloudpass-$VERSION-source.tar.gz" HEAD

info "Creating tag $TAG..."
git tag "$TAG"

info "Pushing tag to origin (this will trigger the release workflow)..."
git push origin "$TAG"

info ""
info "=========================================="
info "  Release $TAG triggered successfully!"
info "=========================================="
info ""
info "Check GitHub Actions for build progress:"
info "  https://github.com/laolusrael/cloudpass/actions"
info ""
