# Multi-stage Dockerfile for Webhook Service

# Build stage
FROM golang:1.22.4-alpine AS builder

# Install dependencies
RUN apk add --no-cache git protobuf protobuf-dev

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
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/api-gateway ./cmd/api-gateway && \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/webhook-registry ./cmd/webhook-registry && \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/event-ingestion ./cmd/event-ingestion && \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/webhook-dispatcher ./cmd/webhook-dispatcher && \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/retry-manager ./cmd/retry-manager && \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/observability ./cmd/observability

# Base runtime image
FROM alpine:3.18 AS base

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 webhook && \
    adduser -D -s /bin/sh -u 1001 -G webhook webhook

# Create app directory
WORKDIR /app

# Copy configuration files
COPY config/ ./config/
COPY migrations/ ./migrations/

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
EXPOSE 8081
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

# Development image with all services
FROM base AS development
COPY --from=builder /app/bin/ ./bin/
RUN chown -R webhook:webhook /app
USER webhook

# Health check for all services
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1 