SHELL := /bin/bash

.PHONY: all build test lint sbom clean

all: build test

build:
	go build -v ./...

test:
	go test -v ./...

lint:
	which golangci-lint || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v1.56.2
	golangci-lint run

sbom:
	which cyclonedx-gomod || go install github.com/cyclonedx/cyclonedx-gomod/cmd/cyclonedx-gomod@latest
	cyclonedx-gomod mod -json -output bom.json

clean:
	@rm -f bom.json
