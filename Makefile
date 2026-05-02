.PHONY: build test test-short lint check tools verify-pure-go clean

VERSION := $(shell cat VERSION 2>/dev/null || echo "dev")

# Pure-Go invariant: Cascade ships as a single static binary across
# darwin / linux / windows with no C toolchain required.
# modernc.org/sqlite (pure-Go SQLite) and the duckdb subprocess wrapper
# make this work. Exporting CGO_ENABLED=0 here propagates to every go
# subcommand below — if a future change pulls in a CGO-only dep,
# `make build` and `make test` fail loudly instead of silently producing
# a CGO-linked binary that won't run on a slim base image.
export CGO_ENABLED := 0

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

tools:
	go install honnef.co/go/tools/cmd/staticcheck@latest
	go install golang.org/x/tools/cmd/deadcode@latest

check: lint
	@if command -v staticcheck >/dev/null 2>&1; then staticcheck ./...; else echo "staticcheck not installed; run 'make tools'"; fi
	@if command -v deadcode >/dev/null 2>&1; then deadcode -test ./...; else echo "deadcode not installed; run 'make tools'"; fi

# verify-pure-go cross-compiles to every supported OS to catch
# platform-specific regressions (e.g. a build-tagged file that only
# exists for one GOOS, or a dep that imports syscall in a non-portable
# way). Run before tagging a release. CGO_ENABLED=0 is already exported
# at the top of the file.
verify-pure-go:
	@set -e; for os in darwin linux windows; do \
	  echo "==> GOOS=$$os"; \
	  GOOS=$$os go build -o /dev/null ./...; \
	done
	@echo "All targets built clean with CGO_ENABLED=0."

clean:
	rm -rf bin/
