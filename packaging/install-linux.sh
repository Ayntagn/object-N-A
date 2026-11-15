#!/bin/sh
set -eu

if [ "$#" -lt 1 ] || [ "$#" -gt 3 ] || [ ! -f "$1" ]; then
    printf 'Usage: %s <built binary> [binary directory] [XDG data directory]\n' "$0" >&2
    exit 2
fi

case "$1" in
    /*) binary="$1" ;;
    *) binary="$PWD/$1" ;;
esac
default_data=${XDG_DATA_HOME:-"$HOME/.local/share"}
case "$default_data" in
    /*) ;;
    *) default_data="$HOME/.local/share" ;;
esac
binary_dir=${2:-"$HOME/.local/bin"}
data_dir=${3:-"$default_data"}
mkdir -p -- "$binary_dir" "$data_dir/applications"

binary_dir=$(cd -P -- "$binary_dir" >/dev/null && pwd)
executable="$binary_dir/object-n-a"
install -m 755 "$binary" "$executable"

# A quoted Exec uses both desktop entry string escaping and argument escaping.
quoted_executable=$(printf '%s' "$executable" | sed -e 's/\\/\\\\\\\\/g' -e 's/["$`]/\\\\&/g' -e 's/%/%%/g')
entry="$data_dir/applications/object-n-a.desktop"
cat > "$entry" <<EOF
[Desktop Entry]
Type=Application
Version=1.0
Name=object-N/A
GenericName=Markdown Workspace
Exec="$quoted_executable"
Terminal=false
StartupWMClass=object-n-a
Categories=Office;TextEditor;
EOF
printf '%s\n' "$entry"
