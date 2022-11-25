.PHONY: all build test clean lint cover cover-html up down
include .env
APP_NAME := rpc-endpoint
GOPATH := $(if $(GOPATH),$(GOPATH),~/go)
COMMIT := $(shell git rev-parse --short HEAD)
DATE := $(shell git log -1 --format=%cd --date=format:"%Y%m%d")
DIFF_HASH := $(shell git diff HEAD -- . | sha1sum | head -c 7)
EMPTY_DIFF_HASH := da39a3e

GIT_VER := $(COMMIT)-$(DATE)
ifneq ($(DIFF_HASH), $(EMPTY_DIFF_HASH))
	GIT_VER := $(GIT_VER)-$(DIFF_HASH)
endif

all: clean build

build:
	CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.version=${GIT_VER}" -v -o ${APP_NAME} cmd/server/main.go

clean:
	rm -rf ${APP_NAME} build/

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

build-for-docker:
	CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.version=${GIT_VER}" -v -o ${APP_NAME} cmd/server/main.go

docker-image:
	DOCKER_BUILDKIT=1 docker build --platform linux/amd64  . -t ${IMAGE_REGISTRY_ACCOUNT}/${APP_NAME}:${GIT_VER}
