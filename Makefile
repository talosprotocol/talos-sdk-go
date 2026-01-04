# talos-sdk-go Makefile

.PHONY: build test conformance clean doctor start stop

# Default target
all: build test

build:
	@echo "Building Go SDK..."
	go build -o talos-sdk ./cmd/talos-sdk

test:
	@echo "Running tests..."
	go test ./pkg/...

conformance: build
	@echo "Running conformance tests..."
	./talos-sdk --vectors ../talos-contracts/test_vectors/sdk/release_sets/v1.0.0.json --report conformance.xml

doctor:
	@echo "Checking environment..."
	@go version || echo "Go missing"

clean:
	@echo "Cleaning..."
	rm -f talos-sdk conformance.xml
	go clean

# Scripts wrapper
start:
	@./scripts/start.sh

stop:
	@./scripts/stop.sh
