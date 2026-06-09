BINARY := portview
CMD    := ./cmd/portview

.PHONY: build run test test-integration lint fmt tidy install clean

build: ## Build the binary into bin/
	go build -o bin/$(BINARY) $(CMD)

run: ## Run portview from source
	go run $(CMD)

test: ## Run unit tests
	go test ./...

test-integration: ## Run tests including the integration suite
	go test -tags integration ./...

lint: ## Run golangci-lint
	golangci-lint run

fmt: ## Format the code
	golangci-lint fmt

tidy: ## Tidy go.mod/go.sum
	go mod tidy

install: ## Install portview into GOBIN
	go install $(CMD)

clean: ## Remove build artifacts
	rm -rf bin/ dist/
