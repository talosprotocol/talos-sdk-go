# =============================================================================
# Talos Go SDK - Tool Image (Monorepo-Root Context)
# =============================================================================

# Builder stage - compile and cache dependencies
FROM golang:1.21-alpine AS builder

WORKDIR /workspace

# Copy go mod files for caching
COPY sdks/go/go.mod sdks/go/go.sum ./sdks/go/
WORKDIR /workspace/sdks/go
RUN go mod download

# Copy source
WORKDIR /workspace
COPY sdks/go ./sdks/go
COPY contracts ./contracts
COPY scripts ./scripts

# Build tests (compile time verification)
WORKDIR /workspace/sdks/go
RUN go test -c ./...

# Test runner stage
FROM golang:1.21-alpine

# OCI labels
LABEL org.opencontainers.image.source="https://github.com/talosprotocol/talos"
LABEL org.opencontainers.image.description="Talos Go SDK Tool Image"
LABEL org.opencontainers.image.licenses="Apache-2.0"

WORKDIR /workspace/sdks/go

# Install runtime dependencies
RUN apk add --no-cache git make bash

# Copy from builder
COPY --from=builder /workspace/sdks/go ./
COPY --from=builder /workspace/contracts /workspace/contracts
COPY --from=builder /workspace/scripts /workspace/scripts

# Create non-root user
RUN adduser -D -u 1000 talos
RUN chown -R talos:talos /workspace
USER talos

# Default: run CI tests
CMD ["scripts/test.sh", "--ci"]
