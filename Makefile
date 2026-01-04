# Universal Makefile Interface
all: install lint test build conformance

install:
	go mod download

typecheck:
	# Go build acts as typecheck
	go build ./...

lint:
	# Style + Types (Fail on error)
	@if command -v golangci-lint >/dev/null; then \
		golangci-lint run ./...; \
	else \
		echo "Warning: golangci-lint not found, falling back to gofmt"; \
		gofmt -l .; \
	fi

format:
	# Auto-fix style
	gofmt -w .
	goimports -w .

test:
	# Unit tests
	go test ./... -cover

conformance:
	# Run conformance vectors
	@if [ -z "$(RELEASE_SET)" ]; then \
		echo "Skipping conformance (No RELEASE_SET provided)"; \
	else \
		go test ./pkg/talos/conformance -v -args -vectors $(RELEASE_SET); \
	fi

build:
	go build -o bin/talos-sdk ./cmd/talos-sdk

clean:
	rm -rf bin
