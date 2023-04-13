.PHONY: all build test clean lint cover cover-html up down

APP_NAME := rpc-endpoint
GOPATH := $(if $(GOPATH),$(GOPATH),~/go)
GIT_VER := $(shell git describe --tags --always --dirty="-dev")
PACKAGES := $(shell go list -mod=readonly ./...)

all: clean build

build:
	CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.version=${GIT_VER}" -v -o ${APP_NAME} cmd/server/main.go

clean:
	rm -rf ${APP_NAME} build/

test:
	go test ./...

gofmt:
	gofmt -w ./

lint:
	go fmt -mod=readonly $(PACKAGES)
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
