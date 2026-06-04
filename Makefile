.PHONY: build test clean help

APP        = iso8583tool
VERSION    = $(shell git describe --tags --abbrev=0 2>/dev/null || echo dev)
GO         = go
GO_BUILD   = $(GO) build
GO_TEST    = $(GO) test
GO_TOOL    = $(GO) tool
GO_PKGROOT = ./...
LDFLAGS    = -s -w -X github.com/nao1215/iso8583tool/cmd.Version=$(VERSION)

build: ## Build binary
	env GO111MODULE=on CGO_ENABLED=0 $(GO_BUILD) -ldflags "$(LDFLAGS)" -o $(APP) main.go

test: ## Run unit tests with coverage output
	$(GO_TEST) -cover -covermode=atomic -coverpkg=$(GO_PKGROOT) -coverprofile=coverage.out $(GO_PKGROOT)
	$(GO_TOOL) cover -html=coverage.out -o coverage.html

clean: ## Clean build and test artifacts
	-rm -rf $(APP) coverage.out coverage.html

.DEFAULT_GOAL := help
help:
	@grep -E '^[0-9a-zA-Z_-]+[[:blank:]]*:.*?## .*$$' $(MAKEFILE_LIST) | sort \
	| awk 'BEGIN {FS = ":.*?## "}; {printf "\033[1;32m%-10s\033[0m %s\n", $$1, $$2}'
