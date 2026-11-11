#!/bin/sh
# Installs the latest ugh release into ~/.local/bin (or $UGH_INSTALL_DIR).
#   curl -fsSL https://raw.githubusercontent.com/themrsami/ugh/main/install.sh | sh
set -eu

repo=themrsami/ugh
dir=${UGH_INSTALL_DIR:-$HOME/.local/bin}

case $(uname -s) in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) echo "ugh: no prebuilt binary for $(uname -s), try: go install github.com/$repo@latest" >&2; exit 1 ;;
esac
case $(uname -m) in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) echo "ugh: no prebuilt binary for $(uname -m), try: go install github.com/$repo@latest" >&2; exit 1 ;;
esac

file=ugh_${os}_${arch}.tar.gz
base=https://github.com/$repo/releases/latest/download
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "downloading $file"
curl -fsSL "$base/$file" -o "$tmp/$file"
curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt"

want=$(grep " $file\$" "$tmp/checksums.txt" | cut -d' ' -f1)
if command -v sha256sum >/dev/null 2>&1; then
  got=$(sha256sum "$tmp/$file" | cut -d' ' -f1)
else
  got=$(shasum -a 256 "$tmp/$file" | cut -d' ' -f1)
fi
if [ -z "$want" ] || [ "$want" != "$got" ]; then
  echo "ugh: checksum mismatch, not installing" >&2
  exit 1
fi

tar -C "$tmp" -xzf "$tmp/$file" ugh
mkdir -p "$dir"
mv "$tmp/ugh" "$dir/ugh"
chmod +x "$dir/ugh"

echo "installed $("$dir/ugh" --version) to $dir/ugh"
case ":$PATH:" in
  *":$dir:"*) ;;
  *) echo "note: $dir is not on your PATH yet" ;;
esac
echo "next: ugh --init   (shows the one line to add to your shell config)"
