#!/bin/bash
set -euo pipefail

echo "Cleaning up..."
rm -f talos-sdk conformance.xml
go clean
