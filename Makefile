# =============================================================================
# Talos Go SDK Makefile
# =============================================================================
SHELL := /bin/bash

# Variables
IMAGE_NAME ?= talos-sdk-go
IMAGE_TAG ?= latest
REGISTRY ?= ghcr.io/talosprotocol
FULL_IMAGE := $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)

.PHONY: all build test lint coverage coverage-html docker-build docker-push install-tools sbom clean help

# Default target
all: install-tools build test

# Help target
help:
	@echo "Talos Go SDK - Available targets:"
	@echo "  make build          - Build the Go module"
	@echo "  make test           - Run all tests"
	@echo "  make lint           - Run linter"
	@echo "  make coverage       - Generate coverage report (requires gocover-cobertura)"
	@echo "  make coverage-html  - Generate HTML coverage report"
	@echo "  make docker-build   - Build Docker image"
	@echo "  make docker-push    - Push Docker image to registry"
	@echo "  make install-tools  - Install required Go tools"
	@echo "  make sbom           - Generate Software Bill of Materials"
	@echo "  make clean          - Clean build artifacts"

# Build
build:
	@echo "🔨 Building Go SDK..."
	go build -v ./...

# Test (delegates to scripts/test.sh)
test:
	@echo "🧪 Running tests..."
	@./scripts/test.sh --unit

# Coverage (delegates to scripts/test.sh)
coverage:
	@echo "📊 Generating coverage report..."
	@./scripts/test.sh --coverage
lint:
	@echo "🔍 Running linter..."
	@which golangci-lint || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin latest
	$$(go env GOPATH)/bin/golangci-lint run


# HTML Coverage
coverage-html:
	@echo "📊 Generating HTML coverage report..."
	@mkdir -p artifacts/coverage
	go test -v -coverprofile=artifacts/coverage/coverage.out ./...
	go tool cover -html=artifacts/coverage/coverage.out -o artifacts/coverage/coverage.html
	@echo "✅ HTML coverage report: artifacts/coverage/coverage.html"

# Docker Build
docker-build:
	@echo "🐳 Building Docker image..."
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) -t $(FULL_IMAGE) -f Dockerfile .
	@echo "✅ Image built: $(IMAGE_NAME):$(IMAGE_TAG)"

# Docker Push
docker-push: docker-build
	@echo "📤 Pushing Docker image..."
	docker push $(FULL_IMAGE)
	@echo "✅ Image pushed: $(FULL_IMAGE)"

# Install Tools
install-tools:
	@echo "🔧 Installing Go tools..."
	@which golangci-lint || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	@which gocover-cobertura || (echo "Installing gocover-cobertura..." && go install github.com/boumenot/gocover-cobertura@latest)
	@which cyclonedx-gomod || (echo "Installing cyclonedx-gomod..." && go install github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@latest)
	@echo "✅ All tools installed"

# SBOM
sbom:
	@echo "📋 Generating SBOM..."
	@which cyclonedx-gomod || go install github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@latest
	cyclonedx-gomod mod -json -output bom.json
	@echo "✅ SBOM generated: bom.json"

# Clean
clean:
	@echo "🧹 Cleaning up..."
	@rm -rf artifacts/ bom.json
	@echo "✅ Cleaned"
