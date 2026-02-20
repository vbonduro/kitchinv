.PHONY: build test test-cover lint vet staticcheck all docker-build docker-up docker-pull-model

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o kitchinv ./cmd/kitchinv

test:
	go test -race -count=1 ./...

test-cover:
	go test -race -count=1 -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run ./...

vet:
	go vet ./...

staticcheck:
	staticcheck ./...

all: lint vet test build

docker-build:
	docker build -t kitchinv:latest .

docker-up:
	docker compose up --build

docker-pull-model:
	docker compose exec ollama ollama pull moondream
