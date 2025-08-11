#!/bin/bash

# Lash Release Script
# Creates a GitHub release with all built packages using GoReleaser and GitHub CLI

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}🚀 Lash Release Script${NC}"
echo "================================"

# Check if gh CLI is installed
if ! command -v gh &> /dev/null; then
    echo -e "${RED}❌ GitHub CLI (gh) is not installed${NC}"
    echo "Install it with: brew install gh"
    exit 1
fi

# Check if user is authenticated
if ! gh auth status &> /dev/null; then
    echo -e "${RED}❌ Not authenticated with GitHub CLI${NC}"
    echo "Run: gh auth login"
    exit 1
fi

# Check if GoReleaser is installed
if ! command -v goreleaser &> /dev/null; then
    echo -e "${RED}❌ GoReleaser is not installed${NC}"
    echo "Install it with: brew install goreleaser"
    exit 1
fi

# Get current version or prompt for new one
CURRENT_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
echo -e "${YELLOW}Current tag: ${CURRENT_TAG}${NC}"

# Parse version and increment patch
if [[ $CURRENT_TAG =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
    MAJOR=${BASH_REMATCH[1]}
    MINOR=${BASH_REMATCH[2]}
    PATCH=${BASH_REMATCH[3]}
    NEW_PATCH=$((PATCH + 1))
    SUGGESTED_VERSION="v${MAJOR}.${MINOR}.${NEW_PATCH}"
else
    SUGGESTED_VERSION="v0.4.6"
fi

echo -e "${YELLOW}Suggested version: ${SUGGESTED_VERSION}${NC}"
read -p "Enter version to release (default: ${SUGGESTED_VERSION}): " VERSION
VERSION=${VERSION:-$SUGGESTED_VERSION}

# Validate version format
if [[ ! $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo -e "${RED}❌ Invalid version format. Use vX.Y.Z${NC}"
    exit 1
fi

echo -e "${GREEN}📋 Release Summary${NC}"
echo "Version: ${VERSION}"
echo "Repository: lacymorrow/lash"
echo "Homebrew Tap: lacymorrow/homebrew-tap"
echo ""

read -p "Continue with release? (y/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Release cancelled."
    exit 0
fi

echo -e "${GREEN}🔄 Starting release process...${NC}"

# Clean up any previous builds
echo "Cleaning previous builds..."
rm -rf dist/

# Check git status
if [[ -n $(git status --porcelain) ]]; then
    echo -e "${YELLOW}⚠️  Working directory is not clean${NC}"
    git status --short
    read -p "Commit changes first? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        git add .
        git commit -m "chore: prepare for release ${VERSION}"
    else
        echo -e "${RED}❌ Please commit your changes first${NC}"
        exit 1
    fi
fi

# Create and push tag
echo -e "${GREEN}🏷️  Creating tag ${VERSION}...${NC}"
git tag ${VERSION}
git push origin ${VERSION}

echo -e "${GREEN}📦 Building release with GoReleaser...${NC}"

# Set required environment variables for release
export GITHUB_TOKEN=$(gh auth token)

# Run GoReleaser
if goreleaser release --clean; then
    echo -e "${GREEN}✅ Release ${VERSION} created successfully!${NC}"
    echo ""
    echo -e "${GREEN}🔗 Links:${NC}"
    echo "Release: https://github.com/lacymorrow/lash/releases/tag/${VERSION}"
    echo "Ubuntu installation:"
    echo "  wget https://github.com/lacymorrow/lash/releases/download/${VERSION}/lash_Linux_x86_64.deb"
    echo "  sudo dpkg -i lash_Linux_x86_64.deb"
    echo ""
    echo "macOS installation:"
    echo "  brew tap lacymorrow/tap"
    echo "  brew install lash"
else
    echo -e "${RED}❌ GoReleaser failed${NC}"
    echo "Cleaning up tag..."
    git tag -d ${VERSION}
    git push origin :refs/tags/${VERSION}
    exit 1
fi

echo -e "${GREEN}🎉 Release complete!${NC}"
