SHELL := /bin/bash

.PHONY: all build test lint sbom clean

all: build test

build:
\tgo build -v ./...

test:
\tgo test -v ./...

lint:
\twhich golangci-lint || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.56.2
\tgolangci-lint run

sbom:
\twhich cyclonedx-gomod || go install github.com/cyclonedx/cyclonedx-gomod/cmd/cyclonedx-gomod@latest
\tcyclonedx-gomod mod -json -output bom.json

clean:
\t@rm -f bom.json
