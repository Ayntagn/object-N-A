#!/bin/sh
set -eu

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ] || [ ! -f "$1" ]; then
    printf 'Usage: %s <built binary> [applications directory]\n' "$0" >&2
    exit 2
fi

case "$1" in
    /*) binary="$1" ;;
    *) binary="$PWD/$1" ;;
esac
package_dir=$(cd -P -- "$(dirname -- "$0")" >/dev/null && pwd)
destination=${2:-"$HOME/Applications"}
bundle="$destination/object-N-A.app"

mkdir -p "$bundle/Contents/MacOS" "$bundle/Contents/Resources"
install -m 755 "$binary" "$bundle/Contents/MacOS/object-n-a"
install -m 644 "$package_dir/Info.plist" "$bundle/Contents/Info.plist"
install -m 644 "$package_dir/object-n-a.icns" "$bundle/Contents/Resources/object-n-a.icns"
plutil -lint "$bundle/Contents/Info.plist"
codesign --force --sign - "$bundle"
printf '%s\n' "$bundle"
