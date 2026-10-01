#!/usr/bin/env bash

set -euo pipefail

REPO="ernesto27/ai-tools"
BINARY="factory"
INSTALL_DIR="${INSTALL_DIR:-${HOME}/.local/bin}"

# --- OS check ---
OS="$(uname -s)"
if [ "$OS" != "Linux" ]; then
	echo "error: only Linux is supported at this time (detected: $OS)" >&2
	exit 1
fi

# --- Architecture check ---
ARCH="$(uname -m)"
case "$ARCH" in
	x86_64) ASSET="factory_linux_amd64.tar.gz" ;;
	*)
		echo "error: unsupported architecture '$ARCH' (only x86_64 is available)" >&2
		exit 1
		;;
esac

# --- Downloader ---
if command -v curl >/dev/null 2>&1; then
	DOWNLOAD=(curl -fsSL)
elif command -v wget >/dev/null 2>&1; then
	DOWNLOAD=(wget -qO-)
else
	echo "error: curl or wget is required" >&2
	exit 1
fi

if ! command -v sha256sum >/dev/null 2>&1; then
	echo "error: sha256sum is required" >&2
	exit 1
fi

# --- Download the latest release assets ---
DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"

# --- Download and extract ---
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

echo "Downloading ${BINARY} from ${DOWNLOAD_URL} ..."
"${DOWNLOAD[@]}" "$DOWNLOAD_URL" > "$TMP_DIR/$ASSET"
"${DOWNLOAD[@]}" "${DOWNLOAD_URL}.sha256" > "$TMP_DIR/$ASSET.sha256"

# Compare the digest directly; do not let a downloaded checksum choose file paths.
EXPECTED_SHA="$(awk 'NR == 1 {print $1}' "$TMP_DIR/$ASSET.sha256")"
ACTUAL_SHA="$(sha256sum "$TMP_DIR/$ASSET")"
if [[ ! "$EXPECTED_SHA" =~ ^[[:xdigit:]]{64}$ ]] || [[ "${EXPECTED_SHA,,}" != "${ACTUAL_SHA%% *}" ]]; then
	echo "error: release archive checksum verification failed" >&2
	exit 1
fi
tar -xzf "$TMP_DIR/$ASSET" -C "$TMP_DIR" -- "$BINARY"

# --- Install ---
mkdir -p "$INSTALL_DIR"
install -m 755 "$TMP_DIR/${BINARY}" "$INSTALL_DIR/${BINARY}"

echo
echo "${BINARY} installed to ${INSTALL_DIR}/${BINARY}"

if [[ ":${PATH}:" != *":${INSTALL_DIR}:"* ]]; then
	echo
	echo "Add this to your shell profile:"
	printf 'export PATH=%q:"$PATH"\n' "$INSTALL_DIR"
fi
