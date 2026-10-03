#!/usr/bin/env bash

set -euo pipefail

REPO="ernesto27/ai-tools"
BINARY="agent-sandbox"
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
    x86_64) ASSET="agent-sandbox_linux_amd64.tar.gz" ;;
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

# --- Resolve the newest release asset through the GitHub API ---
# The repository releases more than one component, so /releases/latest may well
# point at something other than agent-sandbox.
echo "Resolving latest ${BINARY} release for ${REPO} ..."
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT
"${DOWNLOAD[@]}" "https://api.github.com/repos/${REPO}/releases?per_page=100" > "$TMP_DIR/releases.json"
ASSET_LINE="$(grep -m1 -oE "\"browser_download_url\"[[:space:]]*:[[:space:]]*\"https://[^\"]*/${ASSET}\"" "$TMP_DIR/releases.json" || true)"
DOWNLOAD_URL="$(printf '%s\n' "$ASSET_LINE" | sed -E 's/.*"(https:[^"]+)".*/\1/')"

if [ -z "$DOWNLOAD_URL" ]; then
    echo "error: could not find asset '${ASSET}' in recent releases of ${REPO}" >&2
    exit 1
fi

# --- Download and extract ---
echo "Downloading ${BINARY} from ${DOWNLOAD_URL} ..."
"${DOWNLOAD[@]}" "$DOWNLOAD_URL" > "$TMP_DIR/$ASSET"
"${DOWNLOAD[@]}" "${DOWNLOAD_URL}.sha256" > "$TMP_DIR/$ASSET.sha256"

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
