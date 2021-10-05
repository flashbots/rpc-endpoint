.PHONY: all test clean

GOPATH := $(if $(GOPATH),$(GOPATH),~/go)

all:
	go build -v -o rpc-endpoint

clean:
	rm -rf rpc-endpoint build/
