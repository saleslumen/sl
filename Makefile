.PHONY: build test check fmt publish

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/saleslumen/sl/internal/cli.version=$(VERSION)
GO_TEST_TAGS ?= monorepo
TEST_TAGS := $(addprefix -tags ,$(GO_TEST_TAGS))

build:
	go build -ldflags "$(LDFLAGS)" -o bin/sl .

test:
	go test $(TEST_TAGS) ./...

check:
	gofmt -l .
	@test -z "$$(gofmt -l .)"
	go vet $(TEST_TAGS) ./...
	go test $(TEST_TAGS) ./...

fmt:
	gofmt -w .

publish:
	./.saleslumen/publish
