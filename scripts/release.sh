#!/bin/sh
# Cross-compiles the release archives into dist/.
#   scripts/release.sh v0.1.0
set -eu

version=${1:?usage: scripts/release.sh <version>}
rm -rf dist
mkdir -p dist

for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do
  os=${target%/*}
  arch=${target#*/}
  dir=$(mktemp -d)
  GOOS=$os GOARCH=$arch CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X main.version=$version" -o "$dir/ugh" .
  cp LICENSE README.md "$dir/"
  tar -C "$dir" -czf "dist/ugh_${os}_${arch}.tar.gz" ugh LICENSE README.md
  rm -rf "$dir"
done

cd dist
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum ./*.tar.gz | sed 's# \./# #' > checksums.txt
else
  shasum -a 256 ./*.tar.gz | sed 's# \./# #' > checksums.txt
fi
ls -l
