include ../mk/common.mk

SERVICE := gateway-api
BIN := $(BUILD_DIR)/$(SERVICE)

.PHONY: gen lint test build run tidy

gen:
	$(OAPI_CODEGEN) --config api/oapi-codegen.yaml

lint:
	$(GOLANGCI_LINT) run ./...

test:
	$(GOTEST) ./...

build:
	$(GOBUILD) -o $(BIN) ./cmd/server

run: build
	./$(BIN)

tidy:
	$(GOTIDY)
