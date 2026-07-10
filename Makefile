APPNAME = desafio-chat-ia
VERSION ?= dev

.PHONY: help run test test-race build docker-build docker-run sec-check

help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_0-9-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

run: ## Run the application locally
	go run main.go

test: ## Run all tests
	go test ./...

test-race: ## Run all tests with race detector
	go test -race ./...

build: ## Build the binary to bin/
	go build -o bin/$(APPNAME) main.go

docker-build: ## Build Docker image
	docker build -t $(APPNAME):$(VERSION) .

docker-run: ## Run Docker container (reads from .env)
	docker run --env-file .env -p 8000:8000 $(APPNAME):$(VERSION)

sec-check: ## Run gosec (source) and trivy (filesystem via container) security scans
	gosec ./...
	docker run --rm \
		-v $(PWD):/work \
		aquasec/trivy:latest fs /work
