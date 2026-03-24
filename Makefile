APP_NAME := kintsugi-storage

.PHONY: help
help:
	@echo "Available commands:"
	@echo "  make run           - run application locally"
	@echo "  make build         - build binary"
	@echo "  make lint          - run golangci-lint"
	@echo "  make test          - run tests"
	@echo "  make test-race     - run tests with race detector"
	@echo "  make fmt           - format code"
	@echo "  make tidy          - tidy go modules"
	@echo "  make docker-build  - build docker image"
	@echo "  make compose-up    - start with docker compose"
	@echo "  make compose-down  - stop docker compose"

.PHONY: run
run:
	go run ./cmd/server

.PHONY: build
build:
	go build -o bin/$(APP_NAME) ./cmd/server

.PHONY: lint
lint:
	golangci-lint run

.PHONY: test
test:
	go test ./...

.PHONY: test-race
test-race:
	go test -race ./...

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: docker-build
docker-build:
	docker build -f build/Dockerfile -t $(APP_NAME):local .

.PHONY: compose-up
compose-up:
	docker compose up --build

.PHONY: compose-down
compose-down:
	docker compose down