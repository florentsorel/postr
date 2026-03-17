GO_PACKAGES = $(shell go list ./... | grep -v /web/)

.PHONY: build test generate

test-app:
	go test -v $(GO_PACKAGES)

test-web:
	cd web && npm install && npm test

build:
	go build ./cmd/api

test: test-app test-web

generate:
	sqlc generate
