SHELL := /bin/bash
.DEFAULT_GOAL := build

.PHONY: build mog mog-help help fmt test ci

BIN_DIR := $(CURDIR)/bin
BIN := $(BIN_DIR)/mog
CMD := ./cmd/mog

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT := $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo "")
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X github.com/jared/mogcli/internal/cmd.version=$(VERSION) -X github.com/jared/mogcli/internal/cmd.commit=$(COMMIT) -X github.com/jared/mogcli/internal/cmd.date=$(DATE)

ifneq ($(filter mog,$(MAKECMDGOALS)),)
RUN_ARGS := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
$(eval $(RUN_ARGS):;@:)
endif

build:
	@mkdir -p $(BIN_DIR)
	@go build -ldflags "$(LDFLAGS)" -o $(BIN) $(CMD)

mog: build
	@if [ -n "$(RUN_ARGS)" ]; then \
		$(BIN) $(RUN_ARGS); \
	elif [ -z "$(ARGS)" ]; then \
		$(BIN) --help; \
	else \
		$(BIN) $(ARGS); \
	fi

mog-help: build
	@$(BIN) --help

help: mog-help

fmt:
	@gofmt -w cmd internal

test:
	@go test ./...

ci: fmt test
