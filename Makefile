.PHONY: all build test clean lint cover cover-html up down

GOPATH := $(if $(GOPATH),$(GOPATH),~/go)
ACCOUNT_ID=12345 # use proper account id
ECR_URI := ${ACCOUNT_ID}.dkr.ecr.us-east-2.amazonaws.com/rpc-endpoint
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
	CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.version=${GIT_VER}" -v -o rpc-endpoint cmd/server/main.go

clean:
	rm -rf rpc-endpoint build/

test:
	go test ./...

lint:
	gofmt -d ./
	go vet -unreachable=false ./... # go vet checks are disabled for unreachable code
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
	CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.version=${GIT_VER}" -v -o rpc-endpoint cmd/server/main.go

docker-image:
	DOCKER_BUILDKIT=1 docker build --platform=linux/amd64 . -t rpc-endpoint
	docker tag rpc-endpoint:latest ${ECR_URI}:${GIT_VER}
	docker tag rpc-endpoint:latest ${ECR_URI}:latest
