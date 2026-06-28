#!/bin/bash
set -euo pipefail

# sogark release helper
# Usage: ./scripts/release.sh [--dry-run] [--minor|--major|--patch]

DRY_RUN=false
BUMP=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --dry-run) DRY_RUN=true; shift ;;
        --major)   BUMP="major"; shift ;;
        --minor)   BUMP="minor"; shift ;;
        --patch)   BUMP="patch"; shift ;;
        *) echo "Unknown flag: $1"; exit 1 ;;
    esac
done

# Ensure we're on main
BRANCH=$(git branch --show-current)
if [ "$BRANCH" != "main" ]; then
    echo "[!] Not on main branch (current: $BRANCH). Switch to main first."
    exit 1
fi

# Ensure working tree is clean
if [ -n "$(git status --porcelain)" ]; then
    echo "[!] Working tree is dirty. Commit or stash changes first."
    exit 1
fi

# Check svu is available
if ! command -v svu &>/dev/null; then
    echo "[!] svu not found. Install: go install github.com/caarlos0/svu/v3@latest"
    exit 1
fi

# Determine next version
if [ -n "$BUMP" ]; then
    NEXT=$(svu next --"$BUMP" 2>/dev/null)
else
    NEXT=$(svu next 2>/dev/null)
fi

if [ -z "$NEXT" ]; then
    echo "[!] svu could not determine next version. Is the repo tagged?"
    exit 1
fi

CURRENT=$(svu current 2>/dev/null || echo "none")
echo "Current: $CURRENT"
echo "Next:    $NEXT"
echo ""

# Run tests
echo "[*] Running tests..."
if ! go test ./... 2>&1; then
    echo "[!] Tests failed. Fix before releasing."
    exit 1
fi
echo "[✓] Tests pass"
echo ""

if $DRY_RUN; then
    echo "[i] Dry run — would run:"
    echo "    git tag $NEXT"
    echo "    git push origin $NEXT"
    exit 0
fi

# Confirm
read -r -p "Tag and push $NEXT? [y/N] " answer
if [[ ! "$answer" =~ ^[Yy] ]]; then
    echo "Cancelled."
    exit 0
fi

# Tag and push
git tag "$NEXT"
git push origin "$NEXT"

echo ""
echo "[✓] Tag $NEXT pushed. CI will build and publish the release."
echo "    Watch: https://github.com/$(git remote get-url origin | sed 's/.*github.com[:/]\(.*\)\.git/\1/')/actions"