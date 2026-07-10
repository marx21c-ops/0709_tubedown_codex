#!/bin/zsh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
APP="$HOME/Applications/TubeDown Local.app"

for binary in go yt-dlp ffmpeg deno; do
  if ! command -v "$binary" >/dev/null 2>&1; then
    print -u2 "$binary가 필요합니다. Homebrew로 설치한 후 다시 실행하세요."
    exit 1
  fi
done

mkdir -p "$APP/Contents/MacOS"
cp "$ROOT/packaging/TubeDown-Info.plist" "$APP/Contents/Info.plist"
GOCACHE="$ROOT/.cache/go-build" GOMODCACHE="$ROOT/.cache/go-mod" \
  go build -trimpath -ldflags="-s -w" -o "$APP/Contents/MacOS/TubeDown" "$ROOT/cmd/local"
chmod 700 "$APP/Contents/MacOS/TubeDown"
xattr -dr com.apple.quarantine "$APP" 2>/dev/null || true

print "설치 완료: $APP"
open "$APP"
