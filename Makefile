.PHONY: all build test up down migrate_up migrate_down clean lint cover cover-html

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
	go vet ./...
	staticcheck ./...

cover:
	go test -coverpkg=github.com/flashbots/rpc-endpoint/server,github.com/flashbots/rpc-endpoint/types,github.com/flashbots/rpc-endpoint/utils -coverprofile=/tmp/go-rpcendpoint.cover.tmp ./...
	go tool cover -func /tmp/go-rpcendpoint.cover.tmp
	unlink /tmp/go-rpcendpoint.cover.tmp

cover-html:
	go test -coverpkg=github.com/flashbots/rpc-endpoint/server,github.com/flashbots/rpc-endpoint/types,github.com/flashbots/rpc-endpoint/utils -coverprofile=/tmp/go-rpcendpoint.cover.tmp ./...
	go tool cover -html=/tmp/go-rpcendpoint.cover.tmp
	unlink /tmp/go-rpcendpoint.cover.tmp

up:
	docker-compose up -d

down:
	docker-compose down -v
