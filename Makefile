VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  = -s -w -X github.com/lanesket/llm.log/internal/cli.Version=$(VERSION)

.PHONY: build test lint clean setup-hooks build-ui dev-ui

build:
	go build -ldflags "$(LDFLAGS)" -o llm-log ./cmd/llm-log

test:
	go test ./...

lint:
	go vet ./...

clean:
	rm -f llm-log

setup-hooks:
	git config core.hooksPath .githooks

build-ui:
	cd web && npm ci && npm run build
	go build -ldflags "$(LDFLAGS)" -o llm-log ./cmd/llm-log

dev-ui:
	go run -ldflags "$(LDFLAGS)" ./cmd/llm-log ui --dev
