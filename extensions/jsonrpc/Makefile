SHELL := /bin/sh

APP := assets-rpc
CMD := ./cmd/assets-rpc
BIN_DIR ?= ../../bin
DIST_DIR ?= dist
GO ?= go
GOOS ?= $(shell $(GO) env GOOS)
GOARCH ?= $(shell $(GO) env GOARCH)
SYNC_ARGS ?=
EXT :=

ifeq ($(GOOS),windows)
	EXT := .exe
endif

.PHONY: all build build-target build-all test tidy run sync-once clean help

all: test build

build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/$(APP)$(EXT) $(CMD)

build-target:
	@mkdir -p $(DIST_DIR)
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build -o $(DIST_DIR)/$(APP)-$(GOOS)-$(GOARCH)$(EXT) $(CMD)

build-all:
	$(MAKE) build-target GOOS=linux GOARCH=amd64 EXT=
	$(MAKE) build-target GOOS=linux GOARCH=arm64 EXT=
	$(MAKE) build-target GOOS=darwin GOARCH=amd64 EXT=
	$(MAKE) build-target GOOS=darwin GOARCH=arm64 EXT=
	$(MAKE) build-target GOOS=windows GOARCH=amd64 EXT=.exe

test:
	$(GO) test ./...

tidy:
	$(GO) mod tidy

run:
	$(GO) run $(CMD) --root ../.. --addr :8080

sync-once:
	$(GO) run $(CMD) --root ../.. --sync-once $(SYNC_ARGS)

clean:
	rm -rf $(DIST_DIR)
	rm -f $(BIN_DIR)/$(APP) $(BIN_DIR)/$(APP).exe

help:
	@echo "Targets:"
	@echo "  make build       Build for current platform into ../../bin"
	@echo "  make build-all   Build linux/darwin/windows release binaries into dist/"
	@echo "  make test        Run tests"
	@echo "  make run         Start JSON-RPC server on :8080"
	@echo "  make sync-once   Generate JSON caches and exit"
	@echo "Variables:"
	@echo "  GOOS=$(GOOS) GOARCH=$(GOARCH) BIN_DIR=$(BIN_DIR) DIST_DIR=$(DIST_DIR)"
	@echo "  SYNC_ARGS=$(SYNC_ARGS)"
