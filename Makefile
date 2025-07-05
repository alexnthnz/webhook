.PHONY: proto-gen proto-clean build test docker-build docker-up docker-down deps-install

# Protocol Buffer Generation
proto-gen:
	@echo "Generating protobuf code..."
	@mkdir -p proto/generated
	@protoc --go_out=proto/generated --go_opt=paths=source_relative \
		--go-grpc_out=proto/generated --go-grpc_opt=paths=source_relative \
		proto/*.proto
	@echo "Protobuf generation complete"

proto-clean:
	@echo "Cleaning generated protobuf code..."
	@rm -rf proto/generated
	@echo "Protobuf cleanup complete"

# Dependencies
deps-install:
	@echo "Installing Go dependencies..."
	@go mod download
	@go mod tidy
	@echo "Dependencies installed"

# Build
build: proto-gen
	@echo "Building all services..."
	@go build -o bin/api-gateway ./cmd/api-gateway
	@go build -o bin/webhook-registry ./cmd/webhook-registry
	@go build -o bin/event-ingestion ./cmd/event-ingestion
	@go build -o bin/webhook-dispatcher ./cmd/webhook-dispatcher
	@go build -o bin/retry-manager ./cmd/retry-manager
	@go build -o bin/observability ./cmd/observability
	@echo "Build complete"

# Test
test:
	@echo "Running tests..."
	@go test -v ./...
	@echo "Tests complete"

# Docker
docker-build:
	@echo "Building Docker images..."
	@docker build -t webhook-service:latest .
	@echo "Docker build complete"

docker-up:
	@echo "Starting services with Docker Compose..."
	@docker-compose up -d
	@echo "Services started"

docker-down:
	@echo "Stopping services..."
	@docker-compose down
	@echo "Services stopped"

docker-logs:
	@docker-compose logs -f

# Development
dev-setup: deps-install proto-gen
	@echo "Development setup complete"

clean: proto-clean
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@echo "Cleanup complete"

# Database migrations
migrate-up:
	@echo "Running database migrations..."
	@migrate -path ./migrations -database "postgres://postgres:postgres123@localhost:5432/webhook_db?sslmode=disable" up
	@echo "Migrations complete"

migrate-down:
	@echo "Rolling back database migrations..."
	@migrate -path ./migrations -database "postgres://postgres:postgres123@localhost:5432/webhook_db?sslmode=disable" down
	@echo "Rollback complete"

# Help
help:
	@echo "Available targets:"
	@echo "  proto-gen      - Generate protobuf code"
	@echo "  proto-clean    - Clean generated protobuf code"
	@echo "  deps-install   - Install Go dependencies"
	@echo "  build          - Build all services"
	@echo "  test           - Run tests"
	@echo "  docker-build   - Build Docker images"
	@echo "  docker-up      - Start services with Docker Compose"
	@echo "  docker-down    - Stop services"
	@echo "  docker-logs    - View service logs"
	@echo "  dev-setup      - Setup development environment"
	@echo "  clean          - Clean build artifacts"
	@echo "  migrate-up     - Run database migrations"
	@echo "  migrate-down   - Rollback database migrations"
	@echo "  help           - Show this help message" 