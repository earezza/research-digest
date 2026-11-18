#!/bin/sh
# Installs Research Digest for the current user: the program, its open-book icon and a menu entry.
# No root needed. Run from the unpacked folder:  ./install.sh      Remove with:  ./install.sh --uninstall
set -e
here=$(cd "$(dirname "$0")" && pwd)
bin="$HOME/.local/bin/research-digest"
icons="$HOME/.local/share/icons/hicolor"
app="$HOME/.local/share/applications/research-digest.desktop"
if [ "$1" = "--uninstall" ]; then
  rm -f "$bin" "$app"
  for s in 48 64 128 256 512; do rm -f "$icons/${s}x${s}/apps/research-digest.png"; done
  rm -f "$icons/scalable/apps/research-digest.svg"
  echo "Research Digest removed. Your saved categories are kept in ~/.config/ResearchDigest."
  exit 0
fi
prog=$(ls "$here"/ResearchDigest-linux-* 2>/dev/null | head -n1)
[ -n "$prog" ] || { echo "ResearchDigest-linux-* not found next to install.sh"; exit 1; }
mkdir -p "$HOME/.local/bin" "$(dirname "$app")"
install -m 755 "$prog" "$bin"
for s in 48 64 128 256 512; do
  mkdir -p "$icons/${s}x${s}/apps"; install -m 644 "$here/icons/icon-$s.png" "$icons/${s}x${s}/apps/research-digest.png"
done
mkdir -p "$icons/scalable/apps"; install -m 644 "$here/icons/icon.svg" "$icons/scalable/apps/research-digest.svg"
sed "s|^Exec=.*|Exec=$bin|" "$here/research-digest.desktop" > "$app"
chmod 644 "$app"
command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database -q "$(dirname "$app")" || true
command -v gtk-update-icon-cache >/dev/null 2>&1 && gtk-update-icon-cache -q -t "$icons" || true
echo "Installed. Open Research Digest from your applications menu, or run: $bin"
