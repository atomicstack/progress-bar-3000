ROOT_DIR := $(abspath $(dir $(firstword $(MAKEFILE_LIST))))
BIN_NAME := progress-bar-3000
DIST_DIR := $(ROOT_DIR)/dist

export GOCACHE    := $(ROOT_DIR)/.gocache
export GOMODCACHE := $(ROOT_DIR)/.gomodcache

GO ?= go

.PHONY: all build test vet tidy smoke run clean help

all: build test

build:
	mkdir -p $(DIST_DIR)
	$(GO) build -o $(DIST_DIR)/$(BIN_NAME) .

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

tidy:
	$(GO) mod tidy

smoke: build
	./scripts/smoke.sh

run: build
	$(DIST_DIR)/$(BIN_NAME)

clean:
	rm -rf $(DIST_DIR)

help:
	@echo "targets:"
	@echo "  build  - compile binary into ./dist"
	@echo "  test   - run the go test suite"
	@echo "  vet    - run go vet"
	@echo "  tidy   - run go mod tidy"
	@echo "  smoke  - build and run scripts/smoke.sh"
	@echo "  run    - build and run the binary"
	@echo "  clean  - remove ./dist"
