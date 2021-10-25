.PHONY: all build test clean lint

GOPATH := $(if $(GOPATH),$(GOPATH),~/go)
GIT_VER := $(shell git describe --tags)

all: clean build

build:
	go build -ldflags "-X main.version=${GIT_VER}" -v -o rpc-endpoint

clean:
	rm -rf rpc-endpoint build/

test:
	go test test/*

lint:
	gofmt -d ./
