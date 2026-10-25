#!/usr/bin/env bash
set -euo pipefail

# Grubby Homebrew Tap Sync Helper
# Updates Formula/grubby.rb in simonbronner/homebrew-tap

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TAG="${1:-$(git describe --tags --always 2>/dev/null || echo "")}"

if [ -z "${TAG}" ] || [ "${TAG}" = "dev" ]; then
  # Try finding latest git tag
  TAG="$(git tag -l "v*" --sort=-v:refname | head -n1 || true)"
fi

if [ -z "${TAG}" ]; then
  echo "Error: No release tag found. Run ./scripts/build-all.sh <tag> or specify a tag: ./scripts/update-tap.sh v1" >&2
  exit 1
fi

echo "=================================================="
echo " Syncing Homebrew Tap: simonbronner/homebrew-tap"
echo " Target Tag: ${TAG}"
echo "=================================================="

# Check if formula exists locally in Formula/grubby.rb or dist/grubby.rb
FORMULA_SOURCE=""
if [ -f "${PROJECT_ROOT}/dist/grubby.rb" ]; then
  FORMULA_SOURCE="${PROJECT_ROOT}/dist/grubby.rb"
elif [ -f "${PROJECT_ROOT}/Formula/grubby.rb" ]; then
  FORMULA_SOURCE="${PROJECT_ROOT}/Formula/grubby.rb"
else
  echo "--> Formula not found locally. Building release artifacts for ${TAG}..."
  "${PROJECT_ROOT}/scripts/build-all.sh" "${TAG}"
  FORMULA_SOURCE="${PROJECT_ROOT}/dist/grubby.rb"
fi

TAP_DIR=$(mktemp -d)
trap 'rm -rf "${TAP_DIR}"' EXIT

echo "--> Cloning simonbronner/homebrew-tap..."
git clone https://github.com/simonbronner/homebrew-tap.git "${TAP_DIR}"

mkdir -p "${TAP_DIR}/Formula"
cp "${FORMULA_SOURCE}" "${TAP_DIR}/Formula/grubby.rb"

cd "${TAP_DIR}"
git add Formula/grubby.rb

if git diff-index --quiet HEAD --; then
  echo "Homebrew tap is already up to date for ${TAG}."
else
  git commit -m "chore(release): bump grubby to ${TAG}"
  echo "--> Pushing updated formula to origin/main..."
  git push origin main
  echo "✓ simonbronner/homebrew-tap updated successfully to ${TAG}!"
fi

echo "=================================================="
echo " You can test the formula locally with:"
echo "   brew update"
echo "   brew info simonbronner/tap/grubby"
echo "=================================================="
