GO ?= go
GOCMD ?= $(GO)
GOBUILD ?= $(GOCMD) build
GOTEST ?= $(GOCMD) test
GOMOD ?= $(GOCMD) mod
GOGEN ?= $(GOCMD) generate
GOTIDY ?= $(GOMOD) tidy
GOCLEAN ?= $(GOCMD) clean

BIN_DIR ?= bin
BUILD_DIR ?= $(BIN_DIR)

GOLANGCI_LINT ?= golangci-lint
OAPI_CODEGEN ?= oapi-codegen

.DEFAULT_GOAL := build
