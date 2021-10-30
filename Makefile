LDFLAGS ?=-s -w -X main.appVersion=dev-$(shell git rev-parse --short HEAD)-$(shell date +%y-%m-%d)
OUT ?= ./build
PROJECT ?=$(shell basename $(PWD))
SRC ?= ./cmd/$(PROJECT)
BINARY ?= $(OUT)/$(PROJECT)
PREFIX ?= manual

all: configure build lint test

configure:
	go install github.com/fzipp/gocyclo/cmd/gocyclo@latest

build:
	mkdir -p $(OUT)
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -trimpath -o $(BINARY) $(SRC)

run:
	mkdir -p $(OUT)
	CGO_ENABLED=0 go run -ldflags "$(LDFLAGS)" -trimpath $(SRC)

lint:
	golangci-lint run
	gocyclo

test: 
	go test ./...

.PHONY: all build run lint test

$(V).SILENT: