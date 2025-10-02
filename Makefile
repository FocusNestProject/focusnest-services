include mk/common.mk

.PHONY: e2e lint test tidy generate

e2e:
	bash ./scripts/e2e.sh

lint:
	$(GOLANGCI_LINT) run ./...

test:
	go test ./...

tidy:
	go work sync
