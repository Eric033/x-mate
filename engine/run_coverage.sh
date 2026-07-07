#!/usr/bin/env bash
cd ~/.openclaw/workspace-elon/projects/x-mate/engine
export PATH="$HOME/.local/go/bin:$PATH"
echo "=== Compiling ==="
go build ./...
echo ""
echo "=== Running All Tests with Coverage ==="
go test -coverprofile=/tmp/coverage_result.out -count=1 ./...
echo ""
echo "=== Total Coverage ==="
go tool cover -func=/tmp/coverage_result.out 2>/dev/null | grep "total:"
echo ""
echo "=== Remaining 0% Functions ==="
ZERO=$(go tool cover -func=/tmp/coverage_result.out 2>/dev/null | grep "0.0%")
if [ -z "$ZERO" ]; then
    echo "🎉 None! All functions have coverage > 0%"
else
    echo "$ZERO"
fi
