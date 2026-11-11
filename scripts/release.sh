#!/bin/sh
# Cross-compiles the release archives into dist/.
#   scripts/release.sh v0.1.0
set -eu

version=${1:?usage: scripts/release.sh <version>}
rm -rf dist
mkdir -p dist

for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
  os=${target%/*}
  arch=${target#*/}
  dir=$(mktemp -d)
  exe=ugh
  [ "$os" = windows ] && exe=ugh.exe
  GOOS=$os GOARCH=$arch CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X main.version=$version" -o "$dir/$exe" .
  cp LICENSE README.md "$dir/"
  if [ "$os" = windows ]; then
    (cd "$dir" && zip -q "$OLDPWD/dist/ugh_${os}_${arch}.zip" "$exe" LICENSE README.md)
  else
    tar -C "$dir" -czf "dist/ugh_${os}_${arch}.tar.gz" "$exe" LICENSE README.md
  fi
  rm -rf "$dir"
done

cd dist
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum ./*.tar.gz ./*.zip | sed 's# \./# #' > checksums.txt
else
  shasum -a 256 ./*.tar.gz ./*.zip | sed 's# \./# #' > checksums.txt
fi
ls -l
