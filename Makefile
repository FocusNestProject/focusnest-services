.PHONY: lint lint-fix

SERVICES = chatbot-service focus-service gateway-api progress-service user-service shared-libs

lint:
	@echo "Running golangci-lint for all services..."
	@for service in $(SERVICES); do \
		echo "\n==================== Linting $$service ===================="; \
		(cd $$service && golangci-lint run ./...); \
	done

lint-fix:
	@echo "Running golangci-lint with auto-fix..."
	@for service in $(SERVICES); do \
		echo "\n==================== Fixing $$service ===================="; \
		(cd $$service && golangci-lint run --fix ./...); \
	done
