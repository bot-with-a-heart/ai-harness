#!/usr/bin/env sh
set -eu

VERSION="${VERSION:-dev}"
OUTPUT_DIR="${OUTPUT_DIR:-dist}"
TARGETS="${TARGETS:-windows/amd64 linux/amd64 darwin/amd64 darwin/arm64}"

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
dist_root="$repo_root/$OUTPUT_DIR"

mkdir -p "$dist_root"

for target in $TARGETS; do
    goos=${target%/*}
    goarch=${target#*/}
    if [ "$goos" = "$goarch" ]; then
        echo "Invalid target '$target'. Expected GOOS/GOARCH." >&2
        exit 1
    fi

    target_name="ai-harness-$VERSION-$goos-$goarch"
    target_dir="$dist_root/$target_name"
    mkdir -p "$target_dir"

    binary="ai-harness"
    if [ "$goos" = "windows" ]; then
        binary="ai-harness.exe"
    fi

    output="$target_dir/$binary"
    CGO_ENABLED="${CGO_ENABLED:-0}" GOOS="$goos" GOARCH="$goarch" \
        go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o "$output" ./cmd/ai-harness
    echo "Built $output"
done
