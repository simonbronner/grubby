#!/usr/bin/env bash
set -euo pipefail

# Grubby Release Helper
# Tags the repository with the next release version (v1, v2, v3, etc.) and pushes to origin.

YES=false
REQUESTED_TAG=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    -y|--yes)
      YES=true
      shift
      ;;
    -h|--help)
      echo "Usage: ./scripts/release.sh [OPTIONS] [TAG]"
      echo ""
      echo "Options:"
      echo "  -y, --yes     Skip interactive confirmation prompt"
      echo "  -h, --help    Show this help message"
      echo ""
      echo "Examples:"
      echo "  ./scripts/release.sh         # Auto-detects and increments next tag (v1 -> v2)"
      echo "  ./scripts/release.sh v3      # Releases specific tag v3"
      echo "  ./scripts/release.sh -y v3   # Releases v3 non-interactively"
      exit 0
      ;;
    *)
      if [ -z "${REQUESTED_TAG}" ]; then
        REQUESTED_TAG="$1"
      else
        echo "Error: Unexpected argument '$1'" >&2
        exit 1
      fi
      shift
      ;;
  esac
done

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${PROJECT_ROOT}"

# Fetch remote tags
echo "--> Fetching latest tags from origin..."
git fetch --tags origin 2>/dev/null || true

# Check working tree cleanliness
if ! git diff-index --quiet HEAD --; then
  echo "Error: Working directory has uncommitted changes. Please commit or stash before releasing." >&2
  exit 1
fi

# Detect current branch
CURRENT_BRANCH="$(git rev-parse --abbrev-ref HEAD)"
if [ "${CURRENT_BRANCH}" != "main" ] && [ "${YES}" != "true" ]; then
  echo "Warning: You are releasing from branch '${CURRENT_BRANCH}', not 'main'."
  read -r -p "Do you want to continue? [y/N] " response
  if [[ ! "${response}" =~ ^([yY][eE][sS]|[yY])$ ]]; then
    echo "Release aborted."
    exit 0
  fi
fi

# Find highest existing tag
LATEST_TAG="$(git tag -l "v*" --sort=-v:refname | head -n1 || true)"

if [ -n "${REQUESTED_TAG}" ]; then
  # Ensure requested tag starts with 'v'
  if [[ "${REQUESTED_TAG}" != v* ]]; then
    NEXT_TAG="v${REQUESTED_TAG}"
  else
    NEXT_TAG="${REQUESTED_TAG}"
  fi
else
  if [ -z "${LATEST_TAG}" ]; then
    NEXT_TAG="v1"
  elif [[ "${LATEST_TAG}" =~ ^v([0-9]+)$ ]]; then
    CURRENT_NUM="${BASH_REMATCH[1]}"
    NEXT_NUM=$((CURRENT_NUM + 1))
    NEXT_TAG="v${NEXT_NUM}"
  elif [[ "${LATEST_TAG}" =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
    # Semver: increment patch by default
    MAJOR="${BASH_REMATCH[1]}"
    MINOR="${BASH_REMATCH[2]}"
    PATCH="${BASH_REMATCH[3]}"
    NEXT_PATCH=$((PATCH + 1))
    NEXT_TAG="v${MAJOR}.${MINOR}.${NEXT_PATCH}"
  else
    NEXT_TAG="v1"
  fi
fi

# Check if tag already exists
if git rev-parse "${NEXT_TAG}" >/dev/null 2>&1; then
  echo "Error: Git tag '${NEXT_TAG}' already exists locally or remotely." >&2
  exit 1
fi

echo "=================================================="
echo " Grubby Release Automation"
echo " Current Branch: ${CURRENT_BRANCH}"
echo " Latest Tag:     ${LATEST_TAG:-none}"
echo " Next Tag:       ${NEXT_TAG}"
echo "=================================================="

if [ "${YES}" != "true" ]; then
  read -r -p "Create annotated tag '${NEXT_TAG}' and push to origin? [y/N] " confirm
  if [[ ! "${confirm}" =~ ^([yY][eE][sS]|[yY])$ ]]; then
    echo "Release aborted."
    exit 0
  fi
fi

echo "--> Creating annotated tag '${NEXT_TAG}'..."
git tag -a "${NEXT_TAG}" -m "Release ${NEXT_TAG}"

echo "--> Pushing tag '${NEXT_TAG}' to origin..."
git push origin "${NEXT_TAG}"

echo ""
echo "=================================================="
echo " Tag '${NEXT_TAG}' pushed successfully!"
echo " GitHub Actions release workflow has been triggered."
echo ""
echo " Monitor build progress at:"
echo "   https://github.com/simonbronner/grubby/actions"
echo ""
echo " GitHub Release will be available at:"
echo "   https://github.com/simonbronner/grubby/releases/tag/${NEXT_TAG}"
echo ""
echo " Once completed, users can install / upgrade via Homebrew:"
echo "   brew update"
echo "   brew install simonbronner/tap/grubby"
echo "   # or: brew upgrade grubby"
echo "=================================================="
