#!/usr/bin/env bash
set -eo pipefail

# =============================================================================
# Go SDK Standardized Test Entrypoint
# =============================================================================

ARTIFACTS_DIR="artifacts/coverage"
mkdir -p "$ARTIFACTS_DIR"

COMMAND=${1:-"--unit"}

run_unit() {
    echo "=== Running Unit Tests ==="
    go test -v ./...
}

run_smoke() {
    echo "=== Running Smoke Tests ==="
    go test -v -run TestSmoke ./...
}

run_integration() {
    echo "=== Running Integration Tests ==="
    go test -v -tags=integration ./...
}

run_coverage() {
    echo "=== Running Coverage (go test -coverprofile) ==="
    go test -v -coverprofile="$ARTIFACTS_DIR/coverage.out" ./...
    
    # Convert to cobertura XML using gocover-cobertura
    if ! command -v gocover-cobertura &> /dev/null; then
        echo "⚠️  gocover-cobertura not found. Skipping coverage."
        echo "To install: go install github.com/boumenot/gocover-cobertura@latest"
        return 0
    fi
    
    gocover-cobertura < "$ARTIFACTS_DIR/coverage.out" > "$ARTIFACTS_DIR/coverage.xml"
    echo "✅ Coverage report generated: $ARTIFACTS_DIR/coverage.xml"
}

case "$COMMAND" in
    --smoke)
        run_smoke
        ;;
    --unit)
        run_unit
        ;;
    --integration)
        run_integration
        ;;
    --coverage)
        run_coverage
        ;;
    --ci)
        run_smoke
        run_unit
        run_coverage
        ;;
    --full)
        run_smoke
        run_unit
        run_integration
        run_coverage
        ;;
    *)
        echo "Usage: $0 {--smoke|--unit|--integration|--coverage|--ci|--full}"
        exit 1
        ;;
esac

# Generate minimal results.json
mkdir -p artifacts/test
cat <<EOF > artifacts/test/results.json
{
  "repo_id": "sdks-go",
  "command": "$COMMAND",
  "status": "pass",
  "timestamp": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
}
EOF
