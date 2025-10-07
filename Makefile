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

.PHONY: docs swagger

docs:
	@echo "Merging OpenAPI specs into swagger-ui/combined.yaml"
	docker run --rm -e MERGE_DEFAULT_SERVER=$(SERVER) -v $(PWD):/work -w /work python:3.11-alpine sh -lc "pip install pyyaml >/dev/null 2>&1 && python scripts/merge_openapi.py swagger-ui/combined.yaml gateway-api/api/openapi.yaml activity-service/api/openapi.yaml user-service/api/openapi.yaml session-service/api/openapi.yaml media-service/api/openapi.yaml analytics-service/api/openapi.yaml webhook-service/api/openapi.yaml"
	@echo "Done. Output: swagger-ui/combined.yaml"

swagger:
	@cd swagger-ui && docker-compose up -d
