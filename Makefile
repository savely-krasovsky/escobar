PROJECT_NAME := escobar
PROJECT := github.com/L11R/escobar
VERSION := $(shell cat version)
COMMIT := $(shell git rev-parse --short HEAD)
PKG_LIST := $(shell go list ./... | grep -v /vendor/)

GOLANGCI_LINT_VERSION = v1.21.0

LDFLAGS = "-s -w -X $(PROJECT)/internal/version.Version=$(VERSION) -X $(PROJECT)/internal/version.Commit=$(COMMIT)"

build:
	go build -ldflags $(LDFLAGS) -o ./bin/$(PROJECT_NAME) ./cmd/$(PROJECT_NAME)

cross-build:
	GOOS=linux GOARCH=amd64 go build -ldflags $(LDFLAGS) -o ./bin/$(PROJECT_NAME).linux ./cmd/$(PROJECT_NAME)
	GOOS=darwin GOARCH=amd64 go build -ldflags $(LDFLAGS) -o ./bin/$(PROJECT_NAME).darwin ./cmd/$(PROJECT_NAME)
	GOOS=windows GOARCH=amd64 go build -ldflags $(LDFLAGS) -o ./bin/$(PROJECT_NAME).exe ./cmd/$(PROJECT_NAME)

test:
	@go test -v -cover -gcflags=-l --race $(PKG_LIST)

lint:
	@golangci-lint run -v
