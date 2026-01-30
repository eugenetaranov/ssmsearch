.PHONY: build install clean test lint fmt deps

BINARY := ssmsearch
BUILD_DIR := bin
VERSION := $(shell git describe --tags --exact-match 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.gitCommit=$(COMMIT)"

build:
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/ssmsearch

install:
	go install $(LDFLAGS) ./cmd/ssmsearch

clean:
	rm -rf $(BUILD_DIR)

test:
	go test -race -v ./...

lint:
	golangci-lint run ./...

fmt:
	go fmt ./...

deps:
	go mod download
	go mod tidy
