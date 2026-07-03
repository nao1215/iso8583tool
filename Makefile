.PHONY: build test e2e lint tools demo clean help

APP        = iso8583tool
VERSION    = $(shell git describe --tags --abbrev=0 2>/dev/null || echo dev)
GO         = go
GO_BUILD   = $(GO) build
GO_TEST    = $(GO) test
GO_TOOL    = $(GO) tool
GO_INSTALL = $(GO) install
GO_PKGROOT = ./...
LDFLAGS    = -s -w -X github.com/nao1215/iso8583tool/cmd.Version=$(VERSION)

build: ## Build binary
	env GO111MODULE=on CGO_ENABLED=0 $(GO_BUILD) -ldflags "$(LDFLAGS)" -o $(APP) main.go

test: ## Run unit tests with coverage output
	$(GO_TEST) -cover -covermode=atomic -coverpkg=$(GO_PKGROOT) -coverprofile=coverage.out $(GO_PKGROOT)
	$(GO_TOOL) cover -html=coverage.out -o coverage.html

e2e: ## Run atago end-to-end tests against the freshly built binary
	./e2e/run.sh

lint: ## Run golangci-lint
	golangci-lint run --config .golangci.yml

tools: ## Install developer tools (linter, coverage, atago for e2e)
	$(GO_INSTALL) github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	$(GO_INSTALL) github.com/k1LoW/octocov@latest
	$(GO_INSTALL) github.com/nao1215/atago@latest

demo: build ## Regenerate the README GIF from docs/demo.tape (needs vhs)
	@command -v vhs >/dev/null || { echo 'vhs is required: go install github.com/charmbracelet/vhs@latest'; exit 1; }
	@cp examples/basei/0100-auth-request.hex /tmp/before.hex
	@./$(APP) convert examples/basei/0100-auth-request.hex \
		| sed -e 's/4111111111111111/4222222222222222/g' -e 's/000000005000/000000009999/g' \
		| ./$(APP) convert > /tmp/after.hex
	@for tape in docs/*.tape; do vhs "$$tape"; done
	@echo 'Regenerated docs/*.gif'

clean: ## Clean build and test artifacts
	-rm -rf $(APP) coverage.out coverage.html

.DEFAULT_GOAL := help
help:
	@grep -E '^[0-9a-zA-Z_-]+[[:blank:]]*:.*?## .*$$' $(MAKEFILE_LIST) | sort \
	| awk 'BEGIN {FS = ":.*?## "}; {printf "\033[1;32m%-12s\033[0m %s\n", $$1, $$2}'
