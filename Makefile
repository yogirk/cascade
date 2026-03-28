.PHONY: build test test-short lint clean

VERSION := $(shell cat VERSION 2>/dev/null || echo "dev")

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/cascade ./cmd/cascade/

test:
	go test ./... -count=1 -race

test-short:
	go test ./internal/... -short -count=1

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed, falling back to go vet"; \
		go vet ./...; \
	fi

clean:
	rm -rf bin/
