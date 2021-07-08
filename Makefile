PKGS := $(shell go list ./... | grep -v vendor | grep -v service)
SRC := $(shell find . \( -iname '*.go' ! -iname "*test.go" \))
GOPATH := $(shell go env GOPATH)
GOROOT := $(shell go env GOROOT)

.PHONY: test

region-api: $(SRC)
	GO111MODULE=on GOPATH=$(GOPATH) GOROOT=$(GOROOT) go build -v -o region-api .

clean:
	GO111MODULE=on GOPATH=$(GOPATH) GOROOT=$(GOROOT) go clean
	-rm region-api

test:
	GO111MODULE=on GOPATH=$(GOPATH) GOROOT=$(GOROOT) go test -v $(PKGS)
