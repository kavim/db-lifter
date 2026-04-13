BINARY_NAME := bd-lifter
BUILD_DIR   := bin
VERSION     := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS     := -s -w -X main.version=$(VERSION)

.PHONY: build run clean lint

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/db-lift

run:
	go run ./cmd/db-lift $(ARGS)

clean:
	rm -rf $(BUILD_DIR)

lint:
	golangci-lint run ./...
