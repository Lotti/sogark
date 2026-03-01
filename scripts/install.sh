#!/bin/bash
set -euo pipefail

# sogark installer for macOS/Linux
# Usage: curl -fsSL <nexus_url>/repository/<repo>/latest/install.sh | bash
# Override version: VERSION=v1.2.0 curl -fsSL ... | bash

NEXUS_URL="__NEXUS_URL__"
NEXUS_REPO="__NEXUS_REPO__"
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
    local platform binary_name download_url

    echo "[*] Installazione sogark..."
    echo ""

    platform="$(detect_platform)"
    binary_name="sogark-${platform}"
    download_url="${NEXUS_URL}/repository/${NEXUS_REPO}/${VERSION}/${binary_name}"

    # Show version info
    if [ "$VERSION" = "latest" ]; then
        local ver
        ver="$(curl -fsSL "${NEXUS_URL}/repository/${NEXUS_REPO}/latest/version.txt" 2>/dev/null || echo "unknown")"
        echo "[*] Versione: ${ver} (latest)"
    else
        echo "[*] Versione: ${VERSION}"
    fi
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

    # Configure nexus_url and nexus_repo if sogark config exists or can be created
    if [ -x "${INSTALL_DIR}/sogark" ]; then
        # Set nexus config so 'sogark update' works out of the box
        "${INSTALL_DIR}/sogark" config set nexus_url "${NEXUS_URL}" 2>/dev/null || true
        "${INSTALL_DIR}/sogark" config set nexus_repo "${NEXUS_REPO}" 2>/dev/null || true
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
