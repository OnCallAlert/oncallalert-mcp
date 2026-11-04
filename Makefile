.PHONY: build test lint fmt
export GOWORK := off

VERSION ?= dev

build:
	go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o oncallalert-mcp .

test:
	go test -count=1 ./...

lint:
	go vet ./...
	@test -z "$$(gofmt -l .)" || (gofmt -l . && echo "gofmt needed" && exit 1)

fmt:
	gofmt -w .
