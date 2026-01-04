# Universal Makefile Interface
all: install lint test build conformance

install:
	go mod download

typecheck:
	# Go build acts as typecheck
	go build ./...

lint:
	# Style + Types (Fail on error)
	golangci-lint run ./...

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
