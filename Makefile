ifeq ($(OS),Windows_NT)
SHELL := cmd.exe
.SHELLFLAGS := /C
VERSION = $(shell git describe --tags --always --dirty 2>nul || echo dev)
COMMIT = $(shell git rev-parse --short=12 HEAD 2>nul)
DATE = $(shell powershell -NoProfile -Command "[DateTime]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')")
else
SHELL := /bin/bash
VERSION = $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT = $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo "")
DATE = $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
endif
.DEFAULT_GOAL := build

.PHONY: build mog mog-help help fmt fmt-check test ci vendor

BIN_DIR := $(CURDIR)/bin
ifeq ($(OS),Windows_NT)
BIN := $(BIN_DIR)/mog.exe
else
BIN := $(BIN_DIR)/mog
endif
CMD := ./cmd/mog
GO := go
GOFLAGS ?= -mod=vendor
TEST_PKGS := ./cmd/... ./internal/...

LDFLAGS = -X github.com/jaredpalmer/mogcli/internal/cmd.version=$(VERSION) -X github.com/jaredpalmer/mogcli/internal/cmd.commit=$(COMMIT) -X github.com/jaredpalmer/mogcli/internal/cmd.date=$(DATE)

ifneq ($(filter mog,$(MAKECMDGOALS)),)
RUN_ARGS := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
$(eval $(RUN_ARGS):;@:)
endif

build:
ifeq ($(OS),Windows_NT)
	@if not exist "$(BIN_DIR)" mkdir "$(BIN_DIR)"
else
	@mkdir -p $(BIN_DIR)
endif
	@$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN) $(CMD)

mog: build
ifeq ($(OS),Windows_NT)
	@if not "$(RUN_ARGS)"=="" ("$(BIN)" $(RUN_ARGS)) else if "$(ARGS)"=="" ("$(BIN)" --help) else ("$(BIN)" $(ARGS))
else
	@if [ -n "$(RUN_ARGS)" ]; then \
		$(BIN) $(RUN_ARGS); \
	elif [ -z "$(ARGS)" ]; then \
		$(BIN) --help; \
	else \
		$(BIN) $(ARGS); \
	fi
endif

mog-help: build
	@$(BIN) --help

help: mog-help

fmt:
	@gofmt -w cmd internal

fmt-check:
ifeq ($(OS),Windows_NT)
	@powershell -NoProfile -Command "$$files = gofmt -l cmd internal; if ($$files) { Write-Host 'gofmt needs to be run on:'; $$files; Write-Host 'run: make fmt'; exit 1 }"
else
	@files="$$(gofmt -l cmd internal)"; \
	if [ -n "$$files" ]; then \
		echo "gofmt needs to be run on:"; \
		echo "$$files"; \
		echo "run: make fmt"; \
		exit 1; \
	fi
endif

test:
	@$(GO) test $(GOFLAGS) $(TEST_PKGS)

vendor:
	@$(GO) mod vendor

ci: fmt-check test
