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
migrate_up:
	docker run -v ${CURDIR}/sql:/migrations --network host migrate/migrate -path=/migrations/ -database 'postgres://postgres:postgres@localhost:5432/test?sslmode=disable&search_path=requests' up
migrate_down:
	docker run -v ${CURDIR}/sql:/migrations --network host migrate/migrate -path=/migrations/ -database 'postgres://postgres:postgres@localhost:5432/test?sslmode=disable&search_path=requests' down -all
up:
	docker run --rm --name pg-rpc-endpoint -e POSTGRES_DB=test -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=postgres -d -p 5432:5432 -v ${HOME}/docker/volumes/postgres:/var/lib/postgresql/data  -t postgres
	@make migrate_up
down:
	@make migrate_down
	docker stop pg-rpc-endpoint
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
