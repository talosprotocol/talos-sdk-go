#!/usr/bin/env bash
set -euo pipefail

# =============================================================================
# Talos SDK Conformance Runner (Go)
# =============================================================================

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION_FILE="$REPO_ROOT/.talos-version"

if [[ ! -f "$VERSION_FILE" ]]; then
    echo "❌ ERROR: Missing .talos-version file."
    exit 1
fi

RELEASE_SET=$(cat "$VERSION_FILE" | tr -d '[:space:]')
echo "🔒 Conformance Target: $RELEASE_SET"

MONOREPO_ROOT="$(cd "$REPO_ROOT/../.." && pwd)"
CONTRACTS_DIR="${TALOS_CONTRACTS_DIR:-$MONOREPO_ROOT/contracts}"
VECTORS_FILE="$CONTRACTS_DIR/test_vectors/sdk/release_sets/$RELEASE_SET.json"

if [[ ! -f "$VECTORS_FILE" ]]; then
    echo "❌ ERROR: Test vectors for $RELEASE_SET not found at $VECTORS_FILE"
    exit 1
fi

echo "✅ Vectors Found: $VECTORS_FILE"
go run ./cmd/vector-runner "$VECTORS_FILE"
