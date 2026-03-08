#!/usr/bin/env bash
# Run tests with coverage, excluding packages listed in coverage.exclude.
# Usage: ./scripts/run-coverage.sh [args...]
# Example: ./scripts/run-coverage.sh -count=1

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
EXCLUDE_FILE="$BACKEND_DIR/coverage.exclude"

cd "$BACKEND_DIR"

pkg_list=$(go list ./internal/... ./tests/... 2>/dev/null)

if [[ -f "$EXCLUDE_FILE" ]]; then
  while IFS= read -r pattern; do
    [[ -z "$pattern" ]] && continue
    [[ "$pattern" =~ ^[[:space:]]*# ]] && continue
    if [[ "$pattern" == *'$' ]]; then
      pkg_list=$(echo "$pkg_list" | grep -E -v "$pattern" || true)
    else
      pkg_list=$(echo "$pkg_list" | grep -v "$pattern" || true)
    fi
  done < <(grep -v '^[[:space:]]*$' "$EXCLUDE_FILE" 2>/dev/null || true)
fi

# Convert newlines to spaces for go test
packages=$(echo "$pkg_list" | tr '\n' ' ')

# Run tests with coverage; some packages may fail to build (covdata env) but we still get coverage from passing packages
set +e
go test $packages -coverprofile=coverage.out -count=1 "$@"
test_exit=$?
set -e

# Report (coverage.out is still written for successful packages)
if [[ -f coverage.out ]]; then
  go tool cover -func=coverage.out | tail -1
fi

exit $test_exit
