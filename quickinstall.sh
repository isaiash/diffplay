#!/usr/bin/env bash
# Works both as ./quickinstall.sh and when piped into bash.
set -euo pipefail

fail() {
    printf 'diffplay: %s\n' "$*" >&2
    exit 1
}

as_root() {
    if [ "$(id -u)" = 0 ]; then
        "$@"
    elif command -v sudo >/dev/null 2>&1; then
        sudo "$@"
    else
        fail 'Installing system dependencies requires root or sudo.'
    fi
}

ensure_dependencies() {
    local tool missing=0
    for tool in git curl install mktemp; do
        command -v "$tool" >/dev/null 2>&1 || missing=1
    done
    git --version >/dev/null 2>&1 || missing=1
    if ! command -v sha256sum >/dev/null 2>&1 && ! command -v shasum >/dev/null 2>&1; then
        missing=1
    fi
    if [ "$missing" = 0 ]; then return; fi
    printf 'Installing missing system dependencies (sudo may be required)…\n'
    if [ "$os" = darwin ]; then
        # Homebrew may be installed but absent from a new shell's PATH.
        if ! command -v brew >/dev/null 2>&1; then
            export PATH="$PATH:/opt/homebrew/bin:/usr/local/bin"
        fi
        if ! command -v brew >/dev/null 2>&1; then
            local brew_script
            brew_script=$(curl -fsSL --retry 3 https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh) \
                || fail 'Could not download the Homebrew installer.'
            if [ "$(id -u)" != 0 ]; then
                command -v sudo >/dev/null 2>&1 || fail 'Homebrew setup requires sudo.'
                sudo -v || fail 'Homebrew setup requires sudo access.'
            fi
            NONINTERACTIVE=1 /bin/bash -c "$brew_script" \
                || fail 'Homebrew setup failed; install Homebrew, then rerun this command.'
        fi
        brew install git curl coreutils
    elif command -v apt-get >/dev/null 2>&1; then
        as_root apt-get update
        as_root apt-get install -y git curl ca-certificates coreutils
    elif command -v dnf >/dev/null 2>&1; then
        as_root dnf install -y git curl ca-certificates coreutils
    elif command -v yum >/dev/null 2>&1; then
        as_root yum install -y git curl ca-certificates coreutils
    elif command -v pacman >/dev/null 2>&1; then
        as_root pacman -S --needed --noconfirm git curl ca-certificates coreutils
    elif command -v zypper >/dev/null 2>&1; then
        as_root zypper --non-interactive install git curl ca-certificates coreutils
    elif command -v apk >/dev/null 2>&1; then
        as_root apk add git curl ca-certificates coreutils
    else
        fail 'No supported package manager found. Install Git, curl and coreutils, then rerun.'
    fi
    git --version >/dev/null 2>&1 || fail 'Git installation failed.'
}

download_binary() {
    local asset base expected actual
    asset="diffplay-$os-$arch"
    base=https://github.com/isaiash/diffplay/releases
    curl -fsSL --retry 3 -o "$work_dir/checksum" "$base/latest/download/$asset.sha256" \
        || fail 'Could not download the latest release checksum.'
    expected=$(<"$work_dir/checksum")
    expected=${expected%% *}
    [[ "$expected" =~ ^[0-9a-f]{64}$ ]] || fail 'Invalid release checksum metadata.'
    curl -fsSL --retry 3 "$base/latest/download/$asset" -o "$work_dir/diffplay" \
        || fail 'Could not download the latest release binary.'
    if command -v sha256sum >/dev/null 2>&1; then
        actual=$(sha256sum "$work_dir/diffplay")
    else
        actual=$(shasum -a 256 "$work_dir/diffplay")
    fi
    [ "${actual%% *}" = "$expected" ] || fail 'Release download checksum mismatch; rerun if a new release was just published.'
}

main() {
    case "$(uname -s)" in
        Linux) os=linux ;;
        Darwin) os=darwin ;;
        *) fail 'Only macOS and Linux are supported.' ;;
    esac
    case "$(uname -m)" in
        x86_64|amd64) arch=amd64 ;;
        aarch64|arm64) arch=arm64 ;;
        *) fail 'Supported architectures: x86_64 and arm64.' ;;
    esac
    ensure_dependencies

    cache_dir=${TMPDIR:-"$HOME/.cache"}
    mkdir -p "$cache_dir"
    work_dir=$(mktemp -d "$cache_dir/diffplay-install.XXXXXX")
    trap 'rm -rf "$work_dir"' EXIT
    trap 'exit 130' INT
    trap 'exit 143' HUP TERM
    printf 'Installing Diffplay for %s/%s…\n' "$os" "$arch"
    download_binary
    prefix=${PREFIX:-"$HOME/.local"}
    mkdir -p "$prefix/bin"
    install -m 755 "$work_dir/diffplay" "$prefix/bin/diffplay"
    printf 'Installed %s\n' "$prefix/bin/diffplay"
    case ":$PATH:" in
        *":$prefix/bin:"*) ;;
        *) printf 'Add %s/bin to your PATH.\n' "$prefix" ;;
    esac
}

main "$@"
