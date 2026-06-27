#!/bin/bash
set -euo pipefail

# sogark installer for macOS/Linux
# Usage: curl -fsSL https://codeberg.org/lotti/sogark/releases/download/<version>/install.sh | bash
# Override version: VERSION=v1.2.0 curl -fsSL ... | bash

UPDATE_REPO="__UPDATE_REPO__"
VERSION="${VERSION:-latest}"
INSTALL_DIR="${HOME}/.sogark/bin"

# Detect OS and architecture
detect_platform() {
    local os arch
    os="$(uname -s | tr '[:upper:]' '[:lower:]')"
    arch="$(uname -m)"

    case "$os" in
        darwin) os="darwin" ;;
        linux)  os="linux" ;;
        *)      echo "[!] Sistema operativo non supportato: $os" >&2; exit 1 ;;
    esac

    case "$arch" in
        x86_64|amd64)  arch="amd64" ;;
        arm64|aarch64) arch="arm64" ;;
        *)             echo "[!] Architettura non supportata: $arch" >&2; exit 1 ;;
    esac

    echo "${os}-${arch}"
}

main() {
    local platform binary_name base_url download_url

    echo "[*] Installazione sogark..."
    echo ""

    platform="$(detect_platform)"
    binary_name="sogark-${platform}"
    base_url="https://codeberg.org/${UPDATE_REPO}/releases/download"

    if [ "$VERSION" = "latest" ]; then
        # Fetch latest release tag from Codeberg API
        local api_url="https://codeberg.org/api/v1/repos/${UPDATE_REPO}/releases/latest"
        VERSION="$(curl -fsSL "$api_url" 2>/dev/null | grep -o '"tag_name":"[^"]*' | cut -d'"' -f4 || echo "")"
        if [ -z "$VERSION" ]; then
            echo "[!] Impossibile determinare l'ultima versione. Specificare VERSION= manualmente." >&2
            exit 1
        fi
    fi

    download_url="${base_url}/${VERSION}/${binary_name}"

    echo "[*] Versione: ${VERSION}"
    echo "[*] Piattaforma: ${platform}"
    echo "[*] Download: ${download_url}"
    echo ""

    # Create install directory
    mkdir -p "${INSTALL_DIR}"

    # Download binary
    local tmp_file="${INSTALL_DIR}/sogark.tmp"
    if ! curl -fSL --progress-bar -o "${tmp_file}" "${download_url}"; then
        echo "[!] Errore download. Verifica URL e connessione." >&2
        rm -f "${tmp_file}"
        exit 1
    fi

    # Install
    chmod 755 "${tmp_file}"
    mv "${tmp_file}" "${INSTALL_DIR}/sogark"

    echo ""
    echo "[✓] sogark installato in ${INSTALL_DIR}/sogark"

    # Configure update_repo so 'sogark update' works out of the box
    if [ -x "${INSTALL_DIR}/sogark" ]; then
        "${INSTALL_DIR}/sogark" config set update_repo "${UPDATE_REPO}" 2>/dev/null || true
    fi

    # Check if already in PATH
    if echo "$PATH" | tr ':' '\n' | grep -qx "${INSTALL_DIR}"; then
        echo "[✓] ${INSTALL_DIR} è già nel PATH"
    else
        echo ""
        echo "[!] Aggiungi ${INSTALL_DIR} al PATH:"
        echo ""

        local shell_name rc_file
        shell_name="$(basename "${SHELL:-/bin/bash}")"
        case "$shell_name" in
            zsh)  rc_file="${HOME}/.zshrc" ;;
            bash) rc_file="${HOME}/.bashrc" ;;
            *)    rc_file="${HOME}/.profile" ;;
        esac

        local path_line="export PATH=\"\${HOME}/.sogark/bin:\${PATH}\""

        if [ -f "$rc_file" ] && grep -qF '.sogark/bin' "$rc_file" 2>/dev/null; then
            echo "    (già presente in ${rc_file})"
        else
            echo "    echo '${path_line}' >> ${rc_file}"
            echo ""
            read -r -p "Aggiungo automaticamente al ${rc_file}? [Y/n] " answer
            answer="${answer:-Y}"
            if [[ "$answer" =~ ^[Yy] ]]; then
                echo "" >> "$rc_file"
                echo "# sogark" >> "$rc_file"
                echo "${path_line}" >> "$rc_file"
                echo "[✓] Aggiunto a ${rc_file}. Esegui: source ${rc_file}"
            fi
        fi
    fi

    echo ""
    echo "Per iniziare: sogark config init"
}

main