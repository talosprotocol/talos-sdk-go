#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

# In CI, disable test caching to catch failures
if [[ "${CI:-}" == "true" ]]; then
  go test ./... -count=1
else
  go test ./...
fi
