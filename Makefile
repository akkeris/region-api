PKGS := $(shell go list ./... | grep -v vendor | grep -v service)
GOPATH := $(shell go env GOPATH)
GOROOT := $(shell go env GOROOT)

.PHONY: test
server: *.go
	GO111MODULE=on GOPATH=$(GOPATH) GOROOT=$(GOROOT) go build -v .
clean:
	GO111MODULE=on GOPATH=$(GOPATH) GOROOT=$(GOROOT) go clean
test:
	GO111MODULE=on GOPATH=$(GOPATH) GOROOT=$(GOROOT) go test -v $(PKGS)