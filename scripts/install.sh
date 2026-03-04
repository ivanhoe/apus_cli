#!/usr/bin/env bash
set -euo pipefail

REPO="ivanhoe/apus_cli"
BIN_NAME="apus"
INSTALL_DIR="${APUS_INSTALL_DIR:-/usr/local/bin}"

# ── Detect architecture ──────────────────────────────────────────────────────
ARCH="$(uname -m)"
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"

case "${ARCH}" in
  arm64)  GOARCH="arm64" ;;
  x86_64) GOARCH="amd64" ;;
  *)
    echo "Unsupported architecture: ${ARCH}" >&2
    exit 1
    ;;
esac

if [[ "${OS}" != "darwin" ]]; then
  echo "apus CLI currently supports macOS only." >&2
  exit 1
fi

# ── Fetch latest release tag ─────────────────────────────────────────────────
echo "Fetching latest release..."
LATEST_TAG="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' \
  | sed -E 's/.*"tag_name":\s*"([^"]+)".*/\1/')"

if [[ -z "${LATEST_TAG}" ]]; then
  echo "Could not determine latest release tag." >&2
  exit 1
fi

TARBALL="${BIN_NAME}_${LATEST_TAG}_${OS}_${GOARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${LATEST_TAG}/${TARBALL}"

# ── Download & install ───────────────────────────────────────────────────────
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

echo "Downloading ${BIN_NAME} ${LATEST_TAG} (${OS}/${GOARCH})..."
curl -fsSL "${DOWNLOAD_URL}" -o "${TMP_DIR}/${TARBALL}"

echo "Installing to ${INSTALL_DIR}/${BIN_NAME}..."
tar -xzf "${TMP_DIR}/${TARBALL}" -C "${TMP_DIR}"

if [[ ! -f "${TMP_DIR}/${BIN_NAME}" ]]; then
  echo "Binary not found in tarball — unexpected archive layout." >&2
  exit 1
fi

chmod +x "${TMP_DIR}/${BIN_NAME}"

# Need sudo if install dir isn't writable
if [[ -w "${INSTALL_DIR}" ]]; then
  mv "${TMP_DIR}/${BIN_NAME}" "${INSTALL_DIR}/${BIN_NAME}"
else
  sudo mv "${TMP_DIR}/${BIN_NAME}" "${INSTALL_DIR}/${BIN_NAME}"
fi

echo ""
echo "✓ apus ${LATEST_TAG} installed at ${INSTALL_DIR}/${BIN_NAME}"
echo ""
echo "  apus new MyApp    — create a new project"
echo "  apus init         — add Apus to an existing project"
echo ""
