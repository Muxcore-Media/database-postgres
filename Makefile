# database-postgres
.PHONY: build test lint clean fmt tidy docker ci help

GO ?= go
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.0.0-dev")
LDFLAGS ?= -s -w -X main.version=$(VERSION)
BINARY ?= database-postgres

build:
	$(GO) build -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/module

test:
	$(GO) test -race -count=1 -timeout 120s ./...

lint:
	golangci-lint run --timeout 120s ./...

clean:
	rm -f $(BINARY)

fmt:
	$(GO) fmt ./...

tidy:
	$(GO) mod tidy

docker:
	docker build -t ghcr.io/muxcore-media/$(BINARY):$(VERSION) .

ci: lint test build

help:
	@echo "Targets: build test lint clean fmt tidy docker ci"
