#!/bin/bash
#
# ssmsearch Installer
# Usage: curl -fsSL https://raw.githubusercontent.com/eugenetaranov/ssmsearch/main/install.sh | bash
#

set -e

GITHUB_REPO="eugenetaranov/ssmsearch"
BINARY_NAME="ssmsearch"
INSTALL_DIR="/usr/local/bin"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

info() { echo -e "${BLUE}==>${NC} $1"; }
success() { echo -e "${GREEN}==>${NC} $1"; }
warn() { echo -e "${YELLOW}Warning:${NC} $1"; }
error() { echo -e "${RED}Error:${NC} $1"; exit 1; }

detect_os() {
    case "$(uname -s)" in
        Darwin*)    echo "darwin" ;;
        Linux*)     echo "linux" ;;
        *)          error "Unsupported OS: $(uname -s)" ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)   echo "amd64" ;;
        aarch64|arm64)  echo "arm64" ;;
        *)              error "Unsupported architecture: $(uname -m)" ;;
    esac
}

get_latest_release() {
    curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" 2>/dev/null | \
        grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/'
}

main() {
    echo ""
    echo "  ssmsearch installer"
    echo ""

    local os=$(detect_os)
    local arch=$(detect_arch)
    info "Detected: $os/$arch"

    local version=$(get_latest_release)
    if [ -z "$version" ]; then
        error "No releases found. Check https://github.com/${GITHUB_REPO}/releases"
    fi

    local archive="ssmsearch_${version#v}_${os}_${arch}.tar.gz"
    local url="https://github.com/${GITHUB_REPO}/releases/download/${version}/${archive}"

    info "Downloading ${archive}..."

    local tmp_dir=$(mktemp -d)
    trap "rm -rf $tmp_dir" EXIT

    if ! curl -fsSL -o "${tmp_dir}/${archive}" "$url"; then
        error "Failed to download from ${url}"
    fi

    tar -xzf "${tmp_dir}/${archive}" -C "${tmp_dir}"
    chmod +x "${tmp_dir}/${BINARY_NAME}"

    info "Installing to $INSTALL_DIR..."

    if [ -w "$INSTALL_DIR" ]; then
        mv "${tmp_dir}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
    else
        sudo mv "${tmp_dir}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
    fi

    if command -v ssmsearch &> /dev/null; then
        success "ssmsearch ${version} installed!"
        echo ""
        ssmsearch -v
        echo ""
        echo "Usage:"
        echo "  ssmsearch -s term1 term2   Search parameters"
        echo "  ssmsearch -l               List all parameters"
        echo "  ssmsearch --help           Show help"
    else
        warn "Installed to $INSTALL_DIR but not in PATH"
        echo "Add to your shell profile:"
        echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
    fi
}

main "$@"
