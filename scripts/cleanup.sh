#!/bin/bash
set -euo pipefail

echo "Cleaning talos-sdk-go..."
# Binary artifacts
rm -f talos-sdk 2>/dev/null || true
# Test artifacts
rm -f conformance.xml 2>/dev/null || true
# Coverage & reports
rm -f coverage.out coverage.html 2>/dev/null || true
rm -rf coverage 2>/dev/null || true
# Go cache clean
go clean 2>/dev/null || true
echo "✓ talos-sdk-go cleaned"
