#!/bin/sh
set -eu

# Build locally; the installed executable needs neither Go nor shared GUI libraries.
source_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
prefix=${PREFIX:-"$HOME/.local"}
# A mise shim can exist on PATH even when no Go version is activated.
# Probe the compiler, then resolve an already-installed tool without changing
# mise configuration or downloading a toolchain.
go_bin=go
if ! "$go_bin" version >/dev/null 2>&1; then
    go_bin=
    if command -v mise >/dev/null 2>&1; then
        go_version=$(mise latest --installed go 2>/dev/null || true)
        if [ -n "$go_version" ]; then
            go_root=$(mise where "go@$go_version" 2>/dev/null || true)
            if [ -n "$go_root" ] && "$go_root/bin/go" version >/dev/null 2>&1; then
                go_bin="$go_root/bin/go"
            fi
        fi
    fi
    if [ -z "$go_bin" ]; then
        printf '%s\n' 'Go 1.24 or newer is required to build. Install Go (or install it with mise), then rerun ./install.sh.' >&2
        exit 1
    fi
fi
cache_dir=${TMPDIR:-"$HOME/.cache"}
mkdir -p "$cache_dir"
build_dir=$(mktemp -d "$cache_dir/diffplay-build.XXXXXX")
trap 'rm -rf "$build_dir"' EXIT HUP INT TERM
cd "$source_dir"
CGO_ENABLED=0 "$go_bin" build -trimpath -ldflags='-s -w' -o "$build_dir/diffplay" .
mkdir -p "$prefix/bin"
install -m 755 "$build_dir/diffplay" "$prefix/bin/diffplay"
printf 'Installed %s\n' "$prefix/bin/diffplay"
case ":$PATH:" in
    *":$prefix/bin:"*) ;;
    *) printf 'Add %s/bin to your PATH.\n' "$prefix" ;;
esac
