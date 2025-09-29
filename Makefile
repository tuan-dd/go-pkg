# WORKER_IMAGE=1.24.0-alpine3.21

DIRS=$(find . -mindepth 1 -maxdepth 2 -type d ! -name 'no_get')

# DONT_STOP := db redis

# .PHONY: tsl-generate build-services stop-services start-services reset-services doc-generate swagger-2-to-3

go-lint-install:
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	@cp hooks/pre-commit .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit


go-lint:
	golangci-lint run ./...

go-init:
	make go-lint-install
	go mod tidy
	@echo "Finished go-init"


all-mod-tidy:
	@find . -mindepth 1 -maxdepth 2 -type d | while read dir; do \
		if [ -f "$$dir/go.mod" ]; then \
			echo "Running go mod tidy in $$dir"; \
			( cd "$$dir" && go get github.com/tuan-dd/go-common && go mod tidy ); \
		else \
			echo "Skipping $$dir (no go.mod)"; \
		fi; \
	done