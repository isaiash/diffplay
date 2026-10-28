#!/usr/bin/env bash
# Works both as ./quickinstall.sh and when piped into bash.
set -euo pipefail

fail() {
    printf 'diffplay: %s\n' "$*" >&2
    exit 1
}

usable_go() {
    local output
    output=$("$1" version 2>/dev/null) || return 1
    [[ "$output" =~ go([0-9]+)\.([0-9]+) ]] || return 1
    (( BASH_REMATCH[1] > 1 || (BASH_REMATCH[1] == 1 && BASH_REMATCH[2] >= 24) ))
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
    for tool in git curl tar gzip install mktemp; do
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
        brew install git curl gnu-tar gzip coreutils
    elif command -v apt-get >/dev/null 2>&1; then
        as_root apt-get update
        as_root apt-get install -y git curl ca-certificates tar gzip coreutils
    elif command -v dnf >/dev/null 2>&1; then
        as_root dnf install -y git curl ca-certificates tar gzip coreutils
    elif command -v yum >/dev/null 2>&1; then
        as_root yum install -y git curl ca-certificates tar gzip coreutils
    elif command -v pacman >/dev/null 2>&1; then
        as_root pacman -S --needed --noconfirm git curl ca-certificates tar gzip coreutils
    elif command -v zypper >/dev/null 2>&1; then
        as_root zypper --non-interactive install git curl ca-certificates tar gzip coreutils
    elif command -v apk >/dev/null 2>&1; then
        as_root apk add git curl ca-certificates tar gzip coreutils
    else
        fail 'No supported package manager found. Install Git, curl, tar, gzip and coreutils, then rerun.'
    fi
    git --version >/dev/null 2>&1 || fail 'Git installation failed.'
}

ensure_go() {
    if usable_go go; then return; fi
    # Preserve support for an installed but inactive mise toolchain.
    if command -v mise >/dev/null 2>&1; then
        local installed go_root
        installed=$(mise latest --installed go 2>/dev/null || true)
        if [ -n "$installed" ]; then
            go_root=$(mise where "go@$installed" 2>/dev/null || true)
            if [ -n "$go_root" ] && usable_go "$go_root/bin/go"; then
                export PATH="$go_root/bin:$PATH"
                return
            fi
        fi
    fi
    local release archive expected actual
    release=$(curl -fsSL --retry 3 'https://go.dev/VERSION?m=text')
    release=${release%%$'\n'*}
    [[ "$release" =~ ^go[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail 'Invalid Go release metadata.'
    archive="$release.$os-$arch.tar.gz"
    printf 'Downloading %s for the build (removed after installation)…\n' "$release"
    curl -fsSL --retry 3 "https://dl.google.com/go/$archive" -o "$work_dir/go.tar.gz"
    expected=$(curl -fsSL --retry 3 "https://dl.google.com/go/$archive.sha256")
    [[ "$expected" =~ ^[0-9a-f]{64}$ ]] || fail 'Invalid Go checksum metadata.'
    if command -v sha256sum >/dev/null 2>&1; then
        actual=$(sha256sum "$work_dir/go.tar.gz")
    else
        actual=$(shasum -a 256 "$work_dir/go.tar.gz")
    fi
    [ "${actual%% *}" = "$expected" ] || fail 'Go download checksum mismatch.'
    tar -xzf "$work_dir/go.tar.gz" -C "$work_dir"
    export PATH="$work_dir/go/bin:$PATH"
    usable_go go || fail 'Downloaded Go toolchain cannot run on this system.'
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
    ensure_go
    curl --fail --show-error --silent --location --retry 3 \
        https://codeload.github.com/isaiash/diffplay/tar.gz/refs/heads/master \
        -o "$work_dir/source.tar.gz"
    mkdir "$work_dir/source"
    tar -xzf "$work_dir/source.tar.gz" -C "$work_dir/source" --strip-components=1
    sh "$work_dir/source/install.sh"
}

main "$@"
