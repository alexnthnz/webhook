package registry

import (
	"context"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/alexnthnz/webhook/proto/generated"
)

// Service implements the WebhookRegistry gRPC service
type Service struct {
	pb.UnimplementedWebhookRegistryServer
	repo *Repository
	log  *logrus.Logger
}

// NewService creates a new webhook registry service
func NewService(repo *Repository, logger *logrus.Logger) *Service {
	return &Service{
		repo: repo,
		log:  logger,
	}
}

// CreateWebhook creates a new webhook
func (s *Service) CreateWebhook(ctx context.Context, req *pb.CreateWebhookRequest) (*pb.CreateWebhookResponse, error) {
	if req.CustomerId == "" {
		return nil, status.Error(codes.InvalidArgument, "customer_id is required")
	}
	if req.Url == "" {
		return nil, status.Error(codes.InvalidArgument, "url is required")
	}
	if len(req.EventTypes) == 0 {
		return nil, status.Error(codes.InvalidArgument, "at least one event type is required")
	}

	// Convert protobuf retry policy to internal type
	var retryPolicy *RetryPolicy
	if req.RetryPolicy != nil {
		retryPolicy = &RetryPolicy{
			MaxAttempts:    int(req.RetryPolicy.MaxAttempts),
			BackoffType:    req.RetryPolicy.BackoffType,
			InitialDelayMs: int(req.RetryPolicy.InitialDelayMs),
			MaxDelayMs:     int(req.RetryPolicy.MaxDelayMs),
			Multiplier:     req.RetryPolicy.Multiplier,
		}
	}

	createReq := CreateWebhookRequest{
		CustomerID:  req.CustomerId,
		URL:         req.Url,
		EventTypes:  req.EventTypes,
		RetryPolicy: retryPolicy,
		Secret:      req.Secret,
		Headers:     req.Headers,
	}

	webhook, err := s.repo.Create(ctx, createReq)
	if err != nil {
		s.log.WithError(err).Error("Failed to create webhook")
		return nil, status.Error(codes.Internal, "failed to create webhook")
	}

	return &pb.CreateWebhookResponse{
		WebhookId: webhook.ID,
		Secret:    webhook.Secret,
		CreatedAt: timestamppb.New(webhook.CreatedAt),
	}, nil
}

// GetWebhook retrieves a webhook by ID
func (s *Service) GetWebhook(ctx context.Context, req *pb.GetWebhookRequest) (*pb.GetWebhookResponse, error) {
	if req.WebhookId == "" {
		return nil, status.Error(codes.InvalidArgument, "webhook_id is required")
	}
	if req.CustomerId == "" {
		return nil, status.Error(codes.InvalidArgument, "customer_id is required")
	}

	webhook, err := s.repo.GetByID(ctx, req.WebhookId, req.CustomerId)
	if err != nil {
		if err.Error() == "webhook not found" {
			return nil, status.Error(codes.NotFound, "webhook not found")
		}
		s.log.WithError(err).Error("Failed to get webhook")
		return nil, status.Error(codes.Internal, "failed to get webhook")
	}

	return &pb.GetWebhookResponse{
		Webhook: s.convertToProto(webhook),
	}, nil
}

// GetWebhooksForEvent retrieves webhooks for a specific event type
func (s *Service) GetWebhooksForEvent(ctx context.Context, req *pb.GetWebhooksForEventRequest) (*pb.GetWebhooksForEventResponse, error) {
	if req.EventType == "" {
		return nil, status.Error(codes.InvalidArgument, "event_type is required")
	}

	webhooks, err := s.repo.GetByEventType(ctx, req.EventType)
	if err != nil {
		s.log.WithError(err).Error("Failed to get webhooks for event")
		return nil, status.Error(codes.Internal, "failed to get webhooks for event")
	}

	var pbWebhooks []*pb.Webhook
	for _, webhook := range webhooks {
		pbWebhooks = append(pbWebhooks, s.convertToProto(webhook))
	}

	return &pb.GetWebhooksForEventResponse{
		Webhooks: pbWebhooks,
	}, nil
}

// GetWebhooksByCustomer retrieves webhooks for a customer with pagination
func (s *Service) GetWebhooksByCustomer(ctx context.Context, req *pb.GetWebhooksByCustomerRequest) (*pb.GetWebhooksByCustomerResponse, error) {
	if req.CustomerId == "" {
		return nil, status.Error(codes.InvalidArgument, "customer_id is required")
	}

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

	webhooks, totalCount, err := s.repo.GetByCustomer(ctx, req.CustomerId, limit, offset)
	if err != nil {
		s.log.WithError(err).Error("Failed to get webhooks by customer")
		return nil, status.Error(codes.Internal, "failed to get webhooks by customer")
	}

	var pbWebhooks []*pb.Webhook
	for _, webhook := range webhooks {
		pbWebhooks = append(pbWebhooks, s.convertToProto(webhook))
	}

	return &pb.GetWebhooksByCustomerResponse{
		Webhooks:   pbWebhooks,
		TotalCount: int32(totalCount),
	}, nil
}

// UpdateWebhook updates an existing webhook
func (s *Service) UpdateWebhook(ctx context.Context, req *pb.UpdateWebhookRequest) (*pb.UpdateWebhookResponse, error) {
	if req.WebhookId == "" {
		return nil, status.Error(codes.InvalidArgument, "webhook_id is required")
	}
	if req.CustomerId == "" {
		return nil, status.Error(codes.InvalidArgument, "customer_id is required")
	}

	updateReq := UpdateWebhookRequest{
		EventTypes: req.EventTypes,
		Headers:    req.Headers,
	}

	if req.Url != nil {
		updateReq.URL = req.Url
	}

	if req.RetryPolicy != nil {
		updateReq.RetryPolicy = &RetryPolicy{
			MaxAttempts:    int(req.RetryPolicy.MaxAttempts),
			BackoffType:    req.RetryPolicy.BackoffType,
			InitialDelayMs: int(req.RetryPolicy.InitialDelayMs),
			MaxDelayMs:     int(req.RetryPolicy.MaxDelayMs),
			Multiplier:     req.RetryPolicy.Multiplier,
		}
	}

	webhook, err := s.repo.Update(ctx, req.WebhookId, req.CustomerId, updateReq)
	if err != nil {
		if err.Error() == "webhook not found" {
			return nil, status.Error(codes.NotFound, "webhook not found")
		}
		s.log.WithError(err).Error("Failed to update webhook")
		return nil, status.Error(codes.Internal, "failed to update webhook")
	}

	return &pb.UpdateWebhookResponse{
		Webhook: s.convertToProto(webhook),
	}, nil
}

// DeleteWebhook deletes a webhook
func (s *Service) DeleteWebhook(ctx context.Context, req *pb.DeleteWebhookRequest) (*pb.DeleteWebhookResponse, error) {
	if req.WebhookId == "" {
		return nil, status.Error(codes.InvalidArgument, "webhook_id is required")
	}
	if req.CustomerId == "" {
		return nil, status.Error(codes.InvalidArgument, "customer_id is required")
	}

	err := s.repo.Delete(ctx, req.WebhookId, req.CustomerId)
	if err != nil {
		if err.Error() == "webhook not found" {
			return nil, status.Error(codes.NotFound, "webhook not found")
		}
		s.log.WithError(err).Error("Failed to delete webhook")
		return nil, status.Error(codes.Internal, "failed to delete webhook")
	}

	return &pb.DeleteWebhookResponse{
		Success: true,
	}, nil
}

// ListWebhooks lists webhooks with pagination
func (s *Service) ListWebhooks(ctx context.Context, req *pb.ListWebhooksRequest) (*pb.ListWebhooksResponse, error) {
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

	webhooks, totalCount, err := s.repo.List(ctx, limit, offset, req.CustomerId)
	if err != nil {
		s.log.WithError(err).Error("Failed to list webhooks")
		return nil, status.Error(codes.Internal, "failed to list webhooks")
	}

	var pbWebhooks []*pb.Webhook
	for _, webhook := range webhooks {
		pbWebhooks = append(pbWebhooks, s.convertToProto(webhook))
	}

	return &pb.ListWebhooksResponse{
		Webhooks:   pbWebhooks,
		TotalCount: int32(totalCount),
	}, nil
}

// convertToProto converts internal webhook to protobuf webhook
func (s *Service) convertToProto(webhook *Webhook) *pb.Webhook {
	return &pb.Webhook{
		WebhookId:  webhook.ID,
		CustomerId: webhook.CustomerID,
		Url:        webhook.URL,
		EventTypes: webhook.EventTypes,
		RetryPolicy: &pb.RetryPolicy{
			MaxAttempts:    int32(webhook.RetryPolicy.MaxAttempts),
			BackoffType:    webhook.RetryPolicy.BackoffType,
			InitialDelayMs: int32(webhook.RetryPolicy.InitialDelayMs),
			MaxDelayMs:     int32(webhook.RetryPolicy.MaxDelayMs),
			Multiplier:     webhook.RetryPolicy.Multiplier,
		},
		Secret:    webhook.Secret,
		Headers:   webhook.Headers,
		CreatedAt: timestamppb.New(webhook.CreatedAt),
		UpdatedAt: timestamppb.New(webhook.UpdatedAt),
		Active:    webhook.Active,
	}
}
