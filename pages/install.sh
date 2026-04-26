#!/bin/sh
set -e

REPO="wow-look-at-my/wow-cli"
INSTALL_DIR="${HOME}/.local/bin"
BINARY="wow"

OS="$(uname -s)"
case "${OS}" in
  Linux)  OS="linux" ;;
  Darwin) OS="darwin" ;;
  *)
    printf 'Unsupported OS: %s\n' "${OS}" >&2
    exit 1
    ;;
esac

ARCH="$(uname -m)"
case "${ARCH}" in
  x86_64)        ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    printf 'Unsupported architecture: %s\n' "${ARCH}" >&2
    exit 1
    ;;
esac

ASSET="${BINARY}_${OS}_${ARCH}"
URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"

mkdir -p "${INSTALL_DIR}"

TMPFILE="$(mktemp)"
trap 'rm -f "${TMPFILE}"' EXIT

printf 'Downloading %s...\n' "${URL}"
curl -fsSL -o "${TMPFILE}" "${URL}"
chmod +x "${TMPFILE}"
mv "${TMPFILE}" "${INSTALL_DIR}/${BINARY}"
trap - EXIT

printf 'Installed %s to %s/%s\n' "${BINARY}" "${INSTALL_DIR}" "${BINARY}"
