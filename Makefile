SHELL = /bin/bash

PROJECT_ROOT = $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

GO_MODULES ?= $(shell find . \
	-path '*/.*' -prune -o \
	-type f -a -name 'go.mod' -exec dirname '{}' ';')

# ./docs doesn't have any meaningful Go code.
GO_MODULES := $(filter-out ./docs, $(GO_MODULES))


# Setting GOBIN and PATH ensures two things:
# - All 'go install' commands we run
#   only affect the current directory.
# - All installed tools are available on PATH
#   for commands like go generate.
export GOBIN ?= $(PROJECT_ROOT)/bin
export PATH := $(GOBIN):$(PATH)

TEST_FLAGS ?= -race

# Non-test Go files.
GO_SRC_FILES = $(shell find . \
	   -path '*/.*' -prune -o \
	   '(' -type f -a -name '*.go' -a -not -name '*_test.go' ')' -print)

DOC2GO = bin/doc2go

.PHONY: all
all: build lint test

.PHONY: build
build: $(DOC2GO)

$(DOC2GO): $(GO_SRC_FILES) $(wildcard ./internal/html/tmpl/*)
	go install go.abhg.dev/doc2go

.PHONY: lint
lint: golangci-lint tidy-lint

.PHONY: test
test:
	go test $(TEST_FLAGS) ./...

.PHONY: test-integration
test-integration: $(DOC2GO)
	go test -C integration $(TEST_FLAGS) \
		-doc2go $(shell pwd)/$(DOC2GO) -rundir $(PROJECT_ROOT)

.PHONY: cover
cover:
	go test $(TEST_FLAGS) -coverprofile=cover.out -coverpkg=./... ./...
	go tool cover -html=cover.out -o cover.html

.PHONY: cover-integration
cover-integration: export GOEXPERIMENT = coverageredesign
cover-integration:
	$(eval BIN := $(shell mktemp -d))
	$(eval COVERDIR := $(shell mktemp -d))
	GOBIN=$(BIN) \
 		go install -race -cover -coverpkg=./... go.abhg.dev/doc2go
	GOCOVERDIR=$(COVERDIR) PATH=$(BIN):$$PATH \
		go test -C integration $(TEST_FLAGS) \
		-doc2go $(BIN)/doc2go -rundir $(PROJECT_ROOT)
	go tool covdata textfmt -i=$(COVERDIR) -o=cover.integration.out
	go tool cover -html=cover.integration.out -o cover.integration.html

.PHONY: tidy
tidy:
	$(foreach mod,$(GO_MODULES),(cd $(mod) && go mod tidy) &&) true

.PHONY: golangci-lint
golangci-lint:
	$(foreach mod,$(GO_MODULES), \
		(cd $(mod) && golangci-lint run --path-prefix $(mod)) &&) true

.PHONY: tidy-lint
tidy-lint:
	$(foreach mod,$(GO_MODULES), \
		(cd $(mod) && go mod tidy && \
			git diff --exit-code -- go.mod go.sum || \
			(echo "[$(mod)] go mod tidy changed files" && false)) &&) true
