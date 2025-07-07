package dlq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/alexnthnz/webhook/pkg/kafka"
	"github.com/alexnthnz/webhook/pkg/postgres"
	pb "github.com/alexnthnz/webhook/proto/generated"
)

// DLQMessage represents a message in the dead letter queue
type DLQMessage struct {
	EventID       string                 `json:"event_id"`
	WebhookID     string                 `json:"webhook_id"`
	CustomerID    string                 `json:"customer_id"`
	EventType     string                 `json:"event_type"`
	OriginalEvent map[string]interface{} `json:"original_event"`
	FailureReason string                 `json:"failure_reason"`
	Attempts      int                    `json:"attempts"`
	FirstFailedAt time.Time              `json:"first_failed_at"`
	LastFailedAt  time.Time              `json:"last_failed_at"`
	CreatedAt     time.Time              `json:"created_at"`
	Headers       map[string]string      `json:"headers"`
}

// Service implements the DLQ service
type Service struct {
	pb.UnimplementedDLQServiceServer
	producer *kafka.Producer
	db       *postgres.Client
	log      *logrus.Logger
	dlqTopic string
}

// NewService creates a new DLQ service
func NewService(producer *kafka.Producer, db *postgres.Client, dlqTopic string, logger *logrus.Logger) *Service {
	return &Service{
		producer: producer,
		db:       db,
		log:      logger,
		dlqTopic: dlqTopic,
	}
}

// SendToDLQ sends a failed webhook delivery to the dead letter queue
func (s *Service) SendToDLQ(ctx context.Context, req *pb.SendToDLQRequest) (*pb.SendToDLQResponse, error) {
	if req.EventId == "" {
		return nil, status.Error(codes.InvalidArgument, "event_id is required")
	}
	if req.WebhookId == "" {
		return nil, status.Error(codes.InvalidArgument, "webhook_id is required")
	}
	if req.CustomerId == "" {
		return nil, status.Error(codes.InvalidArgument, "customer_id is required")
	}

	// Parse original event
	var originalEvent map[string]interface{}
	if err := json.Unmarshal(req.OriginalEventPayload, &originalEvent); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid original event payload")
	}

	dlqMessage := DLQMessage{
		EventID:       req.EventId,
		WebhookID:     req.WebhookId,
		CustomerID:    req.CustomerId,
		EventType:     req.EventType,
		OriginalEvent: originalEvent,
		FailureReason: req.FailureReason,
		Attempts:      int(req.Attempts),
		FirstFailedAt: req.FirstFailedAt.AsTime(),
		LastFailedAt:  req.LastFailedAt.AsTime(),
		CreatedAt:     time.Now(),
		Headers:       req.Headers,
	}

	// Send to Kafka DLQ topic
	messageBytes, err := json.Marshal(dlqMessage)
	if err != nil {
		s.log.WithError(err).Error("Failed to marshal DLQ message")
		return nil, status.Error(codes.Internal, "failed to marshal DLQ message")
	}

	err = s.producer.ProduceSync(s.dlqTopic, []byte(req.EventId), messageBytes, nil, 5*time.Second)
	if err != nil {
		s.log.WithError(err).Error("Failed to send message to DLQ topic")
		return nil, status.Error(codes.Internal, "failed to send message to DLQ")
	}

	// Store in database for querying and management
	err = s.StoreDLQRecord(ctx, dlqMessage)
	if err != nil {
		s.log.WithError(err).Error("Failed to store DLQ record in database")
		// Don't fail the request if DB storage fails, as message is already in Kafka
	}

	s.log.WithFields(logrus.Fields{
		"event_id":       req.EventId,
		"webhook_id":     req.WebhookId,
		"customer_id":    req.CustomerId,
		"failure_reason": req.FailureReason,
		"attempts":       req.Attempts,
	}).Info("Message sent to dead letter queue")

	return &pb.SendToDLQResponse{
		Success:   true,
		MessageId: fmt.Sprintf("dlq-%s-%d", req.EventId, time.Now().Unix()),
	}, nil
}

// GetDLQMessages retrieves messages from the dead letter queue
func (s *Service) GetDLQMessages(ctx context.Context, req *pb.GetDLQMessagesRequest) (*pb.GetDLQMessagesResponse, error) {
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

	messages, totalCount, err := s.getDLQRecords(ctx, req, limit, offset)
	if err != nil {
		s.log.WithError(err).Error("Failed to get DLQ messages")
		return nil, status.Error(codes.Internal, "failed to get DLQ messages")
	}

	return &pb.GetDLQMessagesResponse{
		Messages:   messages,
		TotalCount: int32(totalCount),
	}, nil
}

// RequeueMessage requeues a message from the DLQ back to the main processing queue
func (s *Service) RequeueMessage(ctx context.Context, req *pb.RequeueMessageRequest) (*pb.RequeueMessageResponse, error) {
	if req.EventId == "" {
		return nil, status.Error(codes.InvalidArgument, "event_id is required")
	}

	// Get the DLQ message
	dlqMessage, err := s.getDLQRecord(ctx, req.EventId)
	if err != nil {
		s.log.WithError(err).Error("Failed to get DLQ record")
		return nil, status.Error(codes.NotFound, "DLQ message not found")
	}

	// Send back to main events topic
	eventBytes, err := json.Marshal(dlqMessage.OriginalEvent)
	if err != nil {
		s.log.WithError(err).Error("Failed to marshal original event")
		return nil, status.Error(codes.Internal, "failed to marshal original event")
	}

	err = s.producer.ProduceSync("webhook-events", []byte(req.EventId), eventBytes, nil, 5*time.Second)
	if err != nil {
		s.log.WithError(err).Error("Failed to requeue message")
		return nil, status.Error(codes.Internal, "failed to requeue message")
	}

	// Mark as requeued in database
	err = s.markDLQRecordRequeued(ctx, req.EventId)
	if err != nil {
		s.log.WithError(err).Error("Failed to mark DLQ record as requeued")
		// Don't fail the request if DB update fails
	}

	s.log.WithFields(logrus.Fields{
		"event_id":    req.EventId,
		"webhook_id":  dlqMessage.WebhookID,
		"customer_id": dlqMessage.CustomerID,
	}).Info("Message requeued from dead letter queue")

	return &pb.RequeueMessageResponse{
		Success: true,
	}, nil
}

// DeleteDLQMessage deletes a message from the DLQ
func (s *Service) DeleteDLQMessage(ctx context.Context, req *pb.DeleteDLQMessageRequest) (*pb.DeleteDLQMessageResponse, error) {
	if req.EventId == "" {
		return nil, status.Error(codes.InvalidArgument, "event_id is required")
	}

	err := s.deleteDLQRecord(ctx, req.EventId)
	if err != nil {
		s.log.WithError(err).Error("Failed to delete DLQ record")
		return nil, status.Error(codes.Internal, "failed to delete DLQ message")
	}

	s.log.WithField("event_id", req.EventId).Info("DLQ message deleted")

	return &pb.DeleteDLQMessageResponse{
		Success: true,
	}, nil
}

// StoreDLQRecord stores DLQ message in database (public method for internal use)
func (s *Service) StoreDLQRecord(ctx context.Context, message DLQMessage) error {
	query := `
		INSERT INTO dlq_messages (
			event_id, webhook_id, customer_id, event_type, original_event,
			failure_reason, attempts, first_failed_at, last_failed_at,
			created_at, headers, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (event_id) DO UPDATE SET
			failure_reason = EXCLUDED.failure_reason,
			attempts = EXCLUDED.attempts,
			last_failed_at = EXCLUDED.last_failed_at,
			headers = EXCLUDED.headers
	`

	originalEventJSON, _ := json.Marshal(message.OriginalEvent)
	headersJSON, _ := json.Marshal(message.Headers)

	_, err := s.db.Pool().Exec(ctx, query,
		message.EventID,
		message.WebhookID,
		message.CustomerID,
		message.EventType,
		originalEventJSON,
		message.FailureReason,
		message.Attempts,
		message.FirstFailedAt,
		message.LastFailedAt,
		message.CreatedAt,
		headersJSON,
		"active",
	)

	return err
}

// getDLQRecords retrieves DLQ records from database
func (s *Service) getDLQRecords(ctx context.Context, req *pb.GetDLQMessagesRequest, limit, offset int) ([]*pb.DLQMessage, int, error) {
	// Build query with filters
	whereClause := "WHERE status = 'active'"
	args := []interface{}{}
	argCount := 0

	if req.CustomerId != "" {
		argCount++
		whereClause += fmt.Sprintf(" AND customer_id = $%d", argCount)
		args = append(args, req.CustomerId)
	}

	if req.WebhookId != "" {
		argCount++
		whereClause += fmt.Sprintf(" AND webhook_id = $%d", argCount)
		args = append(args, req.WebhookId)
	}

	if req.EventType != "" {
		argCount++
		whereClause += fmt.Sprintf(" AND event_type = $%d", argCount)
		args = append(args, req.EventType)
	}

	// Count query
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM dlq_messages %s", whereClause)
	var totalCount int
	err := s.db.Pool().QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	// Main query
	argCount++
	limitArg := argCount
	argCount++
	offsetArg := argCount

	query := fmt.Sprintf(`
		SELECT event_id, webhook_id, customer_id, event_type, original_event,
			   failure_reason, attempts, first_failed_at, last_failed_at,
			   created_at, headers
		FROM dlq_messages %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, limitArg, offsetArg)

	args = append(args, limit, offset)

	rows, err := s.db.Pool().Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var messages []*pb.DLQMessage
	for rows.Next() {
		var message pb.DLQMessage
		var originalEventJSON, headersJSON []byte
		var firstFailedAt, lastFailedAt, createdAt time.Time

		err := rows.Scan(
			&message.EventId,
			&message.WebhookId,
			&message.CustomerId,
			&message.EventType,
			&originalEventJSON,
			&message.FailureReason,
			&message.Attempts,
			&firstFailedAt,
			&lastFailedAt,
			&createdAt,
			&headersJSON,
		)
		if err != nil {
			s.log.WithError(err).Error("Failed to scan DLQ message row")
			continue
		}

		message.FirstFailedAt = timestamppb.New(firstFailedAt)
		message.LastFailedAt = timestamppb.New(lastFailedAt)
		message.CreatedAt = timestamppb.New(createdAt)
		message.OriginalEventPayload = originalEventJSON

		// Parse headers
		var headers map[string]string
		if err := json.Unmarshal(headersJSON, &headers); err == nil {
			message.Headers = headers
		}

		messages = append(messages, &message)
	}

	return messages, totalCount, nil
}

// getDLQRecord retrieves a single DLQ record
func (s *Service) getDLQRecord(ctx context.Context, eventID string) (*DLQMessage, error) {
	query := `
		SELECT event_id, webhook_id, customer_id, event_type, original_event,
			   failure_reason, attempts, first_failed_at, last_failed_at,
			   created_at, headers
		FROM dlq_messages
		WHERE event_id = $1 AND status = 'active'
	`

	var message DLQMessage
	var originalEventJSON, headersJSON []byte

	err := s.db.Pool().QueryRow(ctx, query, eventID).Scan(
		&message.EventID,
		&message.WebhookID,
		&message.CustomerID,
		&message.EventType,
		&originalEventJSON,
		&message.FailureReason,
		&message.Attempts,
		&message.FirstFailedAt,
		&message.LastFailedAt,
		&message.CreatedAt,
		&headersJSON,
	)
	if err != nil {
		return nil, err
	}

	// Parse original event
	if err := json.Unmarshal(originalEventJSON, &message.OriginalEvent); err != nil {
		return nil, err
	}

	// Parse headers
	if err := json.Unmarshal(headersJSON, &message.Headers); err != nil {
		message.Headers = make(map[string]string)
	}

	return &message, nil
}

// markDLQRecordRequeued marks a DLQ record as requeued
func (s *Service) markDLQRecordRequeued(ctx context.Context, eventID string) error {
	query := `
		UPDATE dlq_messages 
		SET status = 'requeued', requeued_at = $1
		WHERE event_id = $2
	`
	_, err := s.db.Pool().Exec(ctx, query, time.Now(), eventID)
	return err
}

// deleteDLQRecord deletes a DLQ record
func (s *Service) deleteDLQRecord(ctx context.Context, eventID string) error {
	query := `DELETE FROM dlq_messages WHERE event_id = $1`
	_, err := s.db.Pool().Exec(ctx, query, eventID)
	return err
}
