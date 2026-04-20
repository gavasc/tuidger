#!/usr/bin/env bash
set -euo pipefail

REPO="gavasc/tuidger"
BINARY="tuidger"
FALLBACK_DIR="$HOME/.local/bin"

# ── Helpers ───────────────────────────────────────────────────────────────────

info()  { printf '\033[1;34m=>\033[0m %s\n' "$*"; }
ok()    { printf '\033[1;32m✓\033[0m  %s\n' "$*"; }
warn()  { printf '\033[1;33m!\033[0m  %s\n' "$*"; }
die()   { printf '\033[1;31m✗\033[0m  %s\n' "$*" >&2; exit 1; }

need() {
    command -v "$1" &>/dev/null || die "Required tool not found: $1 — please install it and retry."
}

# ── Detect architecture ───────────────────────────────────────────────────────

detect_arch() {
    case "$(uname -m)" in
        x86_64)  echo "amd64" ;;
        aarch64) echo "arm64" ;;
        *)       die "Unsupported architecture: $(uname -m)" ;;
    esac
}

# ── Detect distro ─────────────────────────────────────────────────────────────

detect_distro() {
    if [[ -f /etc/os-release ]]; then
        # shellcheck disable=SC1091
        source /etc/os-release
        echo "${ID:-unknown}"
    else
        echo "unknown"
    fi
}

# ── Fetch latest release version from GitHub ─────────────────────────────────

latest_version() {
    need curl
    curl -sSfL "https://api.github.com/repos/$REPO/releases/latest" \
        | grep '"tag_name"' \
        | head -1 \
        | sed 's/.*"tag_name": *"\(.*\)".*/\1/'
}

# ── Install via AUR (Arch) ────────────────────────────────────────────────────

install_arch() {
    if command -v yay &>/dev/null; then
        info "Installing via yay (AUR)…"
        yay -S --noconfirm "$BINARY"
    elif command -v paru &>/dev/null; then
        info "Installing via paru (AUR)…"
        paru -S --noconfirm "$BINARY"
    else
        warn "No AUR helper found (yay/paru). Falling back to binary install."
        warn "To get automatic updates on Arch, install yay: https://github.com/Jguer/yay"
        install_binary
    fi
}

# ── Install via .deb (Debian/Ubuntu/Mint/etc.) ────────────────────────────────

install_deb() {
    need curl
    local arch version deb_url deb_file
    arch=$(detect_arch)
    version=$(latest_version)

    [[ -z "$version" ]] && die "Could not determine latest release version."

    deb_file="${BINARY}_${version}_${arch}.deb"
    deb_url="https://github.com/$REPO/releases/download/$version/$deb_file"

    info "Downloading $deb_file…"
    local tmp
    tmp=$(mktemp -d)
    trap 'rm -rf "$tmp"' EXIT

    curl -sSfL "$deb_url" -o "$tmp/$deb_file" || {
        warn ".deb not found for $version/$arch. Falling back to binary install."
        install_binary
        return
    }

    info "Installing package…"
    sudo dpkg -i "$tmp/$deb_file"
}

# ── Fallback: install raw binary ──────────────────────────────────────────────

install_binary() {
    need curl
    local arch version bin_name bin_url
    arch=$(detect_arch)
    version=$(latest_version)

    [[ -z "$version" ]] && die "Could not determine latest release version."

    bin_name="${BINARY}_linux_${arch}"
    bin_url="https://github.com/$REPO/releases/download/$version/$bin_name"

    info "Downloading binary $bin_name…"
    local tmp
    tmp=$(mktemp -d)
    trap 'rm -rf "$tmp"' EXIT

    curl -sSfL "$bin_url" -o "$tmp/$BINARY" || die "Failed to download binary from $bin_url"
    chmod +x "$tmp/$BINARY"

    mkdir -p "$FALLBACK_DIR"
    mv "$tmp/$BINARY" "$FALLBACK_DIR/$BINARY"
    ok "Installed to $FALLBACK_DIR/$BINARY"

    if [[ ":$PATH:" != *":$FALLBACK_DIR:"* ]]; then
        warn "$FALLBACK_DIR is not in your PATH."
        warn "Add the following to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
        warn "  export PATH=\"\$HOME/.local/bin:\$PATH\""
    fi
}

# ── Main ──────────────────────────────────────────────────────────────────────

main() {
    info "Installing tuidger…"

    local distro
    distro=$(detect_distro)

    case "$distro" in
        arch)
            install_arch
            ;;
        debian|ubuntu|linuxmint|pop|elementary|zorin|kali|raspbian)
            install_deb
            ;;
        *)
            warn "Unrecognised distro: $distro. Attempting binary install."
            install_binary
            ;;
    esac

    if command -v "$BINARY" &>/dev/null; then
        ok "tuidger $(tuidger --version) installed successfully."
        ok "Run: tuidger"
    fi
}

main "$@"
