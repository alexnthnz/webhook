package retry

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/alexnthnz/webhook/proto/generated"
)

// Service implements the RetryManager gRPC service
type Service struct {
	pb.UnimplementedRetryManagerServer
	repo *Repository
	log  *logrus.Logger
}

// NewService creates a new retry manager service
func NewService(repo *Repository, logger *logrus.Logger) *Service {
	return &Service{
		repo: repo,
		log:  logger,
	}
}

// ScheduleRetry schedules a retry for a failed webhook delivery
func (s *Service) ScheduleRetry(ctx context.Context, req *pb.ScheduleRetryRequest) (*pb.ScheduleRetryResponse, error) {
	if req.EventId == "" {
		return nil, status.Error(codes.InvalidArgument, "event_id is required")
	}
	if req.WebhookId == "" {
		return nil, status.Error(codes.InvalidArgument, "webhook_id is required")
	}
	if req.CustomerId == "" {
		return nil, status.Error(codes.InvalidArgument, "customer_id is required")
	}

	// Calculate next retry time if not provided
	var nextRetryAt time.Time
	if req.NextRetryAt != nil {
		nextRetryAt = req.NextRetryAt.AsTime()
	} else {
		// Use default retry policy
		retryPolicy := RetryPolicy{
			MaxAttempts:    5,
			BackoffType:    "exponential",
			InitialDelayMs: 1000,   // 1 second
			MaxDelayMs:     300000, // 5 minutes
			Multiplier:     2.0,
		}
		nextRetryAt = s.CalculateNextRetryTime(int(req.AttemptNumber), retryPolicy)
	}

	scheduleReq := ScheduleRetryRequest{
		EventID:       req.EventId,
		WebhookID:     req.WebhookId,
		CustomerID:    req.CustomerId,
		Payload:       req.Payload,
		AttemptNumber: int(req.AttemptNumber),
		FailureReason: req.FailureReason,
		NextRetryAt:   nextRetryAt,
		Headers:       req.Headers,
		MaxAttempts:   5, // Default max attempts
	}

	retry, err := s.repo.ScheduleRetry(ctx, scheduleReq)
	if err != nil {
		s.log.WithError(err).Error("Failed to schedule retry")
		return nil, status.Error(codes.Internal, "failed to schedule retry")
	}

	return &pb.ScheduleRetryResponse{
		RetryId:     retry.RetryID,
		ScheduledAt: timestamppb.New(retry.CreatedAt),
		Success:     true,
	}, nil
}

// GetRetryStatus retrieves retry status by ID or event ID
func (s *Service) GetRetryStatus(ctx context.Context, req *pb.GetRetryStatusRequest) (*pb.GetRetryStatusResponse, error) {
	var retry *RetryStatus
	var err error

	if req.RetryId != "" {
		retry, err = s.repo.GetRetryStatus(ctx, req.RetryId)
	} else if req.EventId != "" {
		retry, err = s.repo.GetRetryStatusByEvent(ctx, req.EventId)
	} else {
		return nil, status.Error(codes.InvalidArgument, "either retry_id or event_id is required")
	}

	if err != nil {
		if err.Error() == "retry not found" || err.Error() == "no retry found for event" {
			return nil, status.Error(codes.NotFound, "retry not found")
		}
		s.log.WithError(err).Error("Failed to get retry status")
		return nil, status.Error(codes.Internal, "failed to get retry status")
	}

	return &pb.GetRetryStatusResponse{
		Status: s.convertToProto(retry),
	}, nil
}

// CancelRetry cancels a scheduled retry
func (s *Service) CancelRetry(ctx context.Context, req *pb.CancelRetryRequest) (*pb.CancelRetryResponse, error) {
	if req.RetryId == "" && req.EventId == "" {
		return nil, status.Error(codes.InvalidArgument, "either retry_id or event_id is required")
	}

	var retryID string
	if req.RetryId != "" {
		retryID = req.RetryId
	} else {
		// Get retry ID by event ID
		retry, err := s.repo.GetRetryStatusByEvent(ctx, req.EventId)
		if err != nil {
			if err.Error() == "no retry found for event" {
				return nil, status.Error(codes.NotFound, "retry not found")
			}
			s.log.WithError(err).Error("Failed to get retry by event")
			return nil, status.Error(codes.Internal, "failed to get retry")
		}
		retryID = retry.RetryID
	}

	reason := req.Reason
	if reason == "" {
		reason = "cancelled by user"
	}

	err := s.repo.CancelRetry(ctx, retryID, reason)
	if err != nil {
		if err.Error() == "retry not found" {
			return nil, status.Error(codes.NotFound, "retry not found")
		}
		s.log.WithError(err).Error("Failed to cancel retry")
		return nil, status.Error(codes.Internal, "failed to cancel retry")
	}

	return &pb.CancelRetryResponse{
		Success: true,
	}, nil
}

// ListRetries lists retries with filtering and pagination
func (s *Service) ListRetries(ctx context.Context, req *pb.ListRetriesRequest) (*pb.ListRetriesResponse, error) {
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 10 // Default limit
	}
	if limit > 100 {
		limit = 100 // Max limit
	}

	offset := int(req.Offset)
	if offset < 0 {
		offset = 0
	}

	// Convert protobuf state to internal state
	state := s.convertFromProtoState(req.State)

	retries, totalCount, err := s.repo.ListRetries(ctx, req.WebhookId, req.CustomerId, limit, offset, state)
	if err != nil {
		s.log.WithError(err).Error("Failed to list retries")
		return nil, status.Error(codes.Internal, "failed to list retries")
	}

	var pbRetries []*pb.RetryStatus
	for _, retry := range retries {
		pbRetries = append(pbRetries, s.convertToProto(retry))
	}

	return &pb.ListRetriesResponse{
		Retries:    pbRetries,
		TotalCount: int32(totalCount),
	}, nil
}

// ProcessRetries processes due retries (used by the retry processor)
func (s *Service) ProcessRetries(ctx context.Context, req *pb.ProcessRetriesRequest) (*pb.ProcessRetriesResponse, error) {
	batchSize := int(req.BatchSize)
	if batchSize <= 0 {
		batchSize = 10 // Default batch size
	}
	if batchSize > 100 {
		batchSize = 100 // Max batch size
	}

	dueRetries, err := s.repo.GetDueRetries(ctx, batchSize)
	if err != nil {
		s.log.WithError(err).Error("Failed to get due retries")
		return nil, status.Error(codes.Internal, "failed to get due retries")
	}

	var processedRetryIDs []string
	for _, retry := range dueRetries {
		// Mark retry as in progress
		err := s.repo.MarkRetryInProgress(ctx, retry.RetryID)
		if err != nil {
			s.log.WithError(err).WithField("retry_id", retry.RetryID).Error("Failed to mark retry in progress")
			continue
		}
		processedRetryIDs = append(processedRetryIDs, retry.RetryID)
	}

	s.log.WithFields(logrus.Fields{
		"batch_size":      batchSize,
		"processed_count": len(processedRetryIDs),
	}).Info("Processed due retries")

	return &pb.ProcessRetriesResponse{
		ProcessedCount:    int32(len(processedRetryIDs)),
		ProcessedRetryIds: processedRetryIDs,
	}, nil
}

// convertToProto converts internal retry status to protobuf retry status
func (s *Service) convertToProto(retry *RetryStatus) *pb.RetryStatus {
	return &pb.RetryStatus{
		RetryId:         retry.RetryID,
		EventId:         retry.EventID,
		WebhookId:       retry.WebhookID,
		CustomerId:      retry.CustomerID,
		Payload:         retry.Payload,
		AttemptNumber:   int32(retry.AttemptNumber),
		MaxAttempts:     int32(retry.MaxAttempts),
		FailureReason:   retry.FailureReason,
		State:           s.convertToProtoState(retry.State),
		CreatedAt:       timestamppb.New(retry.CreatedAt),
		NextRetryAt:     timestamppb.New(retry.NextRetryAt),
		LastAttemptedAt: timestamppb.New(retry.LastAttemptedAt),
		Headers:         retry.Headers,
	}
}

// convertToProtoState converts internal retry state to protobuf retry state
func (s *Service) convertToProtoState(state RetryState) pb.RetryState {
	switch state {
	case RetryStateScheduled:
		return pb.RetryState_RETRY_STATE_SCHEDULED
	case RetryStateInProgress:
		return pb.RetryState_RETRY_STATE_IN_PROGRESS
	case RetryStateCompleted:
		return pb.RetryState_RETRY_STATE_COMPLETED
	case RetryStateFailed:
		return pb.RetryState_RETRY_STATE_FAILED
	case RetryStateCancelled:
		return pb.RetryState_RETRY_STATE_CANCELLED
	case RetryStateExpired:
		return pb.RetryState_RETRY_STATE_EXPIRED
	default:
		return pb.RetryState_RETRY_STATE_UNSPECIFIED
	}
}

// convertFromProtoState converts protobuf retry state to internal retry state
func (s *Service) convertFromProtoState(state pb.RetryState) RetryState {
	switch state {
	case pb.RetryState_RETRY_STATE_SCHEDULED:
		return RetryStateScheduled
	case pb.RetryState_RETRY_STATE_IN_PROGRESS:
		return RetryStateInProgress
	case pb.RetryState_RETRY_STATE_COMPLETED:
		return RetryStateCompleted
	case pb.RetryState_RETRY_STATE_FAILED:
		return RetryStateFailed
	case pb.RetryState_RETRY_STATE_CANCELLED:
		return RetryStateCancelled
	case pb.RetryState_RETRY_STATE_EXPIRED:
		return RetryStateExpired
	default:
		return RetryStateUnspecified
	}
}

// Additional methods for retry processing (used by webhook dispatcher)

// MarkRetryCompleted marks a retry as completed
func (s *Service) MarkRetryCompleted(ctx context.Context, retryID string) error {
	return s.repo.MarkRetryCompleted(ctx, retryID)
}

// MarkRetryFailed marks a retry as failed
func (s *Service) MarkRetryFailed(ctx context.Context, retryID, reason string) error {
	return s.repo.MarkRetryFailed(ctx, retryID, reason)
}

// GetDueRetries gets retries that are due for processing
func (s *Service) GetDueRetries(ctx context.Context, limit int) ([]*RetryStatus, error) {
	return s.repo.GetDueRetries(ctx, limit)
}

// CalculateNextRetryTime calculates the next retry time based on retry policy
func (s *Service) CalculateNextRetryTime(attemptNumber int, retryPolicy RetryPolicy) time.Time {
	var delay time.Duration

	switch retryPolicy.BackoffType {
	case "exponential":
		// Exponential backoff: initial_delay * (multiplier ^ attempt_number)
		delayMs := float64(retryPolicy.InitialDelayMs)
		for i := 0; i < attemptNumber; i++ {
			delayMs *= retryPolicy.Multiplier
		}
		if delayMs > float64(retryPolicy.MaxDelayMs) {
			delayMs = float64(retryPolicy.MaxDelayMs)
		}
		delay = time.Duration(delayMs) * time.Millisecond
	case "linear":
		// Linear backoff: initial_delay + (attempt_number * multiplier * initial_delay)
		delayMs := retryPolicy.InitialDelayMs + int(float64(attemptNumber)*retryPolicy.Multiplier*float64(retryPolicy.InitialDelayMs))
		if delayMs > retryPolicy.MaxDelayMs {
			delayMs = retryPolicy.MaxDelayMs
		}
		delay = time.Duration(delayMs) * time.Millisecond
	default:
		// Default to exponential backoff
		delayMs := float64(retryPolicy.InitialDelayMs)
		for i := 0; i < attemptNumber; i++ {
			delayMs *= 2.0 // Default multiplier
		}
		if delayMs > float64(retryPolicy.MaxDelayMs) {
			delayMs = float64(retryPolicy.MaxDelayMs)
		}
		delay = time.Duration(delayMs) * time.Millisecond
	}

	return time.Now().Add(delay)
}

// RetryPolicy represents retry configuration
type RetryPolicy struct {
	MaxAttempts    int     `json:"max_attempts"`
	BackoffType    string  `json:"backoff_type"`
	InitialDelayMs int     `json:"initial_delay_ms"`
	MaxDelayMs     int     `json:"max_delay_ms"`
	Multiplier     float64 `json:"multiplier"`
}
