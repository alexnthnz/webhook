# Multi-stage Dockerfile for Webhook Service

# Build stage - use debian-based image for better C library compatibility
FROM golang:1.23.0-bullseye AS builder

# Install dependencies
RUN apt-get update && apt-get install -y \
    git \
    protobuf-compiler \
    libprotobuf-dev \
    make \
    build-essential \
    librdkafka-dev \
    && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Generate protobuf code
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest && \
    export PATH=$PATH:$(go env GOPATH)/bin && \
    make proto-gen

# Build all services
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o bin/api-gateway ./cmd/api-gateway && \
    CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o bin/webhook-registry ./cmd/webhook-registry && \
    CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o bin/event-ingestion ./cmd/event-ingestion && \
    CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o bin/webhook-dispatcher ./cmd/webhook-dispatcher && \
    CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o bin/retry-manager ./cmd/retry-manager && \
    CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o bin/observability ./cmd/observability && \
    CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o bin/dlq ./cmd/dlq

# Base runtime image - use debian slim for better C library compatibility
FROM debian:bullseye-slim AS base

# Install ca-certificates for HTTPS requests and librdkafka runtime
RUN apt-get update && apt-get install -y \
    ca-certificates \
    tzdata \
    librdkafka1 \
    wget \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN groupadd -g 1001 webhook && \
    useradd -m -s /bin/bash -u 1001 -g webhook webhook

# Create app directory
WORKDIR /app

# Copy configuration files
COPY config/ ./config/
COPY migrations/ ./migrations/
COPY config.yaml .

# API Gateway service
FROM base AS api-gateway
COPY --from=builder /app/bin/api-gateway .
USER webhook
EXPOSE 8080
CMD ["./api-gateway"]

# Webhook Registry service
FROM base AS webhook-registry
COPY --from=builder /app/bin/webhook-registry .
USER webhook
EXPOSE 8086
CMD ["./webhook-registry"]

# Event Ingestion service
FROM base AS event-ingestion
COPY --from=builder /app/bin/event-ingestion .
USER webhook
EXPOSE 8082
CMD ["./event-ingestion"]

# Webhook Dispatcher service
FROM base AS webhook-dispatcher
COPY --from=builder /app/bin/webhook-dispatcher .
USER webhook
CMD ["./webhook-dispatcher"]

# Retry Manager service
FROM base AS retry-manager
COPY --from=builder /app/bin/retry-manager .
USER webhook
EXPOSE 8083
CMD ["./retry-manager"]

# Observability service
FROM base AS observability
COPY --from=builder /app/bin/observability .
USER webhook
EXPOSE 8084 9091
CMD ["./observability"]

# DLQ service
FROM base AS dlq
COPY --from=builder /app/bin/dlq .
USER webhook
EXPOSE 8087
CMD ["./dlq"]

# Development image with all services
FROM base AS development
COPY --from=builder /app/bin/ ./bin/
RUN chown -R webhook:webhook /app
USER webhook

# Health check for all services
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1 