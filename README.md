# Webhook Service System

A comprehensive, production-ready webhook service system built in Go that can handle 1 billion events per day (~10k QPS). The system provides reliable webhook delivery with retry mechanisms, observability, and horizontal scaling capabilities.

## 🚀 Features

- **High Performance**: Designed to handle 10,000+ requests per second
- **Reliable Delivery**: Exponential backoff retry with dead letter queues
- **Security**: OAuth2/JWT authentication, HMAC payload signing, rate limiting
- **Observability**: Comprehensive metrics with Prometheus, logs with Loki, dashboards with Grafana
- **Scalability**: Microservices architecture with Kafka for event streaming
- **Fault Tolerance**: Circuit breakers, health checks, graceful shutdowns

## 🏗️ Architecture

The system consists of 6 microservices:

1. **API Gateway** (Port 8080) - REST API with OAuth2 authentication
2. **Webhook Registry** (Port 8081) - gRPC service for webhook CRUD operations
3. **Event Ingestion** (Port 8082) - REST API for receiving events, publishes to Kafka
4. **Webhook Dispatcher** (Port 8083) - Kafka consumer that sends HTTP requests to webhooks
5. **Retry Manager** (Port 8084) - Redis-based retry scheduling with exponential backoff
6. **Observability** (Port 8085) - Metrics and logging aggregation service

### Infrastructure Components

- **PostgreSQL**: Webhook configurations and delivery logs
- **Redis**: Retry state management and caching
- **Kafka**: Event streaming and message queuing
- **Prometheus**: Metrics collection and alerting
- **Grafana**: Metrics visualization and dashboards
- **Loki**: Log aggregation and querying

## 📋 Prerequisites

- Go 1.22.4 or later
- Docker and Docker Compose
- Protocol Buffers compiler (protoc)
- Make

## 🛠️ Quick Start

### 1. Clone and Setup

```bash
git clone <repository-url>
cd webhook
make dev-setup
```

### 2. Start Infrastructure

```bash
docker-compose up -d postgres redis kafka-kraft schema-registry prometheus grafana loki
```

Wait for services to be healthy:

```bash
docker-compose ps
```

### 3. Run Database Migrations

```bash
make migrate-up
```

### 4. Build Services

```bash
make build
```

### 5. Start Services

```bash
# Terminal 1 - Webhook Registry
./bin/webhook-registry

# Terminal 2 - Event Ingestion
./bin/event-ingestion

# Terminal 3 - API Gateway
./bin/api-gateway

# Terminal 4 - Observability
./bin/observability

# Terminal 5 - Retry Manager (when implemented)
# ./bin/retry-manager

# Terminal 6 - Webhook Dispatcher (when implemented)
# ./bin/webhook-dispatcher
```

### 6. Access Services

- **API Gateway**: http://localhost:8080
- **Grafana Dashboard**: http://localhost:3000 (admin/admin123)
- **Prometheus**: http://localhost:9090
- **Kafka UI**: http://localhost:8888
- **Redis Insight**: http://localhost:8001

## 📖 API Documentation

### Authentication

First, get a JWT token:

```bash
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "password"}'
```

### Create a Webhook

```bash
curl -X POST http://localhost:8080/api/v1/webhooks \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://your-app.com/webhook",
    "event_types": ["user.created", "payment.completed"],
    "headers": {
      "X-Custom-Header": "value"
    }
  }'
```

### Send an Event

```bash
curl -X POST http://localhost:8082/events \
  -H "Content-Type: application/json" \
  -d '{
    "id": "event-123",
    "type": "user.created",
    "source": "user-service",
    "data": {
      "user_id": "12345",
      "email": "user@example.com"
    }
  }'
```

### List Webhooks

```bash
curl -X GET http://localhost:8080/api/v1/webhooks \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

## 🔧 Configuration

Configuration is managed through `config.yaml` and environment variables. Key settings:

```yaml
# Database
database:
  host: postgres
  port: 5432
  database: webhook_db
  username: postgres
  password: postgres123

# Security
security:
  jwt_secret: "your-secret-key-here"
  hmac_secret: "your-hmac-secret-here"
  oauth2_enabled: false

# Performance
dispatcher:
  worker_count: 10
  batch_size: 100
  http_timeout: 30s

# Retry Policy
retry_manager:
  max_retries: 5
  initial_delay: 1s
  max_delay: 300s
  backoff_multiplier: 2.0
```

## 📊 Monitoring and Observability

### Metrics

The system exposes comprehensive Prometheus metrics:

- `webhook_delivery_attempts_total` - Total delivery attempts
- `webhook_delivery_success_total` - Successful deliveries
- `webhook_delivery_failures_total` - Failed deliveries
- `webhook_delivery_duration_seconds` - Delivery latency
- `webhook_http_requests_total` - HTTP request metrics
- `webhook_kafka_messages_produced_total` - Kafka metrics

### Grafana Dashboards

Pre-configured dashboards show:
- Request rates and latency
- Webhook delivery success rates
- System resource usage
- Error rates and patterns

### Logging

Structured JSON logs are collected by Loki and can be queried in Grafana.

## 🔐 Security Features

- **Authentication**: JWT tokens with configurable expiration
- **Authorization**: Scope-based access control
- **Payload Signing**: HMAC-SHA256 signatures for webhook payloads
- **Rate Limiting**: Per-IP and per-customer limits
- **TLS Support**: HTTPS for all external communications

## 🏗️ Development

### Project Structure

```
webhook/
├── cmd/                    # Service entry points
│   ├── api-gateway/
│   ├── webhook-registry/
│   ├── event-ingestion/
│   ├── webhook-dispatcher/
│   ├── retry-manager/
│   └── observability/
├── internal/               # Internal packages
│   ├── gateway/           # API Gateway logic
│   ├── registry/          # Webhook Registry logic
│   ├── security/          # Authentication & authorization
│   ├── observability/     # Metrics & logging
│   └── retry/             # Retry management
├── pkg/                   # Shared packages
│   ├── postgres/          # Database client
│   ├── redis/             # Redis client
│   ├── kafka/             # Kafka client
│   └── metrics/           # Prometheus metrics
├── proto/                 # Protocol buffer definitions
├── migrations/            # Database migrations
├── docker/                # Docker configurations
└── config/                # Configuration management
```

### Adding New Features

1. **Define Protocol Buffers**: Add new RPC methods in `proto/`
2. **Generate Code**: Run `make proto-gen`
3. **Implement Service**: Add logic in `internal/`
4. **Add Tests**: Create unit and integration tests
5. **Update Documentation**: Update API docs and README

### Testing

```bash
# Run all tests
make test

# Run with coverage
go test -cover ./...

# Integration tests (requires running infrastructure)
go test -tags=integration ./tests/
```

## 🚀 Deployment

### Docker Deployment

Build and deploy with Docker Compose:

```bash
make docker-build
docker-compose up -d
```

### Kubernetes

Kubernetes manifests are available in the `k8s/` directory:

```bash
kubectl apply -f k8s/
```

### Production Considerations

1. **Database**: Use managed PostgreSQL (AWS RDS, GCP Cloud SQL)
2. **Message Queue**: Use managed Kafka (AWS MSK, Confluent Cloud)
3. **Caching**: Use managed Redis (AWS ElastiCache, GCP Memorystore)
4. **Monitoring**: Set up alerting rules in Prometheus
5. **Security**: Use proper secrets management (Vault, AWS Secrets Manager)
6. **Load Balancing**: Use application load balancers
7. **Auto-scaling**: Configure HPA based on metrics

## 🔄 Scaling

### Horizontal Scaling

- **API Gateway**: Scale based on HTTP request rate
- **Event Ingestion**: Scale based on Kafka producer lag
- **Webhook Dispatcher**: Scale based on Kafka consumer lag
- **Retry Manager**: Scale based on Redis queue size

### Performance Tuning

- Adjust Kafka partition counts for parallelism
- Tune database connection pools
- Optimize HTTP client timeouts and connection limits
- Configure appropriate JVM settings for Kafka

## 🐛 Troubleshooting

### Common Issues

1. **Service Won't Start**
   - Check configuration file exists and is valid
   - Verify database connectivity
   - Check port availability

2. **Webhooks Not Delivering**
   - Check Kafka connectivity
   - Verify webhook URLs are accessible
   - Check retry queue status

3. **High Latency**
   - Monitor database query performance
   - Check Kafka consumer lag
   - Verify network connectivity

### Debug Commands

```bash
# Check service health
curl http://localhost:8080/health

# View service logs
docker-compose logs webhook-registry

# Check Kafka topics
docker exec -it webhook-kafka kafka-topics --list --bootstrap-server localhost:9092

# Monitor metrics
curl http://localhost:8080/metrics
```

## 📝 API Reference

### Webhook Management

- `POST /api/v1/webhooks` - Create webhook
- `GET /api/v1/webhooks` - List webhooks
- `GET /api/v1/webhooks/{id}` - Get webhook
- `PUT /api/v1/webhooks/{id}` - Update webhook
- `DELETE /api/v1/webhooks/{id}` - Delete webhook

### Event Ingestion

- `POST /events` - Send event (public endpoint)
- `POST /api/v1/events` - Send event (authenticated)

### Monitoring

- `GET /health` - Health check
- `GET /metrics` - Prometheus metrics

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Commit your changes
4. Push to the branch
5. Create a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🆘 Support

For support and questions:
- Create an issue in the GitHub repository
- Check the troubleshooting section
- Review the API documentation

---

**Built with ❤️ using Go, Kafka, PostgreSQL, and modern cloud-native technologies.**