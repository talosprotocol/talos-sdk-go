# =============================================================================
# talos-sdk-go Test Script
# =============================================================================
set -euo pipefail
cd "$(dirname "$0")/.."

log() { printf '%s\n' "$*"; }
info() { printf 'ℹ️  %s\n' "$*"; }

info "Testing talos-sdk-go..."

# In CI, disable test caching to catch failures
if [[ "${CI:-}" == "true" ]]; then
  go test ./... -count=1
else
  go test ./...
fi

log "✓ talos-sdk-go tests passed."
