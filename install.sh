#!/bin/sh
# Install termagitchi.
#
#   curl -fsSL https://raw.githubusercontent.com/TevvvB/termagitchi/main/install.sh | sh
#
# Honours PETS_VERSION to pin a release and PETS_INSTALL_DIR to choose where the
# binary lands. Downloads are checksummed against the release before install.

set -eu

REPO="TevvvB/termagitchi"
INSTALL_DIR="${PETS_INSTALL_DIR:-$HOME/.local/bin}"

say() { printf '%s\n' "$*"; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }

need() { command -v "$1" >/dev/null 2>&1; }

fetch() {
  if need curl; then curl -fsSL "$1" -o "$2"
  elif need wget; then wget -qO "$2" "$1"
  else die "need curl or wget"
  fi
}

fetch_stdout() {
  if need curl; then curl -fsSL "$1"
  elif need wget; then wget -qO- "$1"
  else die "need curl or wget"
  fi
}

# Resolve the platform slug the release archives are named with.
detect_target() {
  os=$(uname -s)
  arch=$(uname -m)
  case "$os" in
    Linux) os=linux ;;
    Darwin) os=darwin ;;
    *) die "unsupported OS: $os. Build from source with: go install github.com/$REPO/cmd/pets@latest" ;;
  esac
  case "$arch" in
    x86_64 | amd64) arch=amd64 ;;
    aarch64 | arm64) arch=arm64 ;;
    *) die "unsupported architecture: $arch" ;;
  esac
  printf '%s_%s' "$os" "$arch"
}

latest_version() {
  # Following the /releases/latest redirect avoids the API, which is rate
  # limited for unauthenticated callers.
  if need curl; then
    url=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest" 2>/dev/null || true)
    version=${url##*/tag/}
    if [ -n "$version" ] && [ "$version" != "$url" ]; then
      printf '%s' "${version#v}"
      return
    fi
  fi
  # BusyBox wget, which is all Alpine ships, cannot report redirects. The API
  # reads fine from stdout on any downloader.
  version=$(fetch_stdout "https://api.github.com/repos/$REPO/releases/latest" |
    sed -n 's/.*"tag_name" *: *"v\{0,1\}\([^"]*\)".*/\1/p' | head -1)
  [ -n "$version" ] || die "could not determine the latest version. Set PETS_VERSION to install a specific one."
  printf '%s' "${version#v}"
}

checksum() {
  if need sha256sum; then sha256sum "$1" | cut -d' ' -f1
  elif need shasum; then shasum -a 256 "$1" | cut -d' ' -f1
  else printf ''
  fi
}

target=$(detect_target)
version="${PETS_VERSION:-$(latest_version)}"
version="${version#v}"
archive="termagitchi_${version}_${target}.tar.gz"
base="https://github.com/$REPO/releases/download/v${version}"

say "termagitchi ${version} (${target})"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

say "downloading"
fetch "$base/$archive" "$tmp/$archive" || die "could not download $archive. Is $version a real release?"

# A tampered or truncated download should fail here rather than land on the PATH.
say "verifying"
if fetch "$base/checksums.txt" "$tmp/checksums.txt" 2>/dev/null; then
  want=$(grep " $archive\$" "$tmp/checksums.txt" | cut -d' ' -f1)
  got=$(checksum "$tmp/$archive")
  if [ -z "$got" ]; then
    say "  no sha256 tool found, skipping verification"
  elif [ -z "$want" ]; then
    die "no checksum published for $archive"
  elif [ "$want" != "$got" ]; then
    die "checksum mismatch for $archive
  expected $want
  got      $got"
  fi
else
  say "  no checksums published, skipping verification"
fi

tar -xzf "$tmp/$archive" -C "$tmp" || die "could not extract $archive"
[ -f "$tmp/pets" ] || die "archive did not contain a pets binary"

mkdir -p "$INSTALL_DIR" || die "could not create $INSTALL_DIR"
mv "$tmp/pets" "$INSTALL_DIR/pets" 2>/dev/null ||
  die "could not write to $INSTALL_DIR. Set PETS_INSTALL_DIR to somewhere writable."
chmod 755 "$INSTALL_DIR/pets"

say "installed to $INSTALL_DIR/pets"

# An installed binary that is not on the PATH is the most common way this goes
# wrong, so say it plainly rather than letting the next command fail.
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    say ""
    say "$INSTALL_DIR is not on your PATH. Add it:"
    say "  echo 'export PATH=\"\$PATH:$INSTALL_DIR\"' >> ~/.profile"
    ;;
esac

say ""
say "Next:"
say "  pets install     wire it into your coding agent, then start a new session"
say "  pets party       every live worktree at once"
