include ../mk/common.mk

.PHONY: gen lint test tidy

gen:
	@echo "No code generation necessary for shared-libs"

lint:
	$(GOLANGCI_LINT) run ./...

test:
	$(GOTEST) ./...

tidy:
	$(GOTIDY)
