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

coverage:
	# Run coverage report
	go test -coverprofile=coverage.out ./pkg/...
	go tool cover -func=coverage.out

coverage-check:
	# Enforce 80% threshold
	@go test -coverprofile=coverage.out ./pkg/... > /dev/null
	@TOTAL_COV=$$(go tool cover -func=coverage.out | grep total | grep -o '[0-9]*\.[0-9]*'); \
	echo "Total Coverage: $$TOTAL_COV%"; \
	if [ 1 -eq "$$(echo "$$TOTAL_COV < 80.0" | bc)" ]; then \
		echo "Coverage below 80%"; \
		exit 1; \
	fi

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
