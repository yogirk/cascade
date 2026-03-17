.PHONY: build test test-short lint clean

build:
	go build -o bin/cascade ./cmd/cascade/

test:
	go test ./... -count=1 -race

test-short:
	go test ./internal/... -short -count=1

lint:
	go vet ./...

clean:
	rm -rf bin/
