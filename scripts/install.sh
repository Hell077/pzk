#!/bin/sh
# Install the latest checksummed Portguard release without root privileges.
set -eu
case "$(uname -s)" in Linux) os=linux ;; Darwin) os=darwin ;; *) echo 'Use install.ps1 on Windows; supported Unix systems are Linux and macOS.' >&2; exit 1 ;; esac
case "$(uname -m)" in x86_64|amd64) arch=amd64 ;; arm64|aarch64) arch=arm64 ;; *) echo 'Unsupported architecture; install from source with Go 1.27.3+.' >&2; exit 1 ;; esac
asset="portguard_${os}_${arch}"
base='https://github.com/Hell077/portguard/releases/latest/download'
dir="${PORTGUARD_INSTALL_DIR:-$HOME/.local/bin}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
curl -fsSL --retry 3 "$base/$asset" -o "$tmp/$asset"
curl -fsSL --retry 3 "$base/checksums.txt" -o "$tmp/checksums.txt"
expected="$(awk -v name="$asset" '$2 == name { print $1 }' "$tmp/checksums.txt")"
if command -v sha256sum >/dev/null 2>&1; then
 actual="$(sha256sum "$tmp/$asset" | awk '{print $1}')"
else
 actual="$(shasum -a 256 "$tmp/$asset" | awk '{print $1}')"
fi
if [ -z "$expected" ] || [ "$expected" != "$actual" ]; then echo 'Checksum verification failed; nothing installed.' >&2; exit 1; fi
mkdir -p "$dir"
install -m 0755 "$tmp/$asset" "$dir/portguard"
printf 'Installed Portguard to %s/portguard\n' "$dir"
case ":$PATH:" in *":$dir:"*) ;; *) printf 'Add this directory to your shell PATH: %s\n' "$dir" ;; esac
