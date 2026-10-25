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

# Check if formula can be fetched directly from GitHub Release
FORMULA_SOURCE=""
TEMP_FETCH_DIR=$(mktemp -d)
if command -v gh >/dev/null 2>&1 && gh release view "${TAG}" >/dev/null 2>&1; then
  echo "--> Downloading published formula from GitHub Release ${TAG}..."
  if gh release download "${TAG}" -p "grubby.rb" -D "${TEMP_FETCH_DIR}" --clobber 2>/dev/null; then
    FORMULA_SOURCE="${TEMP_FETCH_DIR}/grubby.rb"
  fi
fi

if [ -z "${FORMULA_SOURCE}" ]; then
  if [ -f "${PROJECT_ROOT}/Formula/grubby.rb" ]; then
    FORMULA_SOURCE="${PROJECT_ROOT}/Formula/grubby.rb"
  elif [ -f "${PROJECT_ROOT}/dist/grubby.rb" ]; then
    FORMULA_SOURCE="${PROJECT_ROOT}/dist/grubby.rb"
  else
    echo "--> Formula not found. Building release artifacts for ${TAG}..."
    "${PROJECT_ROOT}/scripts/build-all.sh" "${TAG}"
    FORMULA_SOURCE="${PROJECT_ROOT}/dist/grubby.rb"
  fi
fi

TAP_DIR=$(mktemp -d)
trap 'rm -rf "${TAP_DIR}" "${TEMP_FETCH_DIR}"' EXIT

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
