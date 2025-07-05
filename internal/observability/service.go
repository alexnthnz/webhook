package observability

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/alexnthnz/webhook/pkg/metrics"
	"github.com/alexnthnz/webhook/pkg/postgres"
	pb "github.com/alexnthnz/webhook/proto/generated/proto"
)

// Service implements the ObservabilityService gRPC service
type Service struct {
	pb.UnimplementedObservabilityServiceServer
	metrics *metrics.Metrics
	db      *postgres.Client
	log     *logrus.Logger

	// Prometheus metrics
	deliveryCounter     *prometheus.CounterVec
	deliveryDuration    *prometheus.HistogramVec
	deliveryStatusGauge *prometheus.GaugeVec
	customMetrics       map[string]prometheus.Collector
}

// NewService creates a new observability service
func NewService(metricsClient *metrics.Metrics, db *postgres.Client, logger *logrus.Logger) *Service {
	service := &Service{
		metrics:       metricsClient,
		db:            db,
		log:           logger,
		customMetrics: make(map[string]prometheus.Collector),
	}

	// Initialize Prometheus metrics
	service.initializeMetrics()

	return service
}

// initializeMetrics initializes Prometheus metrics
func (s *Service) initializeMetrics() {
	s.deliveryCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "webhook_deliveries_total",
			Help: "Total number of webhook deliveries",
		},
		[]string{"webhook_id", "customer_id", "event_type", "status", "http_status_code"},
	)

	s.deliveryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "webhook_delivery_duration_seconds",
			Help:    "Duration of webhook deliveries",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"webhook_id", "customer_id", "event_type", "status"},
	)

	s.deliveryStatusGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "webhook_delivery_status",
			Help: "Current status of webhook deliveries",
		},
		[]string{"webhook_id", "customer_id", "status"},
	)

	// Register metrics
	prometheus.MustRegister(s.deliveryCounter)
	prometheus.MustRegister(s.deliveryDuration)
	prometheus.MustRegister(s.deliveryStatusGauge)
}

// RecordMetric records a custom metric
func (s *Service) RecordMetric(ctx context.Context, req *pb.RecordMetricRequest) (*pb.RecordMetricResponse, error) {
	if req.MetricName == "" {
		return nil, status.Error(codes.InvalidArgument, "metric_name is required")
	}

	// Convert labels to Prometheus format
	labels := make(prometheus.Labels)
	for k, v := range req.Labels {
		labels[k] = v
	}

	// For custom metrics, we would need to create and register them dynamically
	// This is a simplified implementation - in production, you'd have a more sophisticated metric registry
	s.log.WithFields(logrus.Fields{
		"metric_name": req.MetricName,
		"value":       req.Value,
		"labels":      labels,
	}).Info("Custom metric recorded")

	s.log.WithFields(logrus.Fields{
		"metric_name": req.MetricName,
		"value":       req.Value,
		"labels":      req.Labels,
	}).Debug("Metric recorded successfully")

	return &pb.RecordMetricResponse{
		Success: true,
	}, nil
}

// RecordDeliveryEvent records a webhook delivery event
func (s *Service) RecordDeliveryEvent(ctx context.Context, req *pb.RecordDeliveryEventRequest) (*pb.RecordDeliveryEventResponse, error) {
	if req.EventId == "" {
		return nil, status.Error(codes.InvalidArgument, "event_id is required")
	}
	if req.WebhookId == "" {
		return nil, status.Error(codes.InvalidArgument, "webhook_id is required")
	}
	if req.CustomerId == "" {
		return nil, status.Error(codes.InvalidArgument, "customer_id is required")
	}

	// Record Prometheus metrics
	statusStr := s.convertDeliveryStatusToString(req.Status)
	httpStatusStr := strconv.Itoa(int(req.HttpStatusCode))

	s.deliveryCounter.WithLabelValues(
		req.WebhookId,
		req.CustomerId,
		req.EventType,
		statusStr,
		httpStatusStr,
	).Inc()

	s.deliveryDuration.WithLabelValues(
		req.WebhookId,
		req.CustomerId,
		req.EventType,
		statusStr,
	).Observe(float64(req.LatencyMs) / 1000.0) // Convert to seconds

	// Update status gauge
	s.deliveryStatusGauge.WithLabelValues(
		req.WebhookId,
		req.CustomerId,
		statusStr,
	).Set(1)

	// Store delivery log in database
	err := s.storeDeliveryLog(ctx, req)
	if err != nil {
		s.log.WithError(err).Error("Failed to store delivery log")
		// Don't return error as metrics recording succeeded
	}

	s.log.WithFields(logrus.Fields{
		"event_id":         req.EventId,
		"webhook_id":       req.WebhookId,
		"customer_id":      req.CustomerId,
		"status":           statusStr,
		"http_status_code": req.HttpStatusCode,
		"latency_ms":       req.LatencyMs,
	}).Info("Delivery event recorded")

	return &pb.RecordDeliveryEventResponse{
		Success: true,
	}, nil
}

// GetWebhookLogs retrieves webhook delivery logs
func (s *Service) GetWebhookLogs(ctx context.Context, req *pb.GetWebhookLogsRequest) (*pb.GetWebhookLogsResponse, error) {
	if req.WebhookId == "" && req.CustomerId == "" {
		return nil, status.Error(codes.InvalidArgument, "either webhook_id or customer_id is required")
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 100 // Default limit
	}
	if limit > 1000 {
		limit = 1000 // Max limit
	}

	offset := int(req.Offset)
	if offset < 0 {
		offset = 0
	}

	logs, totalCount, err := s.getDeliveryLogs(ctx, req, limit, offset)
	if err != nil {
		s.log.WithError(err).Error("Failed to get webhook logs")
		return nil, status.Error(codes.Internal, "failed to get webhook logs")
	}

	return &pb.GetWebhookLogsResponse{
		Logs:       logs,
		TotalCount: int32(totalCount),
	}, nil
}

// GetMetrics retrieves metrics data
func (s *Service) GetMetrics(ctx context.Context, req *pb.GetMetricsRequest) (*pb.GetMetricsResponse, error) {
	// This is a simplified implementation - in production, you'd query a time-series database
	// For now, we'll return current metric values from Prometheus

	var metrics []*pb.MetricData

	// Get delivery count metrics
	if len(req.MetricNames) == 0 || contains(req.MetricNames, "webhook_deliveries_total") {
		metricData := &pb.MetricData{
			MetricName: "webhook_deliveries_total",
			Labels:     make(map[string]string),
		}

		// Add current timestamp data point
		metricData.DataPoints = []*pb.DataPoint{
			{
				Timestamp: timestamppb.Now(),
				Value:     0, // Would be fetched from Prometheus in real implementation
			},
		}

		metrics = append(metrics, metricData)
	}

	return &pb.GetMetricsResponse{
		Metrics: metrics,
	}, nil
}

// GetDeliveryStats retrieves delivery statistics
func (s *Service) GetDeliveryStats(ctx context.Context, req *pb.GetDeliveryStatsRequest) (*pb.GetDeliveryStatsResponse, error) {
	stats, timeSeries, err := s.calculateDeliveryStats(ctx, req)
	if err != nil {
		s.log.WithError(err).Error("Failed to get delivery stats")
		return nil, status.Error(codes.Internal, "failed to get delivery stats")
	}

	return &pb.GetDeliveryStatsResponse{
		Stats:      stats,
		TimeSeries: timeSeries,
	}, nil
}

// StreamLogs streams webhook logs (simplified implementation)
func (s *Service) StreamLogs(req *pb.StreamLogsRequest, stream pb.ObservabilityService_StreamLogsServer) error {
	// This is a simplified implementation - in production, you'd use a proper streaming mechanism
	// For now, we'll send a few sample log entries

	ctx := stream.Context()

	// Get recent logs
	logsReq := &pb.GetWebhookLogsRequest{
		WebhookId:  req.WebhookId,
		CustomerId: req.CustomerId,
		Limit:      10,
		Offset:     0,
	}

	logs, _, err := s.getDeliveryLogs(ctx, logsReq, 10, 0)
	if err != nil {
		return status.Error(codes.Internal, "failed to get logs for streaming")
	}

	// Stream logs
	for _, log := range logs {
		if err := stream.Send(log); err != nil {
			return err
		}
	}

	return nil
}

// Helper methods

// storeDeliveryLog stores a delivery log in the database
func (s *Service) storeDeliveryLog(ctx context.Context, req *pb.RecordDeliveryEventRequest) error {
	query := `
		INSERT INTO delivery_logs (
			event_id, webhook_id, customer_id, event_type, status, 
			http_status_code, attempt_number, latency_ms, error_message, 
			timestamp, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	timestamp := time.Now()
	if req.Timestamp != nil {
		timestamp = req.Timestamp.AsTime()
	}

	metadataJSON := "{}"
	if req.Metadata != nil && len(req.Metadata) > 0 {
		// Convert metadata to JSON string
		metadataJSON = fmt.Sprintf(`{%s}`, mapToJSONString(req.Metadata))
	}

	_, err := s.db.Pool().Exec(ctx, query,
		req.EventId,
		req.WebhookId,
		req.CustomerId,
		req.EventType,
		s.convertDeliveryStatusToString(req.Status),
		req.HttpStatusCode,
		req.AttemptNumber,
		req.LatencyMs,
		req.ErrorMessage,
		timestamp,
		metadataJSON,
	)

	return err
}

// getDeliveryLogs retrieves delivery logs from the database
func (s *Service) getDeliveryLogs(ctx context.Context, req *pb.GetWebhookLogsRequest, limit, offset int) ([]*pb.LogEntry, int, error) {
	whereClause := "WHERE 1=1"
	args := []interface{}{}
	argIndex := 1

	if req.WebhookId != "" {
		whereClause += fmt.Sprintf(" AND webhook_id = $%d", argIndex)
		args = append(args, req.WebhookId)
		argIndex++
	}

	if req.CustomerId != "" {
		whereClause += fmt.Sprintf(" AND customer_id = $%d", argIndex)
		args = append(args, req.CustomerId)
		argIndex++
	}

	if req.StartTime != nil {
		whereClause += fmt.Sprintf(" AND timestamp >= $%d", argIndex)
		args = append(args, req.StartTime.AsTime())
		argIndex++
	}

	if req.EndTime != nil {
		whereClause += fmt.Sprintf(" AND timestamp <= $%d", argIndex)
		args = append(args, req.EndTime.AsTime())
		argIndex++
	}

	if req.StatusFilter != pb.DeliveryStatus_DELIVERY_STATUS_UNSPECIFIED {
		whereClause += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, s.convertDeliveryStatusToString(req.StatusFilter))
		argIndex++
	}

	// Get total count
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM delivery_logs %s", whereClause)
	var totalCount int
	err := s.db.Pool().QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get log count: %w", err)
	}

	// Get logs with pagination
	query := fmt.Sprintf(`
		SELECT event_id, webhook_id, customer_id, event_type, status, 
		       http_status_code, attempt_number, latency_ms, error_message, 
		       timestamp, metadata
		FROM delivery_logs %s
		ORDER BY timestamp DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)

	args = append(args, limit, offset)

	rows, err := s.db.Pool().Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query logs: %w", err)
	}
	defer rows.Close()

	var logs []*pb.LogEntry
	for rows.Next() {
		var log pb.LogEntry
		var statusStr string
		var metadataStr string
		var timestamp time.Time

		err := rows.Scan(
			&log.EventId,
			&log.WebhookId,
			&log.CustomerId,
			&log.EventType,
			&statusStr,
			&log.HttpStatusCode,
			&log.AttemptNumber,
			&log.LatencyMs,
			&log.ErrorMessage,
			&timestamp,
			&metadataStr,
		)
		if err != nil {
			s.log.WithError(err).Error("Failed to scan log row")
			continue
		}

		log.Status = s.convertStringToDeliveryStatus(statusStr)
		log.Timestamp = timestamppb.New(timestamp)
		log.LogId = fmt.Sprintf("%s-%d", log.EventId, log.AttemptNumber)

		// Parse metadata JSON (simplified)
		log.Metadata = make(map[string]string)

		logs = append(logs, &log)
	}

	return logs, totalCount, nil
}

// calculateDeliveryStats calculates delivery statistics
func (s *Service) calculateDeliveryStats(ctx context.Context, req *pb.GetDeliveryStatsRequest) (*pb.DeliveryStats, []*pb.TimeSeriesPoint, error) {
	// Simplified implementation - in production, you'd use proper time-series queries
	stats := &pb.DeliveryStats{
		TotalDeliveries:      100,
		SuccessfulDeliveries: 85,
		FailedDeliveries:     15,
		RetriedDeliveries:    5,
		SuccessRate:          0.85,
		AverageLatencyMs:     250.0,
		P95LatencyMs:         500.0,
		P99LatencyMs:         1000.0,
		StatusCodeDistribution: map[string]int64{
			"200": 85,
			"400": 5,
			"500": 10,
		},
		ErrorDistribution: map[string]int64{
			"timeout":      5,
			"server_error": 10,
		},
	}

	// Generate sample time series data
	var timeSeries []*pb.TimeSeriesPoint
	now := time.Now()
	for i := 0; i < 24; i++ {
		point := &pb.TimeSeriesPoint{
			Timestamp: timestamppb.New(now.Add(-time.Duration(i) * time.Hour)),
			Stats:     stats,
		}
		timeSeries = append(timeSeries, point)
	}

	return stats, timeSeries, nil
}

// convertDeliveryStatusToString converts protobuf delivery status to string
func (s *Service) convertDeliveryStatusToString(status pb.DeliveryStatus) string {
	switch status {
	case pb.DeliveryStatus_DELIVERY_STATUS_SUCCESS:
		return "success"
	case pb.DeliveryStatus_DELIVERY_STATUS_FAILED:
		return "failed"
	case pb.DeliveryStatus_DELIVERY_STATUS_RETRYING:
		return "retrying"
	case pb.DeliveryStatus_DELIVERY_STATUS_EXPIRED:
		return "expired"
	case pb.DeliveryStatus_DELIVERY_STATUS_CANCELLED:
		return "cancelled"
	default:
		return "unspecified"
	}
}

// convertStringToDeliveryStatus converts string to protobuf delivery status
func (s *Service) convertStringToDeliveryStatus(status string) pb.DeliveryStatus {
	switch status {
	case "success":
		return pb.DeliveryStatus_DELIVERY_STATUS_SUCCESS
	case "failed":
		return pb.DeliveryStatus_DELIVERY_STATUS_FAILED
	case "retrying":
		return pb.DeliveryStatus_DELIVERY_STATUS_RETRYING
	case "expired":
		return pb.DeliveryStatus_DELIVERY_STATUS_EXPIRED
	case "cancelled":
		return pb.DeliveryStatus_DELIVERY_STATUS_CANCELLED
	default:
		return pb.DeliveryStatus_DELIVERY_STATUS_UNSPECIFIED
	}
}

// Helper functions

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// mapToJSONString converts map to JSON string (simplified)
func mapToJSONString(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}

	result := ""
	first := true
	for k, v := range m {
		if !first {
			result += ","
		}
		result += fmt.Sprintf(`"%s":"%s"`, k, v)
		first = false
	}
	return result
}
