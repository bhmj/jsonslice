LDFLAGS ?=-s -w -X main.appVersion=dev-$(shell git rev-parse --short HEAD)-$(shell date +%y-%m-%d)
OUT ?= ./build
PROJECT ?=$(shell basename $(PWD))
SRC ?= ./cmd/$(PROJECT)
BINARY ?= $(OUT)/$(PROJECT)
PREFIX ?= manual

all: configure build lint test

help:
	echo "usage: make <command>"
	echo ""
	echo "  <command> is"
	echo ""
	echo "    configure     - install tools and dependencies"
	echo "    build         - build jsonslice CLI"
	echo "    run           - run jsonslice CLI"
	echo "    lint          - run linters"
	echo "    test          - run tests"
	echo "    test-short    - run tests without fuzzy tests"

configure:
	go install github.com/fzipp/gocyclo/cmd/gocyclo@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

build:
	mkdir -p $(OUT)
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -trimpath -o $(BINARY) $(SRC)

run:
	mkdir -p $(OUT)
	CGO_ENABLED=0 go run -ldflags "$(LDFLAGS)" -trimpath $(SRC)

lint:
	golangci-lint run
	gocyclo -over 18 .

test:
	go test ./...

test-short:
	go test -test.short ./...

.PHONY: all configure help build run lint test test-short

$(V).SILENT: