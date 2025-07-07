#!/bin/bash

# Start Monitoring Stack for Webhook System
echo "🚀 Starting Webhook Monitoring Stack..."

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo "❌ Docker is not running. Please start Docker first."
    exit 1
fi

# Start monitoring services
echo "📊 Starting Prometheus..."
docker-compose up -d prometheus

echo "📈 Starting Grafana..."
docker-compose up -d grafana

echo "📝 Starting Loki..."
docker-compose up -d loki

# Wait for services to be ready
echo "⏳ Waiting for services to be ready..."
sleep 10

# Check service status
echo "🔍 Checking service status..."
docker-compose ps prometheus grafana loki

echo ""
echo "✅ Monitoring Stack Started Successfully!"
echo ""
echo "📊 Access URLs:"
echo "   Grafana:     http://localhost:3000 (admin/admin123)"
echo "   Prometheus:  http://localhost:9090"
echo "   Loki:        http://localhost:3100"
echo "   Kafka UI:    http://localhost:8888"
echo ""
echo "📋 Next Steps:"
echo "   1. Open Grafana at http://localhost:3000"
echo "   2. Login with admin/admin123"
echo "   3. The 'Webhook System Overview' dashboard should be available"
echo "   4. Configure Loki as a data source for log queries"
echo ""
echo "🔧 To view logs:"
echo "   docker-compose logs -f [service-name]"
echo "   Example: docker-compose logs -f event-ingestion"
echo ""
echo "🛑 To stop monitoring:"
echo "   docker-compose stop prometheus grafana loki" 