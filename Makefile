.PHONY: all test clean

GOBASE := $(shell pwd)
GOBIN := $(GOBASE)/bin
GOPATH := $(if $(GOPATH),$(GOPATH),~/go)

all:
	mkdir -p $(GOBIN)
	go build -v -o $(GOBIN)

clean:
	rm -rf $(GOBIN)/*
