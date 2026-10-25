#!/usr/bin/env bash
set -euo pipefail

# Determine Version & Tag
TAG="${1:-$(git describe --tags --always 2>/dev/null || echo "dev")}"
VERSION="${TAG#v}" # Strip leading 'v' if present
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo "none")"
BUILD_DATE="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

echo "=================================================="
echo " Building Grubby"
echo " Tag:        ${TAG}"
echo " Version:    ${VERSION}"
echo " Commit:     ${COMMIT}"
echo " Build Date: ${BUILD_DATE}"
echo "=================================================="

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="${PROJECT_ROOT}/dist"

rm -rf "${DIST_DIR}"
mkdir -p "${DIST_DIR}"

TARGETS=(
  "darwin amd64 tar.gz"
  "darwin arm64 tar.gz"
  "linux amd64 tar.gz"
  "linux arm64 tar.gz"
  "windows amd64 zip"
)

LDFLAGS="-s -w -X main.Version=${TAG} -X main.Commit=${COMMIT} -X main.Date=${BUILD_DATE}"

for TARGET in "${TARGETS[@]}"; do
  read -r OS ARCH EXT <<< "${TARGET}"
  echo "--> Compiling for ${OS}/${ARCH}..."

  PKG_NAME="grubby_${TAG}_${OS}_${ARCH}"
  STAGE_DIR="${DIST_DIR}/${PKG_NAME}"
  mkdir -p "${STAGE_DIR}"

  BIN_NAME="grubby"
  if [ "${OS}" = "windows" ]; then
    BIN_NAME="grubby.exe"
  fi

  CGO_ENABLED=0 GOOS="${OS}" GOARCH="${ARCH}" go build \
    -ldflags "${LDFLAGS}" \
    -o "${STAGE_DIR}/${BIN_NAME}" \
    "${PROJECT_ROOT}/cmd/grubby"

  cp "${PROJECT_ROOT}/README.md" "${STAGE_DIR}/"
  cp "${PROJECT_ROOT}/LICENSE" "${STAGE_DIR}/"

  if [ "${EXT}" = "tar.gz" ]; then
    (cd "${STAGE_DIR}" && tar -czf "${DIST_DIR}/${PKG_NAME}.tar.gz" *)
  elif [ "${EXT}" = "zip" ]; then
    (cd "${STAGE_DIR}" && zip -q -r "${DIST_DIR}/${PKG_NAME}.zip" *)
  fi
  rm -rf "${STAGE_DIR}"
done

echo "--> Generating SHA256 checksums..."
cd "${DIST_DIR}"
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum grubby_* > checksums.txt
elif command -v shasum >/dev/null 2>&1; then
  shasum -a 256 grubby_* > checksums.txt
fi
cat checksums.txt
cd "${PROJECT_ROOT}"

# Extract individual sha256 hashes for Homebrew formula
get_sha() {
  local filename="$1"
  grep "${filename}" "${DIST_DIR}/checksums.txt" | awk '{print $1}'
}

DARWIN_ARM64_SHA="$(get_sha "grubby_${TAG}_darwin_arm64.tar.gz")"
DARWIN_AMD64_SHA="$(get_sha "grubby_${TAG}_darwin_amd64.tar.gz")"
LINUX_ARM64_SHA="$(get_sha "grubby_${TAG}_linux_arm64.tar.gz")"
LINUX_AMD64_SHA="$(get_sha "grubby_${TAG}_linux_amd64.tar.gz")"

echo "--> Generating Homebrew Formula..."
cat <<EOF > "${DIST_DIR}/grubby.rb"
class Grubby < Formula
  desc "Terminal cockpit TUI for GitHub repositories, PRs, and workspaces"
  homepage "https://github.com/simonbronner/grubby"
  version "${VERSION}"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/${TAG}/grubby_${TAG}_darwin_arm64.tar.gz"
      sha256 "${DARWIN_ARM64_SHA}"
    else
      url "https://github.com/simonbronner/grubby/releases/download/${TAG}/grubby_${TAG}_darwin_amd64.tar.gz"
      sha256 "${DARWIN_AMD64_SHA}"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/${TAG}/grubby_${TAG}_linux_arm64.tar.gz"
      sha256 "${LINUX_ARM64_SHA}"
    else
      url "https://github.com/simonbronner/grubby/releases/download/${TAG}/grubby_${TAG}_linux_amd64.tar.gz"
      sha256 "${LINUX_AMD64_SHA}"
    end
  end

  def install
    bin.install "grubby"
  end

  test do
    assert_match "grubby", shell_output("#{bin}/grubby -v")
  end
end
EOF

# Also keep Formula/grubby.rb updated in the workspace
mkdir -p "${PROJECT_ROOT}/Formula"
cp "${DIST_DIR}/grubby.rb" "${PROJECT_ROOT}/Formula/grubby.rb"

echo "=================================================="
echo " Build artifacts successfully created in dist/:"
ls -lh "${DIST_DIR}"
echo "=================================================="
