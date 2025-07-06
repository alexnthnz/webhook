#!/bin/bash

# Generate secure secrets for webhook service
# This script generates random secrets for JWT and HMAC signing

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}🔐 Generating secure secrets for webhook service${NC}"
echo

# Function to generate a random secret
generate_secret() {
    local length=$1
    openssl rand -hex $length 2>/dev/null || head -c $length /dev/urandom | xxd -p | tr -d '\n'
}

# Generate JWT secret (64 bytes = 128 hex chars)
JWT_SECRET=$(generate_secret 64)
echo -e "${GREEN}JWT_SECRET:${NC}"
echo "$JWT_SECRET"
echo

# Generate HMAC secret (32 bytes = 64 hex chars)
HMAC_SECRET=$(generate_secret 32)
echo -e "${GREEN}HMAC_SECRET:${NC}"
echo "$HMAC_SECRET"
echo

# Generate OAuth2 client secret (32 bytes = 64 hex chars)
OAUTH2_CLIENT_SECRET=$(generate_secret 32)
echo -e "${GREEN}OAUTH2_CLIENT_SECRET:${NC}"
echo "$OAUTH2_CLIENT_SECRET"
echo

# Create .env file
ENV_FILE=".env"
echo -e "${YELLOW}📝 Creating $ENV_FILE file...${NC}"

cat > $ENV_FILE << EOF
# Webhook Service Environment Variables
# Generated on $(date)

# Security Secrets
JWT_SECRET=$JWT_SECRET
HMAC_SECRET=$HMAC_SECRET
OAUTH2_CLIENT_SECRET=$OAUTH2_CLIENT_SECRET

# Database Configuration
DB_HOST=postgres
DB_PORT=5432
DB_NAME=webhook_db
DB_USERNAME=postgres
DB_PASSWORD=postgres123
DB_SSL_MODE=disable

# Redis Configuration
REDIS_ADDRESS=redis:6379
REDIS_PASSWORD=redis123

# Kafka Configuration
KAFKA_BROKERS=kafka-kraft:29092

# Service Configuration
ENVIRONMENT=development
LOG_LEVEL=info

# API Gateway
API_GATEWAY_PORT=8080
API_GATEWAY_HOST=0.0.0.0

# Webhook Registry
WEBHOOK_REGISTRY_PORT=8081
WEBHOOK_REGISTRY_HOST=0.0.0.0

# Event Ingestion
EVENT_INGESTION_PORT=8082
EVENT_INGESTION_HOST=0.0.0.0

# Webhook Dispatcher
DISPATCHER_WORKER_COUNT=10
DISPATCHER_BATCH_SIZE=100

# Retry Manager
RETRY_MANAGER_PORT=8084
RETRY_MANAGER_HOST=0.0.0.0
MAX_RETRIES=5
INITIAL_DELAY=1s
MAX_DELAY=300s
BACKOFF_MULTIPLIER=2.0

# Observability
OBSERVABILITY_PORT=8085
OBSERVABILITY_HOST=0.0.0.0

# DLQ Service
DLQ_PORT=8087
DLQ_HOST=0.0.0.0
DLQ_TOPIC=webhook-dlq

# Metrics
METRICS_ENABLED=true
METRICS_LISTEN_ADDR=0.0.0.0:9090
METRICS_PATH=/metrics

# Security Settings
OAUTH2_ENABLED=false
JWT_ISSUER=webhook-service
JWT_AUDIENCE=webhook-api
JWT_EXPIRATION=3600s
TLS_ENABLED=false
RATE_LIMIT_ENABLED=true
RATE_LIMIT_RPS=1000
EOF

echo -e "${GREEN}✅ Environment file created: $ENV_FILE${NC}"
echo

# Create docker-compose override for secrets
OVERRIDE_FILE="docker-compose.override.yml"
echo -e "${YELLOW}📝 Creating $OVERRIDE_FILE file...${NC}"

cat > $OVERRIDE_FILE << EOF
# Docker Compose override for secure secrets
# This file should NOT be committed to version control

version: '3.8'

services:
  api-gateway:
    environment:
      - JWT_SECRET=$JWT_SECRET
      - HMAC_SECRET=$HMAC_SECRET
      - OAUTH2_CLIENT_SECRET=$OAUTH2_CLIENT_SECRET

  webhook-registry:
    environment:
      - JWT_SECRET=$JWT_SECRET
      - HMAC_SECRET=$HMAC_SECRET

  event-ingestion:
    environment:
      - HMAC_SECRET=$HMAC_SECRET

  webhook-dispatcher:
    environment:
      - HMAC_SECRET=$HMAC_SECRET

  retry-manager:
    environment:
      - JWT_SECRET=$JWT_SECRET

  observability:
    environment:
      - JWT_SECRET=$JWT_SECRET

  dlq:
    environment:
      - JWT_SECRET=$JWT_SECRET
      - HMAC_SECRET=$HMAC_SECRET
EOF

echo -e "${GREEN}✅ Docker Compose override created: $OVERRIDE_FILE${NC}"
echo

# Create Kubernetes secret manifest
K8S_SECRET_FILE="k8s/webhook-secrets.yaml"
mkdir -p k8s
echo -e "${YELLOW}📝 Creating Kubernetes secret manifest: $K8S_SECRET_FILE${NC}"

# Base64 encode secrets for Kubernetes
JWT_SECRET_B64=$(echo -n "$JWT_SECRET" | base64)
HMAC_SECRET_B64=$(echo -n "$HMAC_SECRET" | base64)
OAUTH2_CLIENT_SECRET_B64=$(echo -n "$OAUTH2_CLIENT_SECRET" | base64)

cat > $K8S_SECRET_FILE << EOF
apiVersion: v1
kind: Secret
metadata:
  name: webhook-secrets
  namespace: webhook-service
type: Opaque
data:
  jwt-secret: $JWT_SECRET_B64
  hmac-secret: $HMAC_SECRET_B64
  oauth2-client-secret: $OAUTH2_CLIENT_SECRET_B64
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: webhook-config
  namespace: webhook-service
data:
  jwt-issuer: "webhook-service"
  jwt-audience: "webhook-api"
  jwt-expiration: "3600s"
  environment: "production"
  log-level: "info"
EOF

echo -e "${GREEN}✅ Kubernetes secret manifest created: $K8S_SECRET_FILE${NC}"
echo

# Create .gitignore entries
GITIGNORE_FILE=".gitignore"
echo -e "${YELLOW}📝 Updating $GITIGNORE_FILE...${NC}"

if [ ! -f "$GITIGNORE_FILE" ]; then
    touch "$GITIGNORE_FILE"
fi

# Add entries if they don't exist
grep -qF ".env" "$GITIGNORE_FILE" || echo ".env" >> "$GITIGNORE_FILE"
grep -qF "docker-compose.override.yml" "$GITIGNORE_FILE" || echo "docker-compose.override.yml" >> "$GITIGNORE_FILE"
grep -qF "k8s/webhook-secrets.yaml" "$GITIGNORE_FILE" || echo "k8s/webhook-secrets.yaml" >> "$GITIGNORE_FILE"

echo -e "${GREEN}✅ Updated $GITIGNORE_FILE${NC}"
echo

# Security warnings
echo -e "${RED}🚨 SECURITY WARNINGS:${NC}"
echo -e "${YELLOW}1. Never commit .env or docker-compose.override.yml to version control${NC}"
echo -e "${YELLOW}2. Use a proper secret management system in production (e.g., HashiCorp Vault, AWS Secrets Manager)${NC}"
echo -e "${YELLOW}3. Rotate secrets regularly${NC}"
echo -e "${YELLOW}4. Use different secrets for each environment${NC}"
echo -e "${YELLOW}5. Store the Kubernetes secret manifest securely and apply it manually${NC}"
echo

echo -e "${GREEN}🎉 Secret generation complete!${NC}"
echo
echo -e "${GREEN}To use the generated secrets:${NC}"
echo -e "${YELLOW}1. For Docker Compose: docker-compose up -d${NC}"
echo -e "${YELLOW}2. For local development: source .env${NC}"
echo -e "${YELLOW}3. For Kubernetes: kubectl apply -f k8s/webhook-secrets.yaml${NC}" 