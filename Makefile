# Makefile for the Linux/amd64 specific parts of the kernel

.PHONY: build

build:
    GOOS=linux GOARCH=amd64 go mod tidy
    GOOS=linux GOARCH=amd64 go build -o ./bin/cis ./cmd/cis/main.go

clean:
    GOOS=linux GOARCH=amd64 rm -rf ./bin/cis