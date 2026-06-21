.PHONY: build install uninstall test run fmt

build:
	mkdir -p bin
	go build -o bin/ibot ./cmd/ibot

install:
	go install ./cmd/ibot

uninstall:
	rm -f $$(go env GOPATH)/bin/ibot

test:
	go test ./...

run:
	go run ./cmd/ibot serve

fmt:
	gofmt -w cmd internal
