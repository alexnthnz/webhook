# Webhook System Monitoring Guide

This guide explains how to monitor your webhook system using Grafana, Prometheus, and Loki.

## 🚀 Quick Start

### 1. Start the Monitoring Stack

```bash
# Start all monitoring services
./scripts/start-monitoring.sh

# Or manually start specific services
docker-compose up -d prometheus grafana loki
```

### 2. Access Monitoring Tools

- **Grafana**: http://localhost:3000 (admin/admin123)
- **Prometheus**: http://localhost:9090
- **Loki**: http://localhost:3100
- **Kafka UI**: http://localhost:8888

## 📊 Available Dashboards

### Webhook System Overview Dashboard

The main dashboard (`Webhook System Overview`) includes:

- **Event Ingestion Rate**: Events per second being ingested
- **Kafka Message Production Rate**: Messages being sent to Kafka
- **Webhook Delivery Success Rate**: Percentage of successful deliveries
- **Webhook Delivery Latency**: Average and 95th percentile latency
- **Retry Attempts Rate**: Number of retry attempts per second
- **DLQ Message Rate**: Messages going to Dead Letter Queue
- **HTTP Request Rate**: Requests per service
- **HTTP Error Rate**: Error percentage per service

## 📈 Key Metrics

### Webhook Delivery Metrics

- `webhook_delivery_attempts_total`: Total delivery attempts
- `webhook_delivery_latency_seconds`: Delivery latency histogram
- `webhook_delivery_failures_total`: Failed deliveries by reason

### System Health Metrics

- `http_requests_total`: HTTP requests by service and status
- `webhook_events_ingested_total`: Events ingested
- `webhook_kafka_messages_produced_total`: Kafka messages produced
- `webhook_kafka_messages_consumed_total`: Kafka messages consumed

### Infrastructure Metrics

- `webhook_circuit_breaker_open_total`: Circuit breaker events
- `webhook_retry_attempts_total`: Retry attempts
- `webhook_dlq_messages_total`: DLQ messages

## 🔍 Log Monitoring with Loki

### Viewing Logs

1. **In Grafana**:
   - Go to Explore
   - Select Loki data source
   - Use LogQL queries to filter logs

2. **Example LogQL Queries**:
   ```logql
   # All webhook logs
   {service="webhook-dispatcher"}
   
   # Error logs
   {service="webhook-dispatcher"} |= "error"
   
   # Specific event type
   {service="event-ingestion"} |= "user.created"
   
   # Failed deliveries
   {service="webhook-dispatcher"} |= "Failed to dispatch"
   ```

3. **Via Docker**:
   ```bash
   # View logs for specific service
   docker-compose logs -f event-ingestion
   docker-compose logs -f webhook-dispatcher
   
   # View all webhook logs
   docker-compose logs -f | grep webhook
   ```

## 🚨 Alerting

### Setting Up Alerts

1. **In Grafana**:
   - Go to Alerting → Alert Rules
   - Create new alert rules based on metrics

2. **Example Alert Rules**:
   ```yaml
   # High error rate
   - alert: HighWebhookErrorRate
     expr: webhook:http_error_rate > 0.05
     for: 5m
     labels:
       severity: warning
     annotations:
       summary: "High webhook error rate"
   
   # Low success rate
   - alert: LowDeliverySuccessRate
     expr: webhook:delivery_success_rate < 0.95
     for: 5m
     labels:
       severity: critical
     annotations:
       summary: "Low webhook delivery success rate"
   
   # High latency
   - alert: HighDeliveryLatency
     expr: webhook:delivery_latency_p95 > 5
     for: 5m
     labels:
       severity: warning
     annotations:
       summary: "High webhook delivery latency"
   ```

## 🔧 Configuration

### Prometheus Configuration

Located at `docker/prometheus/prometheus.yml`:
- Scrapes metrics from all webhook services
- Includes recording rules for common queries
- 15-second scrape intervals

### Grafana Configuration

- **Data Sources**: Automatically configured (Prometheus, Loki)
- **Dashboards**: Auto-provisioned from `docker/grafana/dashboards/`
- **Provisioning**: Configured in `docker/grafana/provisioning/`

### Loki Configuration

Located at `docker/loki/loki-config.yml`:
- File-based storage
- 24-hour index retention
- Structured metadata disabled

## 📋 Troubleshooting

### Common Issues

1. **Services not showing metrics**:
   - Check if services are running: `docker-compose ps`
   - Verify metrics endpoints: `curl http://localhost:8082/metrics`
   - Check Prometheus targets: http://localhost:9090/targets

2. **Dashboard not loading**:
   - Check Grafana logs: `docker-compose logs grafana`
   - Verify data source connections
   - Check dashboard provisioning

3. **No logs in Loki**:
   - Verify Loki is running: `docker-compose ps loki`
   - Check log drivers in docker-compose.yml
   - Verify log format compatibility

### Useful Commands

```bash
# Check service health
docker-compose ps

# View service logs
docker-compose logs -f [service-name]

# Restart monitoring stack
docker-compose restart prometheus grafana loki

# Check Prometheus targets
curl http://localhost:9090/api/v1/targets

# Check Grafana health
curl http://localhost:3000/api/health

# Check Loki health
curl http://localhost:3100/ready
```

## 🎯 Best Practices

1. **Set up alerts** for critical metrics (error rates, latency)
2. **Monitor resource usage** (CPU, memory, disk)
3. **Use log correlation** to trace issues across services
4. **Set up dashboards** for different stakeholders (ops, dev, business)
5. **Regular backup** of monitoring data
6. **Document custom metrics** and their business meaning

## 📚 Additional Resources

- [Prometheus Query Language](https://prometheus.io/docs/prometheus/latest/querying/)
- [LogQL Reference](https://grafana.com/docs/loki/latest/logql/)
- [Grafana Dashboard Documentation](https://grafana.com/docs/grafana/latest/dashboards/)
- [Webhook System Architecture](../README.md) 