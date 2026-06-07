#!/usr/bin/env sh
set -eu

prefix="${HOME}/.local"
binary_path=""
build=0
dry_run=0

while [ "$#" -gt 0 ]; do
    case "$1" in
        --prefix)
            prefix="$2"
            shift 2
            ;;
        --binary)
            binary_path="$2"
            shift 2
            ;;
        --build)
            build=1
            shift
            ;;
        --dry-run)
            dry_run=1
            shift
            ;;
        -h|--help)
            cat <<'EOF'
Usage: ./install.sh [--prefix DIR] [--binary PATH] [--build] [--dry-run]

Installs ai-harness to DIR/bin. The default prefix is $HOME/.local.
EOF
            exit 0
            ;;
        *)
            echo "Unknown argument: $1" >&2
            exit 1
            ;;
    esac
done

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
install_dir="$prefix/bin"
go_bin="${GO:-go}"

if [ -z "$binary_path" ]; then
    binary_path="$repo_root/ai-harness"
fi

if [ "$build" -eq 1 ]; then
    echo "Build: $go_bin build -trimpath -o '$binary_path' ./cmd/ai-harness"
    if [ "$dry_run" -eq 0 ]; then
        (cd "$repo_root" && "$go_bin" build -trimpath -o "$binary_path" ./cmd/ai-harness)
    fi
fi

if [ "$dry_run" -eq 0 ] && [ ! -f "$binary_path" ]; then
    echo "Binary not found at $binary_path. Run scripts/build.sh or pass --build." >&2
    exit 1
fi

target="$install_dir/ai-harness"
echo "Install directory: $install_dir"
echo "Binary source: $binary_path"
echo "Binary target: $target"

if [ "$dry_run" -eq 0 ]; then
    mkdir -p "$install_dir"
    cp "$binary_path" "$target"
    chmod 755 "$target"
fi

case ":$PATH:" in
    *":$install_dir:"*)
        ;;
    *)
        echo "PATH does not include $install_dir. Add it to your shell profile to run ai-harness from anywhere."
        ;;
esac

if [ "$dry_run" -eq 1 ]; then
    echo "Dry run complete. No files were changed."
else
    echo "Installed ai-harness to $target"
fi
