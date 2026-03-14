GO_PACKAGES = $(shell go list ./... | grep -v /web/)

.PHONY: build test generate

build:
	go build ./cmd/api

test:
	go test $(GO_PACKAGES)

generate:
	sqlc generate
