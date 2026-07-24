#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
coverage_file="${1:-}"

if [ -n "$coverage_file" ]; then
    case "$coverage_file" in
        /*) ;;
        *) coverage_file="$(pwd)/$coverage_file" ;;
    esac
    coverage_tmp="$(mktemp "${coverage_file}.tmp.XXXXXX")"
    trap 'rm -f "$coverage_tmp"' EXIT
else
    coverage_file="$(mktemp "${TMPDIR:-/tmp}/x-mate-coverage.XXXXXX")"
    coverage_tmp="$coverage_file"
    trap 'rm -f "$coverage_file"' EXIT
fi

cd "$script_dir"

echo "=== Compiling ==="
go build ./...
echo ""
echo "=== Running All Tests with Coverage ==="
go test -coverprofile="$coverage_tmp" -count=1 ./...

if [ "$coverage_tmp" != "$coverage_file" ]; then
    chmod 0644 "$coverage_tmp"
    mv "$coverage_tmp" "$coverage_file"
fi

echo ""
echo "=== Total Coverage ==="
go tool cover -func="$coverage_file" | grep "total:"
echo ""
echo "=== Remaining 0% Functions ==="
zero_coverage="$(go tool cover -func="$coverage_file" | awk '$NF == "0.0%"')"
if [ -z "$zero_coverage" ]; then
    echo "🎉 None! All functions have coverage > 0%"
else
    echo "$zero_coverage"
fi
