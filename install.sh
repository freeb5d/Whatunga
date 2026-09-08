#!/bin/bash
# Whatunga installer — run with:
#   bash <(curl -Ls https://raw.githubusercontent.com/freeb5d/Whatunga/main/install.sh)
#
# Installs the latest GitHub Release binary for this machine's OS/arch,
# sets it up as a systemd service, and creates a starter config file.
# Modeled on the familiar 3x-ui-style installer UX (colored output,
# a menu for install/uninstall/update/status) but written from scratch
# for Whatunga's own layout (systemd unit, config path, binary name).

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
PLAIN='\033[0m'

REPO="freeb5d/Whatunga"
BIN_NAME="whatunga"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/whatunga"
CONFIG_FILE="${CONFIG_DIR}/config.yaml"
SERVICE_FILE="/etc/systemd/system/whatunga.service"

log()  { echo -e "${GREEN}[Whatunga]${PLAIN} $1"; }
warn() { echo -e "${YELLOW}[Whatunga]${PLAIN} $1"; }
err()  { echo -e "${RED}[Whatunga]${PLAIN} $1"; }

require_root() {
    if [[ $EUID -ne 0 ]]; then
        err "This script must be run as root. Try: sudo bash <(curl -Ls https://raw.githubusercontent.com/${REPO}/main/install.sh)"
        exit 1
    fi
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *)
            err "Unsupported architecture: $(uname -m)"
            exit 1
            ;;
    esac
}

detect_os() {
    case "$(uname -s)" in
        Linux) OS="linux" ;;
        Darwin) OS="darwin" ;;
        *)
            err "Unsupported OS: $(uname -s). This installer targets Linux servers (systemd)."
            exit 1
            ;;
    esac
}

latest_tag() {
    curl -Ls "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep '"tag_name":' \
        | sed -E 's/.*"tag_name":\s*"([^"]+)".*/\1/'
}

download_and_install() {
    detect_os
    detect_arch

    TAG=$(latest_tag)
    if [[ -z "$TAG" ]]; then
        err "Could not determine the latest release tag from GitHub. Check your internet connection or https://github.com/${REPO}/releases"
        exit 1
    fi
    log "Latest release: ${TAG}"

    ASSET="whatunga_${OS}_${ARCH}.tar.gz"
    URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET}"

    TMP_DIR=$(mktemp -d)
    log "Downloading ${ASSET}..."
    if ! curl -Ls -o "${TMP_DIR}/${ASSET}" "$URL"; then
        err "Download failed: ${URL}"
        rm -rf "$TMP_DIR"
        exit 1
    fi

    tar -xzf "${TMP_DIR}/${ASSET}" -C "$TMP_DIR"

    if [[ ! -f "${TMP_DIR}/${BIN_NAME}" ]]; then
        err "Expected binary '${BIN_NAME}' not found in the downloaded archive."
        rm -rf "$TMP_DIR"
        exit 1
    fi

    install -m 755 "${TMP_DIR}/${BIN_NAME}" "${INSTALL_DIR}/${BIN_NAME}"
    mkdir -p "$CONFIG_DIR"

    if [[ ! -f "$CONFIG_FILE" ]]; then
        cp "${TMP_DIR}/config.example.yaml" "$CONFIG_FILE"
        warn "Created a starter config at ${CONFIG_FILE} — edit it with your real RouterOS device(s) before starting the service."
    else
        warn "Existing config found at ${CONFIG_FILE} — left untouched."
    fi

    rm -rf "$TMP_DIR"
    log "Installed ${BIN_NAME} ${TAG} to ${INSTALL_DIR}/${BIN_NAME}"
}

install_service() {
    cat > "$SERVICE_FILE" <<EOF
[Unit]
Description=Whatunga - RouterOS monitoring
After=network.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/${BIN_NAME} serve -config ${CONFIG_FILE}
WorkingDirectory=${CONFIG_DIR}
Restart=on-failure
RestartSec=5
User=root

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable whatunga.service >/dev/null 2>&1
    log "systemd service installed and enabled (starts automatically on boot)."
}

do_install() {
    require_root
    download_and_install
    install_service

    log "Edit ${CONFIG_FILE} with your RouterOS device(s), then start the service:"
    echo "    systemctl start whatunga"
    echo "    systemctl status whatunga"
    echo
    warn "Default admin panel login is admin / admin — change it immediately from the Account page after first sign-in."
}

do_uninstall() {
    require_root
    systemctl stop whatunga.service 2>/dev/null
    systemctl disable whatunga.service 2>/dev/null
    rm -f "$SERVICE_FILE"
    rm -f "${INSTALL_DIR}/${BIN_NAME}"
    systemctl daemon-reload
    log "Whatunga binary and service removed."
    warn "Config and database left in place at ${CONFIG_DIR} — delete manually if you want a full wipe:"
    echo "    rm -rf ${CONFIG_DIR}"
}

do_update() {
    require_root
    log "Updating Whatunga to the latest release..."
    systemctl stop whatunga.service 2>/dev/null
    download_and_install
    systemctl start whatunga.service
    log "Updated and restarted."
}

do_status() {
    systemctl status whatunga.service --no-pager
}

show_menu() {
    echo ""
    echo "  Whatunga installer"
    echo "  ------------------"
    echo "  1) Install"
    echo "  2) Update to latest release"
    echo "  3) Uninstall"
    echo "  4) Service status"
    echo "  0) Exit"
    echo ""
    read -rp "Choose an option: " choice
    case "$choice" in
        1) do_install ;;
        2) do_update ;;
        3) do_uninstall ;;
        4) do_status ;;
        0) exit 0 ;;
        *) err "Invalid option." ;;
    esac
}

# Allow non-interactive use too: `bash install.sh install|update|uninstall|status`
case "$1" in
    install) do_install ;;
    update) do_update ;;
    uninstall) do_uninstall ;;
    status) do_status ;;
    "") show_menu ;;
    *)
        err "Unknown argument: $1 (expected install|update|uninstall|status)"
        exit 1
        ;;
esac
