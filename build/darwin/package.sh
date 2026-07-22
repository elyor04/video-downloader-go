#!/usr/bin/env bash
set -euo pipefail

# Packages build/bin/video-downloader-go.app (produced by
# `wails build -platform darwin/universal`) into a distributable
# build/bin/video-downloader-go.dmg.
#
# Wails' own packaging only ever embeds the Go binary itself -- unlike
# build/windows/installer/project.nsi, nothing here copies
# bin/{yt-dlp,ffmpeg,ffprobe} into the bundle, so without this step
# utils.ResolveBundledPath finds nothing inside a distributed .app and every
# operation falls back to (usually absent) PATH binaries. This script does
# that copy, the macOS equivalent of project.nsi's explicit bin/ copy.

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
APP="$REPO_ROOT/build/bin/video-downloader-go.app"
DMG="$REPO_ROOT/build/bin/video-downloader-go.dmg"

if [ ! -d "$APP" ]; then
  echo "error: $APP not found -- run 'wails build -platform darwin/universal' first" >&2
  exit 1
fi

if ! command -v create-dmg >/dev/null 2>&1; then
  echo "error: create-dmg not found -- install it with 'brew install create-dmg'" >&2
  exit 1
fi

BIN_DIR="$APP/Contents/MacOS/bin"
mkdir -p "$BIN_DIR"
for name in yt-dlp ffmpeg ffprobe; do
  src="$REPO_ROOT/bin/$name"
  if [ ! -f "$src" ]; then
    echo "error: $src not found -- run 'go generate ./...' first to fetch it" >&2
    exit 1
  fi
  cp "$src" "$BIN_DIR/$name"
  chmod +x "$BIN_DIR/$name"
done

# Adding files after Wails' own signing invalidates the seal, so re-sign the
# whole bundle the same way Wails does it (ad-hoc, no Developer ID/notarization).
codesign --force --deep --sign - "$APP"

rm -f "$DMG"
create-dmg \
  --volname "Video Downloader" \
  --window-size 660 400 \
  --icon-size 128 \
  --icon "video-downloader-go.app" 180 170 \
  --hide-extension "video-downloader-go.app" \
  --app-drop-link 480 170 \
  "$DMG" \
  "$APP"

echo "Created $DMG"
