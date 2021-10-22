.PHONY: all test clean lint

GOPATH := $(if $(GOPATH),$(GOPATH),~/go)

all:
	go build -v -o rpc-endpoint

clean:
	rm -rf rpc-endpoint build/

test:
	go test test/*

lint:
	gofmt -d ./
