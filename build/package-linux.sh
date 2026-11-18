#!/bin/sh
# Packs each Linux build with its icon, menu entry and installer: dist/ResearchDigest-linux-<arch>.tar.gz
set -e
cd "$(dirname "$0")/.."
for arch in x64 arm64; do
  [ -f "dist/ResearchDigest-linux-$arch" ] || continue
  d=$(mktemp -d); dir="$d/ResearchDigest-linux-$arch"; mkdir -p "$dir/icons"
  cp "dist/ResearchDigest-linux-$arch" "$dir/"; chmod +x "$dir/ResearchDigest-linux-$arch"
  cp build/linux/install.sh build/linux/research-digest.desktop "$dir/"
  for s in 48 64 128 256 512; do cp "build/icon/icon-$s.png" "$dir/icons/"; done
  cp web/icon.svg "$dir/icons/icon.svg"
  tar -C "$d" -czf "dist/ResearchDigest-linux-$arch.tar.gz" "ResearchDigest-linux-$arch"
  rm -rf "$d"
done
