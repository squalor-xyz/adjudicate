#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Install adj from a GitHub release: download the archive for this system,
# check it against SHA256SUMS, and copy adj into a directory.
#
#   sh scripts/install.sh [--version X.Y.Z] [--dir DIR] [--dry-run]
#
# Defaults: the latest release, DIR = $ADJ_INSTALL_DIR or ~/.local/bin.
# Linux, macOS and FreeBSD on amd64 and arm64. On Windows use the zip
# (see README.md). Needs curl or wget, tar, and sha256sum or shasum.
set -eu

repo="squalor-xyz/adjudicate"
base="https://github.com/$repo"
version="${ADJ_VERSION:-}"
dir="${ADJ_INSTALL_DIR:-$HOME/.local/bin}"
dry=0

die() { echo "install.sh: $*" >&2; exit 1; }

while [ $# -gt 0 ]; do
	case "$1" in
	--version) [ $# -ge 2 ] || die "--version needs a value"; version=$2; shift 2 ;;
	--dir) [ $# -ge 2 ] || die "--dir needs a value"; dir=$2; shift 2 ;;
	--dry-run) dry=1; shift ;;
	-h|--help) sed -n '3,11p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
	*) die "unknown argument: $1" ;;
	esac
done

case "$(uname -s)" in
Linux) os=linux ;;
Darwin) os=darwin ;;
FreeBSD) os=freebsd ;;
*) die "unsupported OS $(uname -s); on Windows use the zip from $base/releases" ;;
esac
case "$(uname -m)" in
x86_64|amd64) arch=amd64 ;;
aarch64|arm64) arch=arm64 ;;
*) die "unsupported architecture $(uname -m)" ;;
esac

if command -v curl >/dev/null 2>&1; then
	fetch() { curl -fsSL -o "$2" "$1"; }
	final_url() { curl -fsSLI -o /dev/null -w '%{url_effective}' "$1"; }
elif command -v wget >/dev/null 2>&1; then
	fetch() { wget -q -O "$2" "$1"; }
	final_url() { wget -q --spider -S "$1" 2>&1 | sed -n 's/^ *[Ll]ocation: *//p' | tail -n 1 | tr -d '\r'; }
else
	die "need curl or wget"
fi
if command -v sha256sum >/dev/null 2>&1; then
	sha() { sha256sum "$1" | cut -d' ' -f1; }
elif command -v shasum >/dev/null 2>&1; then
	sha() { shasum -a 256 "$1" | cut -d' ' -f1; }
else
	die "need sha256sum or shasum"
fi

if [ -z "$version" ]; then
	tag=$(final_url "$base/releases/latest" | sed 's|.*/tag/||')
	case "$tag" in v[0-9]*) ;; *) die "could not find the latest release (got '$tag'); pass --version" ;; esac
	version=${tag#v}
fi
version=${version#v}
case "$version" in
*[!0-9A-Za-z.+-]*|"") die "bad version '$version'" ;;
esac

name="adj_${version}_${os}_${arch}"
url="$base/releases/download/v$version"

if [ "$dry" -eq 1 ]; then
	echo "would download $url/$name.tar.gz"
	echo "would install adj $version to $dir/adj"
	exit 0
fi

tmp=$(mktemp -d "${TMPDIR:-/tmp}/adj-install.XXXXXX")
trap 'rm -rf "$tmp"' EXIT INT TERM

echo "Downloading adj $version for $os/$arch"
fetch "$url/$name.tar.gz" "$tmp/$name.tar.gz" || die "download failed: $url/$name.tar.gz"
fetch "$url/SHA256SUMS" "$tmp/SHA256SUMS" || die "download failed: $url/SHA256SUMS"

want=$(awk -v f="$name.tar.gz" '$2 == f || $2 == "*" f { print $1 }' "$tmp/SHA256SUMS")
[ -n "$want" ] || die "$name.tar.gz is not listed in SHA256SUMS"
got=$(sha "$tmp/$name.tar.gz")
[ "$got" = "$want" ] || die "checksum mismatch for $name.tar.gz (want $want, got $got)"

tar -xzf "$tmp/$name.tar.gz" -C "$tmp"
[ -f "$tmp/$name/adj" ] || die "archive has no $name/adj"

mkdir -p "$dir"
install -m 0755 "$tmp/$name/adj" "$dir/adj"
echo "Installed adj $version to $dir/adj"

case ":$PATH:" in
*":$dir:"*) ;;
*) echo "Note: $dir is not on your PATH." ;;
esac
