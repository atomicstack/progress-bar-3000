ROOT_DIR := $(abspath $(dir $(firstword $(MAKEFILE_LIST))))
BIN_NAME := progress-bar-3000
BIN_PATH := $(ROOT_DIR)/$(BIN_NAME)

export GOCACHE    := $(ROOT_DIR)/.gocache
export GOMODCACHE := $(ROOT_DIR)/.gomodcache

GO ?= go

.PHONY: all build test vet tidy smoke run clean help

all: build test

build:
	$(GO) build -o $(BIN_PATH) .

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

tidy:
	$(GO) mod tidy

smoke: build
	./scripts/smoke.sh

run: build
	$(BIN_PATH)

clean:
	rm -f $(BIN_PATH)

help:
	@echo "targets:"
	@echo "  build  - compile binary at the repo root"
	@echo "  test   - run the go test suite"
	@echo "  vet    - run go vet"
	@echo "  tidy   - run go mod tidy"
	@echo "  smoke  - build and run scripts/smoke.sh"
	@echo "  run    - build and run the binary"
	@echo "  clean  - remove the built binary"
