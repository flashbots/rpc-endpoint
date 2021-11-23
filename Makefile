.PHONY: all build test clean lint cover

GOPATH := $(if $(GOPATH),$(GOPATH),~/go)
GIT_VER := $(shell git describe --tags --always --dirty="-dev")

all: clean build

build:
	go build -ldflags "-X main.version=${GIT_VER}" -v -o rpc-endpoint cmd/server/main.go

clean:
	rm -rf rpc-endpoint build/

test:
	go test ./...

lint:
	gofmt -d ./

cover:
	go test -coverprofile=/tmp/go-rpcendpoint.cover.tmp ./...
	go tool cover -html=/tmp/go-rpcendpoint.cover.tmp
	unlink /tmp/go-rpcendpoint.cover.tmp
