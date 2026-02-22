.PHONY: build run test clean docker-build docker-push deploy-local deploy-k8s help

BINARY_NAME=kahook
DOCKER_IMAGE=kahook
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.1.0")
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS=-ldflags "-w -s -X github.com/kahook/internal/version.Version=$(VERSION) -X github.com/kahook/internal/version.BuildTime=$(BUILD_TIME) -X github.com/kahook/internal/version.GitCommit=$(GIT_COMMIT)"

help:
	@echo "Kahook - Webhook to Kafka bridge"
	@echo ""
	@echo "Usage:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'

## build: Build the binary
build:
	CGO_ENABLED=1 go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/server

## run: Run locally with default config
run:
	go run ./cmd/server

## run-confluent: Run locally with Confluent Cloud config
run-confluent:
	CONFIG_PATH=config.confluent.yaml go run ./cmd/server

## test: Run tests (excludes internal/kafka which requires a live broker)
test:
	CGO_ENABLED=1 go test -v -coverprofile=coverage.out \
		$(shell go list ./... | grep -v 'internal/kafka')

## coverage: View test coverage
coverage: test
	go tool cover -html=coverage.out

## clean: Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out

## lint: Run linters
lint:
	golangci-lint run ./...

## fmt: Format code
fmt:
	go fmt ./...

## vet: Run go vet
vet:
	go vet ./...

## mod: Download and tidy dependencies
mod:
	go mod download
	go mod tidy

## docker-build: Build Docker image
docker-build:
	docker build -t $(DOCKER_IMAGE):$(VERSION) -t $(DOCKER_IMAGE):latest .

## docker-push: Push Docker image to registry
docker-push:
	docker push $(DOCKER_IMAGE):$(VERSION)
	docker push $(DOCKER_IMAGE):latest

## docker-run: Run Docker container locally
docker-run:
	docker run -p 8080:8080 $(DOCKER_IMAGE):latest

## compose-up: Start local development stack
compose-up:
	docker-compose up -d

## compose-down: Stop local development stack
compose-down:
	docker-compose down

## compose-logs: View docker-compose logs
compose-logs:
	docker-compose logs -f kahook

## kafka-topics: List Kafka topics
kafka-topics:
	docker-compose exec kafka kafka-topics --bootstrap-server localhost:9092 --list

## kafka-consume: Consume from a topic (usage: make kafka-consume TOPIC=my-topic)
kafka-consume:
	docker-compose exec kafka kafka-console-consumer --bootstrap-server localhost:9092 --topic $(TOPIC) --from-beginning

## deploy-k8s: Deploy to Kubernetes
deploy-k8s:
	kubectl apply -f deploy/kubernetes/

## delete-k8s: Delete from Kubernetes
delete-k8s:
	kubectl delete -f deploy/kubernetes/

## logs-k8s: View Kubernetes logs
logs-k8s:
	kubectl logs -f -l app=kahook -n kahook

## all: Run fmt, vet, test, and build
all: fmt vet test build
