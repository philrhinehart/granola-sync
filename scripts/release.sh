#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RESET='\033[0m'

# Check for gh CLI
if ! command -v gh &> /dev/null; then
    echo -e "${RED}Error: gh CLI is not installed${RESET}"
    echo "Install with: brew install gh"
    exit 1
fi

# Check gh authentication
if ! gh auth status &> /dev/null; then
    echo -e "${RED}Error: gh CLI is not authenticated${RESET}"
    echo "Run: gh auth login"
    exit 1
fi

# Fetch latest from origin
echo -e "${YELLOW}Fetching latest tags from origin...${RESET}"
git fetch origin --tags

# Get the latest tag
LATEST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
echo "Latest tag: $LATEST_TAG"

# Parse version components
if [[ $LATEST_TAG =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
    MAJOR="${BASH_REMATCH[1]}"
    MINOR="${BASH_REMATCH[2]}"
    PATCH="${BASH_REMATCH[3]}"
else
    echo -e "${RED}Error: Could not parse version from tag: $LATEST_TAG${RESET}"
    exit 1
fi

# Calculate next patch version
NEXT_PATCH=$((PATCH + 1))
NEXT_VERSION="v${MAJOR}.${MINOR}.${NEXT_PATCH}"

echo -e "${GREEN}Next version: $NEXT_VERSION${RESET}"

# Prompt for confirmation or custom version
read -p "Press Enter to use $NEXT_VERSION, or type a custom version (vX.Y.Z): " CUSTOM_VERSION

if [[ -n "$CUSTOM_VERSION" ]]; then
    # Validate custom version format
    if [[ ! $CUSTOM_VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        echo -e "${RED}Error: Invalid version format. Use vX.Y.Z (e.g., v1.2.3)${RESET}"
        exit 1
    fi
    NEXT_VERSION="$CUSTOM_VERSION"
fi

# Parse the new version to validate single bump
if [[ $NEXT_VERSION =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
    NEW_MAJOR="${BASH_REMATCH[1]}"
    NEW_MINOR="${BASH_REMATCH[2]}"
    NEW_PATCH="${BASH_REMATCH[3]}"
else
    echo -e "${RED}Error: Invalid version format${RESET}"
    exit 1
fi

# Check that only one component changed and it's a +1 bump
MAJOR_DIFF=$((NEW_MAJOR - MAJOR))
MINOR_DIFF=$((NEW_MINOR - MINOR))
PATCH_DIFF=$((NEW_PATCH - PATCH))

if [[ $MAJOR_DIFF -eq 1 && $NEW_MINOR -eq 0 && $NEW_PATCH -eq 0 ]]; then
    echo "Major version bump detected"
elif [[ $MAJOR_DIFF -eq 0 && $MINOR_DIFF -eq 1 && $NEW_PATCH -eq 0 ]]; then
    echo "Minor version bump detected"
elif [[ $MAJOR_DIFF -eq 0 && $MINOR_DIFF -eq 0 && $PATCH_DIFF -eq 1 ]]; then
    echo "Patch version bump detected"
else
    echo -e "${YELLOW}Warning: Version jump is not a single increment${RESET}"
    echo "From: $LATEST_TAG -> To: $NEXT_VERSION"
    read -p "Continue anyway? (y/N): " CONFIRM
    if [[ "$CONFIRM" != "y" && "$CONFIRM" != "Y" ]]; then
        echo "Aborted."
        exit 1
    fi
fi

# Create the release
echo -e "${YELLOW}Creating release $NEXT_VERSION...${RESET}"
gh release create "$NEXT_VERSION" \
    --title "$NEXT_VERSION" \
    --generate-notes

echo -e "${GREEN}✓ Release $NEXT_VERSION created successfully!${RESET}"
