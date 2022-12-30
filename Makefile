export GOBIN ?= $(shell pwd)/bin
export PATH := $(GOBIN):$(PATH)

STATICCHECK = bin/staticcheck
REVIVE = bin/revive
DOC2GO = bin/doc2go

GO_FILES = $(shell find . \
	   -path '*/.*' -prune -o \
	   '(' -type f -a -name '*.go' ')' -print)

GO_SRC_FILES = $(filter-out %_test.go,$(GO_FILES))

.PHONY: all
all: build lint test

.PHONY: build
build: $(DOC2GO)

$(DOC2GO): $(GO_SRC_FILES)
	go install go.abhg.dev/doc2go

.PHONY: lint
lint: gofmt staticcheck revive

.PHONY: gofmt
gofmt:
	$(eval FMT_LOG := $(shell mktemp -t gofmt.XXXXX))
	@gofmt -e -s -l $(GO_FILES) > $(FMT_LOG) || true
	@[ ! -s "$(FMT_LOG)" ] || \
		(echo "gofmt failed. Please reformat the following files:" | \
		cat - $(FMT_LOG) && false)

.PHONY: staticcheck
staticcheck: $(STATICCHECK)
	staticcheck ./...

.PHONY: revive
revive: $(REVIVE)
	revive -config revive.toml ./...

.PHONY: test
test:
	go test -v ./...

.PHONY: cover
cover:
	go test -coverprofile=cover.out -coverpkg=./... ./...
	go tool cover -html=cover.out -o cover.html

$(STATICCHECK): tools/go.mod
	cd tools && go install honnef.co/go/tools/cmd/staticcheck

$(REVIVE): tools/go.mod
	cd tools && go install github.com/mgechev/revive
